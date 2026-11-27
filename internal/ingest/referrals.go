package ingest

import (
	"context"
	"fmt"
	"slices"

	"github.com/easterthebunny/pubcommittee/internal/billxml"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/versiondiff"
)

// referralsStep names every listed version code that is missing from the
// stage rank table (stored, not listed, reported on every pull until the
// table covers it) and collapses referral versions whose text equals the
// version before them (M1-05b). It needs the comparison store; without one,
// nothing collapses.
func referralsStep(ctx context.Context, r *Run) error {
	for _, c := range r.sourceCodes() {
		if !data.IsVersionCode(c) {
			r.Report.Notes = append(r.Report.Notes, fmt.Sprintf(
				"version code %q is not in the stage rank table: stored, not listed, until the table covers it", c))
		}
	}
	if !r.Decision.Tracked || r.DryRun || r.Ports.ComparisonStore == nil {
		return nil
	}
	codes, err := r.storedCodes(ctx)
	if err != nil {
		r.fail(r.Bill.String()+"/referrals", err)
		return nil
	}
	if _, err := versiondiff.CollapseStep(
		ctx, r.Ports.BillText, r.Ports.ComparisonStore, r.Ports.Publisher, r.Bill, codes, billxml.ParserVersion,
	); err != nil {
		r.fail(r.Bill.String()+"/referrals", err)
	}
	return nil
}

// sourceCodes lists the version codes the source names for the bill, sorted.
func (r *Run) sourceCodes() []string {
	out := make([]string, 0, len(r.Record.Versions))
	for _, v := range r.Record.Versions {
		out = append(out, v.Ref.Version)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// storedCodes is the set of version codes to consider for collapsing: the
// source's codes, or, with no source record (Analyze), the listed versions
// and the referrals collapsed earlier.
func (r *Run) storedCodes(ctx context.Context) ([]string, error) {
	if codes := r.sourceCodes(); len(codes) > 0 {
		return codes, nil
	}
	vs, err := r.Ports.BillText.ListVersions(ctx, r.Bill)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	var out []string
	for _, v := range vs {
		out = append(out, v.Ref.Version)
		for _, ref := range v.Referrals {
			out = append(out, ref.Code)
		}
	}
	return out, nil
}
