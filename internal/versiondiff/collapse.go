package versiondiff

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// Identical reports whether a stored comparison shows the same text: every
// section unchanged, every old line unchanged (nothing moved, changed, or
// removed), no run of difference, and no line added. Equal bytes are not
// required; only the parsed sections and lines are compared.
func Identical(c data.Comparison) bool {
	if c.LineCountOld != c.LineCountNew {
		return false
	}
	for _, sc := range c.Sections {
		if sc.Status != data.SectionUnchanged || len(sc.Changes) > 0 {
			return false
		}
		for _, m := range sc.Lines {
			if m.Class != data.LineUnchanged {
				return false
			}
		}
	}
	return true
}

// CollapseStep decides which referral versions (rfh, rfs) have the same text
// as the version before them and stores the decision as the bill's
// ReferralSet, which adapters apply when they list versions. codes are the
// version codes the source lists for the bill; only codes whose original is
// stored and that are in the stage rank table take part. Each referral is
// compared with the closest earlier version that is not itself a collapsed
// referral; the comparison is stored (write-once) and the referral collapses
// when Identical says so. A referral with no earlier version, or whose
// comparison cannot be made, stays a version of its own: unknown is better
// than wrong. Originals are never changed. It returns the collapsed
// referrals.
func CollapseStep(
	ctx context.Context, text data.BillText, store data.ComparisonWriter, pub data.Publisher,
	bill data.BillRef, codes []string, parserVersion string,
) ([]data.Referral, error) {
	ordered := slices.Clone(codes)
	ordered = slices.DeleteFunc(ordered, func(c string) bool { return !data.IsVersionCode(c) })
	slices.SortFunc(ordered, data.VersionCodeOrder(bill))
	ordered = slices.Compact(ordered)
	loaded := map[string]data.OfficialVersion{}
	var stored []string
	for _, c := range ordered {
		v, err := text.GetVersion(ctx, data.VersionRef{Bill: bill, Version: c})
		if errors.Is(err, data.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("collapse %s: get version %s: %w", bill, c, err)
		}
		loaded[c] = v
		stored = append(stored, c)
	}
	var out []data.Referral
	var base string // the closest earlier version that stays listed
	for _, c := range stored {
		if !data.IsReferralCode(c) || base == "" {
			base = c
			continue
		}
		cmp := Compare(loaded[base], loaded[c], parserVersion)
		if err := store.PutComparison(ctx, cmp); err != nil {
			return nil, fmt.Errorf("collapse %s: store comparison: %w", bill, err)
		}
		if !Identical(cmp) {
			base = c
			continue
		}
		v := loaded[c]
		out = append(out, data.Referral{Code: c, Into: base, Type: data.ReferralLabel(c), PublishedAt: v.PublishedAt})
	}
	if err := writeReferrals(ctx, pub, bill, out); err != nil {
		return nil, err
	}
	return out, nil
}

// writeReferrals replaces the bill's ReferralSet. Nothing is written for a
// bill that has none and never had one.
func writeReferrals(ctx context.Context, pub data.Publisher, bill data.BillRef, set []data.Referral) error {
	path := data.ReferralsPath(bill)
	if len(set) == 0 {
		if _, err := pub.GetFile(ctx, path); errors.Is(err, data.ErrNotFound) {
			return nil
		} else if err != nil {
			return fmt.Errorf("collapse %s: read referrals: %w", bill, err)
		}
	}
	body, err := data.MarshalReferrals(data.ReferralSet{Bill: bill, Collapsed: set})
	if err != nil {
		return fmt.Errorf("collapse %s: %w", bill, err)
	}
	if _, err := pub.PutReplaceable(ctx, path, body); err != nil {
		return fmt.Errorf("collapse %s: write referrals: %w", bill, err)
	}
	return nil
}
