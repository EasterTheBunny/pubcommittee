// Package lawingest is the existing-law step: it checks the law source for
// its newest release point and stores the sections of the requested titles,
// the release record, and the current pointer. Stored sections and release
// records are write-once, so a run with no new release point changes
// nothing. Nothing runs it automatically in the local phase; a command or
// job calls it (design/01 § Existing law freshness).
package lawingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/uslm"
)

// Result says what a run did.
type Result struct {
	Release  data.Release
	New      bool // the release record was not stored before this run
	Sections int  // sections written or confirmed
}

const (
	lastTitle     = 54
	reservedTitle = 53
)

// AllTitles returns the numbers of the US Code titles (title 53 is
// reserved and has no file).
func AllTitles() []int {
	out := make([]int, 0, lastTitle)
	for t := 1; t <= lastTitle; t++ {
		if t != reservedTitle {
			out = append(out, t)
		}
	}
	return out
}

// Option configures Step.
type Option func(*options)

type options struct {
	log    *slog.Logger
	now    func() time.Time
	source string
}

// Progress log values. A run's mode says which titles it stores: every title
// (full) or the ones given (subset). A title's method says how it was stored:
// fetched from the source. M1-07b adds the incremental mode and the copy method.
const (
	ModeFull    = "full"
	ModeSubset  = "subset"
	MethodFetch = "fetch"
)

// WithLogger logs one info line per stored title, so a long run shows its
// progress. Without it, Step logs nothing.
func WithLogger(l *slog.Logger) Option {
	return func(o *options) { o.log = l }
}

// WithSource names the law source in progress lines, for example "uscode"
// (the Law Revision Counsel) or "fixtures".
func WithSource(name string) Option {
	return func(o *options) { o.source = name }
}

// WithClock sets the clock for the elapsed times in progress lines (tests).
func WithClock(now func() time.Time) Option {
	return func(o *options) { o.now = now }
}

// Step stores the newest release point of src. With no titles, an already
// stored release is left alone and nothing is fetched except the release
// list; a release not yet stored needs titles (ErrInvalid otherwise). With
// titles, each is fetched, parsed and stored even for a stored release,
// which adds titles to it; sections already stored are confirmed
// byte for byte.
func Step(
	ctx context.Context, src data.LawSource, r data.LawReader, w data.LawWriter, titles []int, opts ...Option,
) (Result, error) {
	o := newOptions(opts)
	rp, rel, err := newestRelease(ctx, src)
	if err != nil {
		return Result{}, err
	}
	_, err = r.GetRelease(ctx, rel.ID)
	stored := err == nil
	if err != nil && !errors.Is(err, data.ErrNotFound) {
		return Result{}, fmt.Errorf("law ingest: get release %s: %w", rel.ID, err)
	}
	res := Result{Release: rel, New: !stored}
	if len(titles) == 0 {
		if !stored {
			return Result{}, fmt.Errorf("law ingest: release %s is new and no titles were given: %w", rel.ID, data.ErrInvalid)
		}
		return res, nil
	}
	n, err := o.storeTitles(ctx, src, w, rp, rel, titles)
	if err != nil {
		return Result{}, err
	}
	res.Sections = n
	if err := w.PutRelease(ctx, rel); err != nil {
		return Result{}, fmt.Errorf("law ingest: %w", err)
	}
	if err := w.SetCurrentRelease(ctx, rel.ID); err != nil {
		return Result{}, fmt.Errorf("law ingest: %w", err)
	}
	return res, nil
}

func newOptions(opts []Option) options {
	o := options{now: time.Now}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// storeTitles stores each title in order and logs its progress.
func (o options) storeTitles(
	ctx context.Context, src data.LawSource, w data.LawWriter, rp data.ReleasePoint, rel data.Release, titles []int,
) (int, error) {
	start := o.now()
	o.started(ctx, rel.ID, titles)
	total := 0
	for i, t := range titles {
		n, err := storeTitle(ctx, src, w, rp, rel, t)
		if err != nil {
			return 0, err
		}
		total += n
		o.progress(ctx, rel.ID, t, n, i+1, len(titles), start)
	}
	return total, nil
}

// progress logs one stored title: which, how many sections, and how far
// the run is. Never logs section text.
func (o options) progress(ctx context.Context, release string, title, sections, done, total int, start time.Time) {
	if o.log == nil {
		return
	}
	o.log.InfoContext(ctx, "law title stored",
		"release", release, "title", title, "method", MethodFetch, "source", o.source, "sections", sections,
		"done", done, "total", total, "elapsed_s", int(o.now().Sub(start).Seconds()))
}

// started logs the run's mode, source, and title count before the first title.
func (o options) started(ctx context.Context, release string, titles []int) {
	if o.log == nil {
		return
	}
	mode := ModeSubset
	if slices.Equal(titles, AllTitles()) {
		mode = ModeFull
	}
	o.log.InfoContext(ctx, "law ingest started",
		"release", release, "mode", mode, "source", o.source, "total", len(titles))
}

// storeTitle fetches one title, parses it, and stores its sections.
func storeTitle(
	ctx context.Context, src data.LawSource, w data.LawWriter, rp data.ReleasePoint, rel data.Release, title int,
) (int, error) {
	xmlBytes, err := src.FetchTitle(ctx, rp, title)
	if err != nil {
		return 0, fmt.Errorf("law ingest: fetch title %d of %s: %w", title, rel.ID, err)
	}
	n := 0
	err = uslm.ParseTitle(xmlBytes, rel, func(s data.LawSection) error {
		if err := w.PutLawSection(ctx, s); err != nil {
			return fmt.Errorf("store %s: %w", s.Ref, err)
		}
		n++
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("law ingest: title %d of %s: %w", title, rel.ID, err)
	}
	return n, nil
}

// newestRelease returns the newest release point of the source and its record.
func newestRelease(ctx context.Context, src data.LawSource) (data.ReleasePoint, data.Release, error) {
	rps, err := src.ListReleasePoints(ctx)
	if err != nil {
		return data.ReleasePoint{}, data.Release{}, fmt.Errorf("law ingest: list release points: %w", err)
	}
	if len(rps) == 0 {
		return data.ReleasePoint{}, data.Release{}, fmt.Errorf("law ingest: no release points: %w", data.ErrUnavailable)
	}
	rp := rps[0]
	if rp.Date.IsZero() {
		return data.ReleasePoint{}, data.Release{},
			fmt.Errorf("law ingest: release %s has no date: %w", rp.ID, data.ErrUnavailable)
	}
	return rp, data.Release{
		ID: rp.ID, PublicLaw: rp.ID, Date: rp.Date,
		Meta: data.Meta{Version: 1, AsOf: rp.Date, Provenance: data.ProvenanceSource, ContentClass: data.ContentOfficial},
	}, nil
}
