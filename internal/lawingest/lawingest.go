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

// Step stores the newest release point of src. With no titles, an already
// stored release is left alone and nothing is fetched except the release
// list; a release not yet stored needs titles (ErrInvalid otherwise). With
// titles, each is fetched, parsed and stored even for a stored release,
// which adds titles to it; sections already stored are confirmed
// byte for byte.
func Step(
	ctx context.Context, src data.LawSource, r data.LawReader, w data.LawWriter, titles []int,
) (Result, error) {
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
	for _, t := range titles {
		n, err := storeTitle(ctx, src, w, rp, rel, t)
		if err != nil {
			return Result{}, err
		}
		res.Sections += n
	}
	if err := w.PutRelease(ctx, rel); err != nil {
		return Result{}, fmt.Errorf("law ingest: %w", err)
	}
	if err := w.SetCurrentRelease(ctx, rel.ID); err != nil {
		return Result{}, fmt.Errorf("law ingest: %w", err)
	}
	return res, nil
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
