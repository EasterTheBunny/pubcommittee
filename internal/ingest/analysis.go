package ingest

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/billxml"
	"github.com/easterthebunny/pubcommittee/internal/citations"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/versiondiff"
)

// WithAnalysis inserts the citation step (M1-07) and the comparison step
// (M1-08) after the after-store step, in that order.
func WithAnalysis(p Pipeline) (Pipeline, error) {
	p, err := p.After(StepAfterStore, StepFunc{StepName: StepCitations, Fn: citationsStep})
	if err != nil {
		return nil, err
	}
	return p.After(StepCitations, StepFunc{StepName: StepComparisons, Fn: comparisonsStep})
}

// analysisWanted is true for a tracked bill on a real (not dry) run. The
// steps cover every stored version, not only the ones this pull stored, so
// a bill stored before the steps existed catches up; sets and comparisons
// are write-once, so repeating them is a no-op.
func (r *Run) analysisWanted() bool {
	return r.Decision.Tracked && !r.DryRun && r.Ports.CitationStore != nil
}

func citationsStep(ctx context.Context, r *Run) error {
	if !r.analysisWanted() || r.Ports.Law == nil {
		return nil
	}
	rel, err := r.Ports.Law.CurrentRelease(ctx)
	if errors.Is(err, data.ErrNotFound) {
		r.Report.Notes = append(r.Report.Notes,
			"citations skipped: no existing-law release point is stored (run job-ingest --law)")
		return nil
	}
	if err != nil {
		r.fail(r.Bill.String()+"/citations", err)
		return nil
	}
	later, err := LaterLaws(ctx, r.Ports, rel)
	if err != nil {
		r.fail(r.Bill.String()+"/citations", err)
		return nil
	}
	n, err := citations.Step(
		ctx, r.Ports.BillText, r.Ports.Law, r.Ports.CitationStore, r.Bill, billxml.ParserVersion, later)
	switch {
	case errors.Is(err, data.ErrConflict):
		// A set stored earlier for this release differs, for example because
		// a later law now flags a provision. Stored sets are never rewritten.
		r.Report.Notes = append(r.Report.Notes,
			"citations: a stored set for this release differs and was kept: "+shortErr(err))
	case err != nil:
		r.fail(r.Bill.String()+"/citations", err)
	default:
		r.Report.CitationSets = n
	}
	return nil
}

func comparisonsStep(ctx context.Context, r *Run) error {
	if !r.analysisWanted() || r.Ports.ComparisonStore == nil {
		return nil
	}
	n, err := versiondiff.Step(ctx, r.Ports.BillText, r.Ports.ComparisonStore, r.Bill, billxml.ParserVersion)
	if err != nil {
		r.fail(r.Bill.String()+"/comparisons", err)
		return nil
	}
	r.Report.Comparisons = n
	return nil
}

var publicLawNo = regexp.MustCompile(`(?i)public law(?: no\.?:?)? (\d+-\d+)`)

// LaterLaws lists the public laws enacted after the release point that the
// stored data can show: each catalog bill with status enacted whose latest
// action names a public law dated after the release date and whose enrolled
// text is stored. The enrolled text's own citation set gives the sections
// the law cites. It is empty when no stored bill qualifies.
func LaterLaws(ctx context.Context, ports data.PlatformPorts, rel data.Release) ([]citations.LaterLaw, error) {
	cat, err := LoadCatalog(ctx, ports.PublicDocs)
	if err != nil {
		return nil, err
	}
	var out []citations.LaterLaw
	for _, e := range cat.entries() {
		if e.Status != "enacted" || e.LatestAction == nil {
			continue
		}
		m := publicLawNo.FindStringSubmatch(e.LatestAction.Text)
		d, derr := time.Parse(time.DateOnly, e.LatestAction.Date)
		if m == nil || derr != nil || !d.After(rel.Date) {
			continue
		}
		set, ok, err := enrolledSet(ctx, ports, e.Bill, rel)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, citations.LaterLawFromSet(strings.TrimSpace(m[1]), d.UTC(), set))
		}
	}
	return out, nil
}

// enrolledSet resolves the citations of a bill's stored enrolled version.
func enrolledSet(
	ctx context.Context, ports data.PlatformPorts, bill data.BillRef, rel data.Release,
) (data.CitationSet, bool, error) {
	ref := data.VersionRef{Bill: bill, Version: "enr"}
	v, err := ports.BillText.GetVersion(ctx, ref)
	if errors.Is(err, data.ErrNotFound) {
		return data.CitationSet{}, false, nil
	}
	if err != nil {
		return data.CitationSet{}, false, fmt.Errorf("later laws: get %s: %w", ref, err)
	}
	src, err := ports.BillText.GetSource(ctx, ref)
	if err != nil {
		return data.CitationSet{}, false, fmt.Errorf("later laws: source %s: %w", ref, err)
	}
	set, err := citations.Build(ctx, ports.Law, rel, v, src, billxml.ParserVersion, nil)
	if err != nil {
		return data.CitationSet{}, false, fmt.Errorf("later laws: %w", err)
	}
	return set, true, nil
}
