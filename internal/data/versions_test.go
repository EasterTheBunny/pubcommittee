package data_test

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

func summaries(codes ...string) []data.VersionSummary {
	bill := data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: 3427}
	day := time.Date(2025, 9, 15, 0, 0, 0, 0, time.UTC)
	out := make([]data.VersionSummary, 0, len(codes))
	for _, c := range codes {
		out = append(out, data.VersionSummary{Ref: data.VersionRef{Bill: bill, Version: c}, PublishedAt: day})
	}
	return out
}

func codesOf(list []data.VersionSummary) []string {
	out := make([]string, 0, len(list))
	for _, v := range list {
		out = append(out, v.Ref.Version)
	}
	return out
}

func TestVersionRankTotalOrder(t *testing.T) {
	t.Parallel()
	// The govinfo list of bill version codes (https://www.govinfo.gov/help/bills).
	govinfo := []string{
		"as", "ash", "ath", "ats", "cdh", "cds", "cph", "cps", "eah", "eas", "eh", "eph", "enr", "es", "fah", "fph",
		"fps", "hdh", "hds", "ih", "iph", "ips", "is", "lth", "lts", "oph", "ops", "pav", "pch", "pcs", "pp", "pap",
		"pwah", "rah", "ras", "rch", "rcs", "rdh", "rds", "reah", "res", "renr", "rfh", "rfs", "rh", "rhuc", "rih",
		"ris", "rs", "rth", "rts", "sas", "sc",
	}
	for _, origin := range []data.Chamber{data.House, data.Senate} {
		bill := data.BillRef{Jurisdiction: "us", Session: "119", Type: map[data.Chamber]string{data.House: "hr", data.Senate: "s"}[origin], Number: 1}
		ranks := map[int]string{}
		for _, c := range govinfo {
			r, ok := data.VersionRank(bill, c)
			if !ok {
				t.Errorf("origin %d: code %q has no rank", origin, c)
				continue
			}
			if other, dup := ranks[r]; dup {
				t.Errorf("origin %d: codes %q and %q share rank %d", origin, other, c, r)
			}
			ranks[r] = c
		}
		if got := len(data.VersionCodes(origin)); got != len(govinfo) {
			t.Errorf("origin %d: table has %d codes, govinfo lists %d", origin, got, len(govinfo))
		}
	}
}

// The order follows the chamber a bill starts in: its own stages, then the
// other chamber's, then the amendment exchange led by the other chamber.
func TestVersionOrderByOrigin(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		bill data.BillRef
		path []string
	}{
		{data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: 1}, []string{"ih", "rh", "eh", "rds", "rfs", "rs", "eas", "eah", "enr"}},
		{data.BillRef{Jurisdiction: "us", Session: "119", Type: "hjres", Number: 1}, []string{"ih", "eh", "rfs", "es", "enr"}},
		{data.BillRef{Jurisdiction: "us", Session: "119", Type: "s", Number: 1}, []string{"is", "rs", "es", "rdh", "rfh", "rh", "eah", "eas", "enr"}},
		{data.BillRef{Jurisdiction: "us", Session: "119", Type: "sconres", Number: 1}, []string{"is", "es", "rfh", "eh", "enr"}},
	} {
		order := data.VersionCodeOrder(c.bill)
		shuffled := slices.Clone(c.path)
		slices.Reverse(shuffled)
		slices.SortFunc(shuffled, order)
		if !slices.Equal(shuffled, c.path) {
			t.Errorf("%s: order %v, want %v", c.bill, shuffled, c.path)
		}
	}
	if data.Origin(data.BillRef{Type: "sres"}) != data.Senate || data.Origin(data.BillRef{Type: "hconres"}) != data.House {
		t.Error("origin by bill type")
	}
}

func TestPublicVersionsOrder(t *testing.T) {
	t.Parallel()
	// H.R. 3427: rh and eh share a day; BILLSTATUS order is not trusted.
	for _, in := range [][]string{{"ih", "eh", "rh", "rfs"}, {"rfs", "eh", "rh", "ih"}, {"eh", "ih", "rfs", "rh"}} {
		got, unknown := data.PublicVersions(summaries(in...), data.ReferralSet{})
		if want := []string{"ih", "rh", "eh", "rfs"}; !reflect.DeepEqual(codesOf(got), want) || len(unknown) != 0 {
			t.Errorf("PublicVersions(%v) = %v, unknown %v, want %v", in, codesOf(got), unknown, want)
		}
	}
}

func TestPublicVersionsUnknownCode(t *testing.T) {
	t.Parallel()
	got, unknown := data.PublicVersions(summaries("eh", "zz9", "ih"), data.ReferralSet{})
	if want := []string{"ih", "eh"}; !reflect.DeepEqual(codesOf(got), want) {
		t.Errorf("list = %v, want %v", codesOf(got), want)
	}
	if !reflect.DeepEqual(unknown, []string{"zz9"}) {
		t.Errorf("unknown = %v, want [zz9]", unknown)
	}
}

func TestPublicVersionsCollapse(t *testing.T) {
	t.Parallel()
	ref := data.Referral{Code: "rfs", Into: "eh", Type: "Referred in Senate", PublishedAt: time.Date(2025, 9, 16, 0, 0, 0, 0, time.UTC)}
	got, _ := data.PublicVersions(summaries("ih", "rh", "eh", "rfs"), data.ReferralSet{Collapsed: []data.Referral{ref}})
	if want := []string{"ih", "rh", "eh"}; !reflect.DeepEqual(codesOf(got), want) {
		t.Fatalf("list = %v, want %v", codesOf(got), want)
	}
	if !reflect.DeepEqual(got[2].Referrals, []data.Referral{ref}) {
		t.Errorf("eh referrals = %+v, want %+v", got[2].Referrals, ref)
	}
	// A collapse whose target is not stored leaves the referral a version of its own.
	got, _ = data.PublicVersions(summaries("ih", "rfs"), data.ReferralSet{Collapsed: []data.Referral{ref}})
	if want := []string{"ih", "rfs"}; !reflect.DeepEqual(codesOf(got), want) {
		t.Errorf("list without target = %v, want %v", codesOf(got), want)
	}
}

func TestReferralsRoundTrip(t *testing.T) {
	t.Parallel()
	set := data.ReferralSet{Bill: data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: 1}}
	b, err := data.MarshalReferrals(set)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := data.ParseReferrals(b); err != nil {
		t.Fatal(err)
	}
	if _, err := data.ParseReferrals([]byte("{")); err == nil {
		t.Error("ParseReferrals of bad JSON: want an error")
	}
}
