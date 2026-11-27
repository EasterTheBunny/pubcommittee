package versiondiff_test

import (
	"io/fs"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/billxml"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/versiondiff"
)

// putFixture stores one fixture version of a bill under ref, dated day of September 2025.
func putFixture(t *testing.T, store *memory.Store, fixture string, ref data.BillRef, code string, day int) {
	t.Helper()
	src, err := fs.ReadFile(os.DirFS("../../fixtures/bills"), fixture+"/v/"+code+"/source.xml")
	if err != nil {
		t.Fatal(err)
	}
	d, err := billxml.Parse(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	v := data.OfficialVersion{
		Ref: data.VersionRef{Bill: ref, Version: code}, Type: "referred-in-senate",
		PublishedAt:   time.Date(2025, 9, day, 0, 0, 0, 0, time.UTC),
		ParserVersion: billxml.ParserVersion, Sections: d.DataSections(),
	}
	if err := store.PutVersion(t.Context(), v, src); err != nil {
		t.Fatal(err)
	}
}

// listedCodes returns the codes ListVersions gives for the bill, in order.
func listedCodes(t *testing.T, store *memory.Store, ref data.BillRef) []string {
	t.Helper()
	list, err := store.ListVersions(t.Context(), ref)
	if err != nil {
		t.Fatal(err)
	}
	codes := make([]string, 0, len(list))
	for _, v := range list {
		codes = append(codes, v.Ref.Version)
	}
	return codes
}

func collapse(t *testing.T, store *memory.Store, ref data.BillRef, codes ...string) []data.Referral {
	t.Helper()
	got, err := versiondiff.CollapseStep(t.Context(), store, store, store, ref, codes, billxml.ParserVersion)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestCollapseStepExitBills(t *testing.T) {
	t.Parallel()
	// H.R. 4313 and H.R. 3062 each have an rfs whose text is the same as eh.
	for _, fixture := range []string{"us-119-hr-4313", "us-119-hr-3062"} {
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()
			store := memory.New()
			ref := data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: 4313}
			if fixture == "us-119-hr-3062" {
				ref.Number = 3062
			}
			for i, c := range []string{"ih", "rh", "eh", "rfs"} {
				putFixture(t, store, fixture, ref, c, 1+i)
			}
			got := collapse(t, store, ref, "rfs", "eh", "rh", "ih")
			if len(got) != 1 || got[0].Code != "rfs" || got[0].Into != "eh" || got[0].Type != "Referred in Senate" {
				t.Fatalf("collapsed = %+v, want rfs into eh", got)
			}
			if codes := listedCodes(t, store, ref); !slices.Equal(codes, []string{"ih", "rh", "eh"}) {
				t.Errorf("listed = %v, want [ih rh eh]", codes)
			}
			// The original stays stored.
			if _, err := store.GetSource(t.Context(), data.VersionRef{Bill: ref, Version: "rfs"}); err != nil {
				t.Errorf("rfs source: %v", err)
			}
		})
	}
}

// TestCollapseStepChangedReferral: a synthetic rfs with one changed line stays
// a version of its own, after eh.
func TestCollapseStepChangedReferral(t *testing.T) {
	t.Parallel()
	ref := data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: 4313}
	scratch := memory.New()
	putFixture(t, scratch, "us-119-hr-4313", ref, "rfs", 2)
	rfs, err := scratch.GetVersion(t.Context(), data.VersionRef{Bill: ref, Version: "rfs"})
	if err != nil {
		t.Fatal(err)
	}
	rfs.Sections[0].Lines[0].Text += " and one more clause"

	store := memory.New()
	putFixture(t, store, "us-119-hr-4313", ref, "eh", 1)
	if err := store.PutVersion(t.Context(), rfs, []byte("<synthetic/>")); err != nil {
		t.Fatal(err)
	}
	if got := collapse(t, store, ref, "eh", "rfs"); len(got) != 0 {
		t.Fatalf("collapsed = %+v, want none", got)
	}
	if codes := listedCodes(t, store, ref); !slices.Equal(codes, []string{"eh", "rfs"}) {
		t.Errorf("listed = %v, want [eh rfs]", codes)
	}
}

func TestIdentical(t *testing.T) {
	t.Parallel()
	if !versiondiff.Identical(data.Comparison{}) {
		t.Error("empty comparison: want identical")
	}
	same := func() data.Comparison {
		return data.Comparison{LineCountOld: 1, LineCountNew: 1, Sections: []data.SectionComparison{
			{Status: data.SectionUnchanged, Lines: []data.LineMapping{{N: 1, Class: data.LineUnchanged}}},
		}}
	}
	if !versiondiff.Identical(same()) {
		t.Error("all unchanged: want identical")
	}
	for name, mut := range map[string]func(c *data.Comparison){
		"moved line": func(c *data.Comparison) { c.Sections[0].Lines[0].Class = data.LineMoved },
		"added line": func(c *data.Comparison) { c.LineCountNew = 2 },
		"added section": func(c *data.Comparison) {
			c.Sections = append(c.Sections, data.SectionComparison{Status: data.SectionAdded})
		},
		"change run": func(c *data.Comparison) {
			c.Sections[0].Changes = []data.ComparisonChange{{Kind: data.ChangeAdded}}
		},
	} {
		c := same()
		mut(&c)
		if versiondiff.Identical(c) {
			t.Errorf("%s: want not identical", name)
		}
	}
}
