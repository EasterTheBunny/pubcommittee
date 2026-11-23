package readapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	apiv1 "github.com/easterthebunny/pubcommittee/api/v1"
	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/readapi/seed"
)

// TestExplanationsArePinnedAndSkipShortTitles stores a third version whose section s1 only gives the short title,
// with an explanation on s1 and two on s4: one with stored anchors, one without (stored before anchors were kept).
func TestExplanationsArePinnedAndSkipShortTitles(t *testing.T) {
	t.Parallel()
	store, base := citationsStore(t)
	putThirdVersion(t, store)
	got := do(t, http.MethodGet, base+"/v1/bills/"+seed.TrackedBill+"/v/3/explanations", nil)
	if got.status != http.StatusOK {
		t.Fatalf("status %d: %s", got.status, got.body)
	}
	var page apiv1.ExplanationPage
	if err := json.Unmarshal(got.body, &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("items = %d, want 2 (the short-title section has none): %s", len(page.Items), got.body)
	}
	whole, pinned := page.Items[0], page.Items[1] // sorted by section, then provision id
	checkLines(t, "section-wide", whole, 2, 5)
	checkLines(t, "anchored", pinned, 3, 4)
	if pinned.ProvisionCitation == nil || *pinned.ProvisionCitation != "16 U.S.C. 824a(f)" {
		t.Errorf("provision_citation = %v", pinned.ProvisionCitation)
	}
}

func checkLines(t *testing.T, what string, e apiv1.Explanation, from, to int) {
	t.Helper()
	if e.LineFrom == nil || e.LineTo == nil || *e.LineFrom != from || *e.LineTo != to {
		t.Errorf("%s explanation lines = %v-%v, want %d-%d", what, e.LineFrom, e.LineTo, from, to)
	}
}

// putThirdVersion stores a third version of the tracked bill with a short-title section s1 and a section s4 of four
// lines, and three explanations: s1 (short title), s4 with stored anchors, and s4 without (stored before anchors).
func putThirdVersion(t *testing.T, store *memory.Store) {
	t.Helper()
	ctx := context.Background()
	vs := trackedVersions(t, store)
	ref := vs[0].Ref
	ref.Version = "eh"
	line := func(n int, text string) data.Line { return data.Line{N: n, Text: text} }
	meta := data.Meta{
		Version: 1, AsOf: time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC), Provenance: data.ProvenanceSource, ContentClass: data.ContentOfficial,
	}
	v := data.OfficialVersion{
		Ref: ref, Type: "engrossed", PublishedAt: meta.AsOf, SourceSHA256: "00", Meta: meta,
		Sections: []data.Section{
			{Path: []string{"s1"}, Label: "Sec. 1.", Heading: "Short title", Lines: []data.Line{line(1, "This Act may be cited as the “Example Act”.")}},
			{Path: []string{"s4"}, Label: "Sec. 4.", Heading: "Rules", Lines: []data.Line{line(2, "a"), line(3, "b"), line(4, "c"), line(5, "d")}},
		},
	}
	if err := store.PlatformPorts().Publisher.PutVersion(ctx, v, []byte("<bill/>")); err != nil {
		t.Fatal(err)
	}
	pm := data.Meta{Version: 1, AsOf: meta.AsOf, Provenance: data.ProvenanceAuto, ContentClass: data.ContentPlatform}
	for _, e := range []data.ChangeExplanation{
		{Version: ref, Section: []string{"s1"}, WhatChanges: "Names the Act.", Flags: []data.Provenance{data.ProvenanceAuto}, Meta: pm},
		{Version: ref, Section: []string{"s4"}, ProvisionID: "usc/16/824a/f", Anchors: []string{"s4-l3-4"}, WhatChanges: "x", Flags: []data.Provenance{data.ProvenanceAuto}, Meta: pm},
		{Version: ref, Section: []string{"s4"}, WhatChanges: "y", Flags: []data.Provenance{data.ProvenanceAuto}, Meta: pm},
	} {
		if err := store.PlatformPorts().PlatformDocs.PutExplanation(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
}
