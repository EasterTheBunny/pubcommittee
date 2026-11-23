package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/adapters/fixturesrc"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/ingest"
	"github.com/easterthebunny/pubcommittee/internal/lawingest"
)

var (
	errLawStore = errors.New("this target has no law store")
	errTitles   = errors.New("--titles wants numbers or all")
)

// lawReport is the JSON a --law run prints.
type lawReport struct {
	Release     string `json:"release"`
	CurrentThru string `json:"current_through"`
	New         bool   `json:"new_release"`
	Sections    int    `json:"sections"`
	DryRun      bool   `json:"dry_run,omitempty"`
}

// ingestLaw runs the existing-law step (design/01 § Existing law freshness):
// the newest release point is stored once; a repeat run with nothing new
// fetches no title and changes nothing.
func ingestLaw(
	ctx context.Context, d ingest.Deps, fetcher data.BillFetcher, cfg config, opts ingest.Options, out io.Writer,
) (int, error) {
	if d.Ports.LawStore == nil || d.Ports.Law == nil {
		return 0, errLawStore
	}
	src, err := newLawSource(cfg, fetcher)
	if err != nil {
		return 0, fmt.Errorf("law source: %w", err)
	}
	titles, err := lawTitles(ctx, src, cfg.Titles)
	if err != nil {
		return 0, err
	}
	rp, stored, err := newestRelease(ctx, src, d.Ports.Law)
	if err != nil {
		return 0, err
	}
	if opts.DryRun {
		return 0, encode(out, lawReport{
			Release: rp.ID, CurrentThru: rp.Date.Format(time.DateOnly), New: !stored, DryRun: true,
		})
	}
	// A release already stored needs no titles unless --titles adds some: nothing is fetched.
	if stored && cfg.Titles == "" {
		titles = nil
	}
	res, err := lawingest.Step(ctx, src, d.Ports.Law, d.Ports.LawStore, titles)
	if err != nil {
		return 0, fmt.Errorf("law step: %w", err)
	}
	return res.Sections, encode(out, lawReport{
		Release: res.Release.ID, CurrentThru: res.Release.Date.Format(time.DateOnly), New: res.New, Sections: res.Sections,
	})
}

// newestRelease returns the source's newest release point and whether it is stored.
func newestRelease(ctx context.Context, src data.LawSource, r data.LawReader) (data.ReleasePoint, bool, error) {
	rps, err := src.ListReleasePoints(ctx)
	if err != nil || len(rps) == 0 {
		return data.ReleasePoint{}, false, fmt.Errorf("list release points: %w", errors.Join(err, data.ErrUnavailable))
	}
	if _, err := r.GetRelease(ctx, rps[0].ID); err != nil {
		if errors.Is(err, data.ErrNotFound) {
			return rps[0], false, nil
		}
		return data.ReleasePoint{}, false, fmt.Errorf("get release: %w", err)
	}
	return rps[0], true, nil
}

// lawTitles reads --titles. Empty means: with the fixtures, the titles they
// hold; otherwise none (a new release then needs --titles).
func lawTitles(ctx context.Context, src data.LawSource, s string) ([]int, error) {
	switch s {
	case "all":
		return lawingest.AllTitles(), nil
	case "":
		fx, ok := src.(*fixturesrc.Source)
		if !ok {
			return nil, nil
		}
		rps, err := fx.ListReleasePoints(ctx)
		if err != nil || len(rps) == 0 {
			return nil, fmt.Errorf("list release points: %w", errors.Join(err, data.ErrUnavailable))
		}
		titles, err := fx.LawTitles(ctx, rps[0])
		if err != nil {
			return nil, fmt.Errorf("fixture titles: %w", err)
		}
		return titles, nil
	}
	var out []int
	for f := range strings.SplitSeq(s, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil {
			return nil, fmt.Errorf("%w: %q", errTitles, s)
		}
		out = append(out, n)
	}
	return out, nil
}
