package conformance

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// Values shared by the docs suite. Bill numbers 1, 2, 3, 10, and 11 keep the
// catalog order numeric; docsBillBetween sorts between 3 and 10.
const (
	docsPageSize    = 2
	docsNextVersion = 2
	docsBillBetween = 4
	docsIntroduced  = "introduced"
)

// docsFactory returns fresh ports backed by one store; each subtest calls it
// once, so no state leaks between subtests.
type docsFactory = func(t *testing.T) (data.PublicDocs, data.PlatformDocs, data.Moderation)

// docsCase is one named check of the docs suite.
type docsCase struct {
	name string
	run  func(t *testing.T, factory docsFactory)
}

// Docs runs the PublicDocs, PlatformDocs, and Moderation suite. factory must
// return ports backed by one store, so writes through PlatformDocs and
// Moderation are visible through PublicDocs.
func Docs(t *testing.T, factory func(t *testing.T) (data.PublicDocs, data.PlatformDocs, data.Moderation)) {
	t.Helper()
	cases := slices.Concat(
		docsBillCases(),
		docsPolicyCases(),
		docsCatalogCases(),
		docsExplanationCases(),
		docsGenerationIngestCases(),
		docsLogCases(),
		docsApplyCases(),
	)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			c.run(t, factory)
		})
	}
}

func docsBillCases() []docsCase {
	return []docsCase{
		{"GetBill unknown is not found", docsBillUnknown},
		{"PutBill then GetBill round trips", docsBillRoundTrip},
		{"PutBill with invalid ref is invalid", docsBillInvalid},
		{"PutBill twice replaces", docsBillReplace},
	}
}

func docsPolicyCases() []docsCase {
	return []docsCase{
		{"GetSelectionPolicy on empty store is not found", docsPolicyEmpty},
		{"GetSelectionPolicy returns the latest version", docsPolicyLatest},
		{"GetSelectionPolicyVersion returns one version or not found", docsPolicyByVersion},
		{"identical policy re-put is nil, different content conflicts", docsPolicyWriteOnce},
		{"policy version below one is invalid", docsPolicyInvalidVersion},
	}
}

func docsCatalogCases() []docsCase {
	return []docsCase{
		{"ListCatalog pages in bill key order", docsCatalogPages},
		{"ListCatalog cursor is stable under insert", docsCatalogCursorStable},
		{"ListCatalog on empty store is empty", docsCatalogEmpty},
		{"ListCatalog invalid cursor is invalid", docsCatalogInvalidCursor},
		{"PutCatalogEntry replaces by bill", docsCatalogReplace},
		{"catalog Tracked flag round trips", docsCatalogTracked},
	}
}

func docsExplanationCases() []docsCase {
	return []docsCase{
		{"GetExplanations on version with none is empty", docsExplanationsNone},
		{"PutExplanation twice with same key replaces", docsExplanationsReplace},
		{"GetExplanations orders by section then provision", docsExplanationsOrder},
		{"invalid VersionRef is invalid", docsExplanationsInvalidVersion},
	}
}

func docsGenerationIngestCases() []docsCase {
	return []docsCase{
		{"PutGeneration same ID: identical nil, different conflicts", docsGenerationWriteOnce},
		{"PutGeneration empty ID is invalid", docsGenerationEmptyID},
		{"GetIngestState unknown is not found", docsIngestUnknown},
		{"PutIngestState then GetIngestState round trips", docsIngestRoundTrip},
		{"PutIngestState empty source is invalid", docsIngestEmptySource},
	}
}

func docsLogCases() []docsCase {
	return []docsCase{
		{"Record then ListLog returns the entry", docsLogRecordList},
		{"ListLog is newest first and pages stably", docsLogNewestFirst},
		{"Record same ID and content is a no-op", docsLogIdempotent},
		{"Record same ID and different content conflicts", docsLogConflict},
		{"Record empty ID is invalid", docsLogEmptyID},
	}
}

func docsApplyCases() []docsCase {
	return []docsCase{
		{"ApplyLogged writes the catalog change and the log entry", docsApplyWrites},
		{"ApplyLogged with zero or two fields writes nothing", docsApplyInvalidChange},
		{"ApplyLogged with conflicting log ID applies nothing", docsApplyLogConflict},
		{"ApplyLogged replay writes nothing new", docsApplyReplay},
		{"ApplyLogged with conflicting policy applies nothing", docsApplyPolicyConflict},
		{"ApplyLogged applies a bill change", docsApplyBill},
		{"ApplyLogged applies a policy change", docsApplyPolicy},
		{"ApplyLogged applies an explanation change", docsApplyExplanation},
	}
}

// docsNoErr fails the test when err is not nil.
func docsNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// docsWantErr fails the test unless err matches want with errors.Is.
func docsWantErr(t *testing.T, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

// docsSampleCatalog returns a catalog entry for bill n.
func docsSampleCatalog(n int, title string, tracked bool) data.CatalogEntry {
	return data.CatalogEntry{
		Bill:              SampleBill(n),
		Title:             title,
		Status:            "introduced",
		Chamber:           data.ChamberHouse,
		Subjects:          []string{"Health"},
		CommitteeActivity: true,
		Tracked:           tracked,
		Reasons:           []string{"sponsor"},
		MarkupStatus:      data.MarkupEligible,
		OfficialLinks:     []data.Link{{Label: "Congress.gov", URL: "https://example.test/bill", Kind: data.LinkBillPage}},
		Meta:              SampleMeta(),
	}
}

// docsSamplePolicy returns a policy version with the given rules.
func docsSamplePolicy(version int, rules ...string) data.SelectionPolicy {
	return data.SelectionPolicy{Version: version, Rules: rules, Meta: SampleMeta()}
}

// docsSampleExplanation returns an explanation of version ih of bill 1.
func docsSampleExplanation(section []string, provision, what string) data.ChangeExplanation {
	return data.ChangeExplanation{
		Version:     SampleVersion(1, "ih").Ref,
		Section:     section,
		ProvisionID: provision,
		WhatChanges: what,
		WhyMatters:  "because",
		Flags:       []data.Provenance{data.ProvenanceAuto},
		Meta:        SampleMeta(),
	}
}

// docsSampleLog returns a log entry with the given ID, time, and reason.
func docsSampleLog(id string, at time.Time, reason string) data.LogEntry {
	return data.LogEntry{ID: id, At: at, Kind: "moderation", Subject: "us/119/hr/1", Reason: reason, Meta: SampleMeta()}
}

// docsCatalogPager returns a page function over the catalog with page size 2.
func docsCatalogPager(ctx context.Context, pub data.PublicDocs) func(string) (data.Paged[data.CatalogEntry], error) {
	return func(cursor string) (data.Paged[data.CatalogEntry], error) {
		return pub.ListCatalog(ctx, data.Page{Cursor: cursor, Limit: docsPageSize})
	}
}

// docsLogPager returns a page function over the log with page size 2.
func docsLogPager(ctx context.Context, pub data.PublicDocs) func(string) (data.Paged[data.LogEntry], error) {
	return func(cursor string) (data.Paged[data.LogEntry], error) {
		return pub.ListLog(ctx, data.Page{Cursor: cursor, Limit: docsPageSize})
	}
}

// docsDrain follows Next from cursor until it is empty and returns the items
// of each page. It fails the test if the list does not end within 10 pages.
func docsDrain[T any](t *testing.T, cursor string, list func(cursor string) (data.Paged[T], error)) [][]T {
	t.Helper()
	var pages [][]T
	for range 10 {
		page, err := list(cursor)
		docsNoErr(t, err)
		pages = append(pages, page.Items)
		if page.Next == "" {
			return pages
		}
		cursor = page.Next
	}
	t.Fatalf("list did not end within 10 pages")
	return nil
}

// docsNumbers returns the bill numbers of catalog entries in order.
func docsNumbers(items []data.CatalogEntry) []int {
	out := make([]int, 0, len(items))
	for _, e := range items {
		out = append(out, e.Bill.Number)
	}
	return out
}

func docsBillUnknown(t *testing.T, factory docsFactory) {
	t.Helper()
	pub, _, _ := factory(t)
	_, err := pub.GetBill(t.Context(), SampleBill(1))
	docsWantErr(t, err, data.ErrNotFound)
}

func docsBillRoundTrip(t *testing.T, factory docsFactory) {
	t.Helper()
	ctx := t.Context()
	pub, plat, _ := factory(t)
	want := data.Bill{
		Ref:            SampleBill(1),
		Title:          "A bill",
		Sponsor:        "Rep. Example",
		Status:         docsIntroduced,
		Chamber:        data.ChamberHouse,
		ReviewState:    data.ReviewOpen,
		OfficialLinks:  []data.Link{{Label: "Congress.gov", URL: "https://example.test/bill", Kind: data.LinkBillPage}},
		TrackedReasons: []string{"sponsor"},
		PolicyVersion:  1,
		Versions: []data.VersionSummary{{
			Ref:         SampleVersion(1, "ih").Ref,
			Type:        docsIntroduced,
			PublishedAt: SampleVersion(1, "ih").PublishedAt,
		}},
		Meta: SampleMeta(),
	}
	docsNoErr(t, plat.PutBill(ctx, want))
	got, err := pub.GetBill(ctx, SampleBill(1))
	docsNoErr(t, err)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetBill = %+v, want %+v", got, want)
	}
}

func docsBillInvalid(t *testing.T, factory docsFactory) {
	t.Helper()
	_, plat, _ := factory(t)
	err := plat.PutBill(t.Context(), data.Bill{Ref: data.BillRef{}})
	docsWantErr(t, err, data.ErrInvalid)
}

func docsBillReplace(t *testing.T, factory docsFactory) {
	t.Helper()
	ctx := t.Context()
	pub, plat, _ := factory(t)
	docsNoErr(t, plat.PutBill(ctx, data.Bill{Ref: SampleBill(1), Title: "first", Meta: SampleMeta()}))
	second := data.Bill{Ref: SampleBill(1), Title: "second", Meta: SampleMeta()}
	docsNoErr(t, plat.PutBill(ctx, second))
	got, err := pub.GetBill(ctx, SampleBill(1))
	docsNoErr(t, err)
	if !reflect.DeepEqual(got, second) {
		t.Fatalf("GetBill = %+v, want %+v", got, second)
	}
}

func docsPolicyEmpty(t *testing.T, factory docsFactory) {
	t.Helper()
	pub, _, _ := factory(t)
	_, err := pub.GetSelectionPolicy(t.Context())
	docsWantErr(t, err, data.ErrNotFound)
}

func docsPolicyLatest(t *testing.T, factory docsFactory) {
	t.Helper()
	ctx := t.Context()
	pub, plat, _ := factory(t)
	docsNoErr(t, plat.PutSelectionPolicy(ctx, docsSamplePolicy(1, "a")))
	v2 := docsSamplePolicy(docsNextVersion, "a", "b")
	docsNoErr(t, plat.PutSelectionPolicy(ctx, v2))
	got, err := pub.GetSelectionPolicy(ctx)
	docsNoErr(t, err)
	if !reflect.DeepEqual(got, v2) {
		t.Fatalf("GetSelectionPolicy = %+v, want %+v", got, v2)
	}
}

func docsPolicyByVersion(t *testing.T, factory docsFactory) {
	t.Helper()
	ctx := t.Context()
	pub, plat, _ := factory(t)
	v1 := docsSamplePolicy(1, "a")
	docsNoErr(t, plat.PutSelectionPolicy(ctx, v1))
	docsNoErr(t, plat.PutSelectionPolicy(ctx, docsSamplePolicy(docsNextVersion, "a", "b")))
	got, err := pub.GetSelectionPolicyVersion(ctx, 1)
	docsNoErr(t, err)
	if !reflect.DeepEqual(got, v1) {
		t.Fatalf("GetSelectionPolicyVersion(1) = %+v, want %+v", got, v1)
	}
	_, err = pub.GetSelectionPolicyVersion(ctx, docsNextVersion+1)
	docsWantErr(t, err, data.ErrNotFound)
}

func docsPolicyWriteOnce(t *testing.T, factory docsFactory) {
	t.Helper()
	ctx := t.Context()
	_, plat, _ := factory(t)
	v1 := docsSamplePolicy(1, "a")
	docsNoErr(t, plat.PutSelectionPolicy(ctx, v1))
	docsNoErr(t, plat.PutSelectionPolicy(ctx, v1))
	err := plat.PutSelectionPolicy(ctx, docsSamplePolicy(1, "other"))
	docsWantErr(t, err, data.ErrConflict)
}

func docsPolicyInvalidVersion(t *testing.T, factory docsFactory) {
	t.Helper()
	_, plat, _ := factory(t)
	err := plat.PutSelectionPolicy(t.Context(), docsSamplePolicy(0, "a"))
	docsWantErr(t, err, data.ErrInvalid)
}

func docsCatalogPages(t *testing.T, factory docsFactory) {
	t.Helper()
	ctx := t.Context()
	pub, plat, _ := factory(t)
	for _, n := range []int{1, 2, 3, 10, 11} {
		docsNoErr(t, plat.PutCatalogEntry(ctx, docsSampleCatalog(n, "t", false)))
	}
	pages := docsDrain(t, "", docsCatalogPager(ctx, pub))
	sizes := make([]int, 0, len(pages))
	got := make([]int, 0, len(pages)*docsPageSize)
	for _, p := range pages {
		sizes = append(sizes, len(p))
		got = append(got, docsNumbers(p)...)
	}
	if want := []int{2, 2, 1}; !reflect.DeepEqual(sizes, want) {
		t.Fatalf("page sizes = %v, want %v", sizes, want)
	}
	if want := []int{1, 2, 3, 10, 11}; !reflect.DeepEqual(got, want) {
		t.Fatalf("catalog order = %v, want %v", got, want)
	}
}

func docsCatalogCursorStable(t *testing.T, factory docsFactory) {
	t.Helper()
	ctx := t.Context()
	pub, plat, _ := factory(t)
	for _, n := range []int{1, 2, 3, 10, 11} {
		docsNoErr(t, plat.PutCatalogEntry(ctx, docsSampleCatalog(n, "t", false)))
	}
	first, err := pub.ListCatalog(ctx, data.Page{Limit: docsPageSize})
	docsNoErr(t, err)
	docsNoErr(t, plat.PutCatalogEntry(ctx, docsSampleCatalog(docsBillBetween, "t", false)))
	got := docsNumbers(first.Items)
	for _, p := range docsDrain(t, first.Next, docsCatalogPager(ctx, pub)) {
		got = append(got, docsNumbers(p)...)
	}
	seen := map[int]bool{}
	for _, n := range got {
		if seen[n] {
			t.Fatalf("bill %d repeated across pages: %v", n, got)
		}
		seen[n] = true
	}
	for _, want := range []int{1, 2, 3, 10, 11} {
		if !seen[want] {
			t.Fatalf("bill %d skipped: %v", want, got)
		}
	}
}

func docsCatalogEmpty(t *testing.T, factory docsFactory) {
	t.Helper()
	pub, _, _ := factory(t)
	page, err := pub.ListCatalog(t.Context(), data.Page{})
	docsNoErr(t, err)
	if len(page.Items) != 0 || page.Next != "" {
		t.Fatalf("empty catalog page = %+v, want no items and no Next", page)
	}
}

func docsCatalogInvalidCursor(t *testing.T, factory docsFactory) {
	t.Helper()
	pub, _, _ := factory(t)
	_, err := pub.ListCatalog(t.Context(), data.Page{Cursor: "!!not-a-cursor!!"})
	docsWantErr(t, err, data.ErrInvalid)
}

func docsCatalogReplace(t *testing.T, factory docsFactory) {
	t.Helper()
	ctx := t.Context()
	pub, plat, _ := factory(t)
	docsNoErr(t, plat.PutCatalogEntry(ctx, docsSampleCatalog(1, "old", false)))
	docsNoErr(t, plat.PutCatalogEntry(ctx, docsSampleCatalog(1, "new", false)))
	page, err := pub.ListCatalog(ctx, data.Page{})
	docsNoErr(t, err)
	if len(page.Items) != 1 || page.Items[0].Title != "new" {
		t.Fatalf("catalog = %+v, want one entry titled new", page.Items)
	}
}

func docsCatalogTracked(t *testing.T, factory docsFactory) {
	t.Helper()
	ctx := t.Context()
	pub, plat, _ := factory(t)
	docsNoErr(t, plat.PutCatalogEntry(ctx, docsSampleCatalog(1, "t", true)))
	page, err := pub.ListCatalog(ctx, data.Page{})
	docsNoErr(t, err)
	if len(page.Items) != 1 || !page.Items[0].Tracked {
		t.Fatalf("catalog = %+v, want one tracked entry", page.Items)
	}
	docsNoErr(t, plat.PutCatalogEntry(ctx, docsSampleCatalog(1, "t", false)))
	page, err = pub.ListCatalog(ctx, data.Page{})
	docsNoErr(t, err)
	if len(page.Items) != 1 || page.Items[0].Tracked {
		t.Fatalf("catalog = %+v, want one untracked entry", page.Items)
	}
}

func docsExplanationsNone(t *testing.T, factory docsFactory) {
	t.Helper()
	pub, _, _ := factory(t)
	got, err := pub.GetExplanations(t.Context(), SampleVersion(1, "ih").Ref)
	docsNoErr(t, err)
	if len(got) != 0 {
		t.Fatalf("GetExplanations = %+v, want none", got)
	}
}

func docsExplanationsReplace(t *testing.T, factory docsFactory) {
	t.Helper()
	ctx := t.Context()
	pub, plat, _ := factory(t)
	docsNoErr(t, plat.PutExplanation(ctx, docsSampleExplanation([]string{"s1"}, "p1", "first")))
	docsNoErr(t, plat.PutExplanation(ctx, docsSampleExplanation([]string{"s1"}, "p1", "second")))
	got, err := pub.GetExplanations(ctx, SampleVersion(1, "ih").Ref)
	docsNoErr(t, err)
	if len(got) != 1 || got[0].WhatChanges != "second" {
		t.Fatalf("GetExplanations = %+v, want one explanation with second", got)
	}
}

func docsExplanationsOrder(t *testing.T, factory docsFactory) {
	t.Helper()
	ctx := t.Context()
	pub, plat, _ := factory(t)
	docsNoErr(t, plat.PutExplanation(ctx, docsSampleExplanation([]string{"s2"}, "p1", "c")))
	docsNoErr(t, plat.PutExplanation(ctx, docsSampleExplanation([]string{"s1"}, "p2", "b")))
	docsNoErr(t, plat.PutExplanation(ctx, docsSampleExplanation([]string{"s1"}, "p1", "a")))
	got, err := pub.GetExplanations(ctx, SampleVersion(1, "ih").Ref)
	docsNoErr(t, err)
	order := make([]string, 0, len(got))
	for _, x := range got {
		order = append(order, x.WhatChanges)
	}
	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("explanation order = %v, want %v", order, want)
	}
}

func docsExplanationsInvalidVersion(t *testing.T, factory docsFactory) {
	t.Helper()
	ctx := t.Context()
	pub, plat, _ := factory(t)
	_, err := pub.GetExplanations(ctx, data.VersionRef{})
	docsWantErr(t, err, data.ErrInvalid)
	err = plat.PutExplanation(ctx, data.ChangeExplanation{})
	docsWantErr(t, err, data.ErrInvalid)
}

func docsGenerationWriteOnce(t *testing.T, factory docsFactory) {
	t.Helper()
	ctx := t.Context()
	_, plat, _ := factory(t)
	g := data.GenerationRecord{
		ID: "g1", Task: "summarize", Model: "model-x", PromptHash: "h1",
		Outcome: "ok", At: SampleMeta().AsOf, TokensIn: 1, TokensOut: 1, Meta: SampleMeta(),
	}
	docsNoErr(t, plat.PutGeneration(ctx, g))
	docsNoErr(t, plat.PutGeneration(ctx, g))
	other := g
	other.Outcome = "refusal"
	docsWantErr(t, plat.PutGeneration(ctx, other), data.ErrConflict)
}

func docsGenerationEmptyID(t *testing.T, factory docsFactory) {
	t.Helper()
	_, plat, _ := factory(t)
	docsWantErr(t, plat.PutGeneration(t.Context(), data.GenerationRecord{}), data.ErrInvalid)
}

func docsIngestUnknown(t *testing.T, factory docsFactory) {
	t.Helper()
	_, plat, _ := factory(t)
	_, err := plat.GetIngestState(t.Context(), "congress")
	docsWantErr(t, err, data.ErrNotFound)
}

func docsIngestRoundTrip(t *testing.T, factory docsFactory) {
	t.Helper()
	ctx := t.Context()
	_, plat, _ := factory(t)
	want := data.IngestState{Source: "congress", Watermark: SampleMeta().AsOf, Meta: SampleMeta()}
	docsNoErr(t, plat.PutIngestState(ctx, want))
	got, err := plat.GetIngestState(ctx, "congress")
	docsNoErr(t, err)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetIngestState = %+v, want %+v", got, want)
	}
}

func docsIngestEmptySource(t *testing.T, factory docsFactory) {
	t.Helper()
	_, plat, _ := factory(t)
	docsWantErr(t, plat.PutIngestState(t.Context(), data.IngestState{}), data.ErrInvalid)
}

func docsLogRecordList(t *testing.T, factory docsFactory) {
	t.Helper()
	ctx := t.Context()
	pub, _, mod := factory(t)
	e := docsSampleLog("l1", SampleMeta().AsOf, "spam")
	docsNoErr(t, mod.Record(ctx, e))
	page, err := pub.ListLog(ctx, data.Page{})
	docsNoErr(t, err)
	if len(page.Items) != 1 || !reflect.DeepEqual(page.Items[0], e) {
		t.Fatalf("ListLog = %+v, want %+v", page.Items, e)
	}
}

func docsLogNewestFirst(t *testing.T, factory docsFactory) {
	t.Helper()
	ctx := t.Context()
	pub, _, mod := factory(t)
	base := SampleMeta().AsOf
	hour := base.Add(time.Hour)
	twoHours := hour.Add(time.Hour)
	threeHours := twoHours.Add(time.Hour)
	entries := []data.LogEntry{
		docsSampleLog("e1", base, "r"),
		docsSampleLog("e2", hour, "r"),
		docsSampleLog("e3", hour, "r"),
		docsSampleLog("e4", twoHours, "r"),
		docsSampleLog("e5", threeHours, "r"),
	}
	for _, e := range entries {
		docsNoErr(t, mod.Record(ctx, e))
	}
	pages := docsDrain(t, "", docsLogPager(ctx, pub))
	got := make([]string, 0, len(entries))
	sizes := make([]int, 0, len(pages))
	for _, p := range pages {
		sizes = append(sizes, len(p))
		for _, e := range p {
			got = append(got, e.ID)
		}
	}
	if want := []int{2, 2, 1}; !reflect.DeepEqual(sizes, want) {
		t.Fatalf("page sizes = %v, want %v", sizes, want)
	}
	if want := []string{"e5", "e4", "e3", "e2", "e1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ListLog order = %v, want %v", got, want)
	}
}

func docsLogIdempotent(t *testing.T, factory docsFactory) {
	t.Helper()
	ctx := t.Context()
	pub, _, mod := factory(t)
	e := docsSampleLog("l1", SampleMeta().AsOf, "r")
	docsNoErr(t, mod.Record(ctx, e))
	docsNoErr(t, mod.Record(ctx, e))
	page, err := pub.ListLog(ctx, data.Page{})
	docsNoErr(t, err)
	if len(page.Items) != 1 {
		t.Fatalf("ListLog has %d entries, want 1", len(page.Items))
	}
}

func docsLogConflict(t *testing.T, factory docsFactory) {
	t.Helper()
	ctx := t.Context()
	_, _, mod := factory(t)
	docsNoErr(t, mod.Record(ctx, docsSampleLog("l1", SampleMeta().AsOf, "a")))
	err := mod.Record(ctx, docsSampleLog("l1", SampleMeta().AsOf, "b"))
	docsWantErr(t, err, data.ErrConflict)
}

func docsLogEmptyID(t *testing.T, factory docsFactory) {
	t.Helper()
	_, _, mod := factory(t)
	docsWantErr(t, mod.Record(t.Context(), data.LogEntry{}), data.ErrInvalid)
}

func docsApplyWrites(t *testing.T, factory docsFactory) {
	t.Helper()
	ctx := t.Context()
	pub, plat, _ := factory(t)
	entry := docsSampleCatalog(1, "t", true)
	e := docsSampleLog("l1", SampleMeta().AsOf, "track")
	docsNoErr(t, plat.ApplyLogged(ctx, data.Change{Catalog: &entry}, e))
	cat, err := pub.ListCatalog(ctx, data.Page{})
	docsNoErr(t, err)
	if len(cat.Items) != 1 || !reflect.DeepEqual(cat.Items[0], entry) {
		t.Fatalf("catalog = %+v, want %+v", cat.Items, entry)
	}
	logs, err := pub.ListLog(ctx, data.Page{})
	docsNoErr(t, err)
	if len(logs.Items) != 1 || !reflect.DeepEqual(logs.Items[0], e) {
		t.Fatalf("log = %+v, want %+v", logs.Items, e)
	}
}

func docsApplyInvalidChange(t *testing.T, factory docsFactory) {
	t.Helper()
	ctx := t.Context()
	pub, plat, _ := factory(t)
	e := docsSampleLog("l1", SampleMeta().AsOf, "x")
	docsWantErr(t, plat.ApplyLogged(ctx, data.Change{}, e), data.ErrInvalid)
	entry := docsSampleCatalog(1, "t", false)
	bill := data.Bill{Ref: SampleBill(1), Meta: SampleMeta()}
	docsWantErr(t, plat.ApplyLogged(ctx, data.Change{Catalog: &entry, Bill: &bill}, e), data.ErrInvalid)
	logs, err := pub.ListLog(ctx, data.Page{})
	docsNoErr(t, err)
	if len(logs.Items) != 0 {
		t.Fatalf("log = %+v, want nothing written", logs.Items)
	}
	cat, err := pub.ListCatalog(ctx, data.Page{})
	docsNoErr(t, err)
	if len(cat.Items) != 0 {
		t.Fatalf("catalog = %+v, want nothing written", cat.Items)
	}
}

func docsApplyLogConflict(t *testing.T, factory docsFactory) {
	t.Helper()
	ctx := t.Context()
	pub, plat, mod := factory(t)
	docsNoErr(t, mod.Record(ctx, docsSampleLog("l1", SampleMeta().AsOf, "a")))
	entry := docsSampleCatalog(1, "t", false)
	err := plat.ApplyLogged(ctx, data.Change{Catalog: &entry}, docsSampleLog("l1", SampleMeta().AsOf, "b"))
	docsWantErr(t, err, data.ErrConflict)
	cat, err := pub.ListCatalog(ctx, data.Page{})
	docsNoErr(t, err)
	if len(cat.Items) != 0 {
		t.Fatalf("catalog = %+v, want the change not applied", cat.Items)
	}
}

func docsApplyReplay(t *testing.T, factory docsFactory) {
	t.Helper()
	ctx := t.Context()
	pub, plat, _ := factory(t)
	entry := docsSampleCatalog(1, "t", false)
	e := docsSampleLog("l1", SampleMeta().AsOf, "x")
	docsNoErr(t, plat.ApplyLogged(ctx, data.Change{Catalog: &entry}, e))
	docsNoErr(t, plat.ApplyLogged(ctx, data.Change{Catalog: &entry}, e))
	cat, err := pub.ListCatalog(ctx, data.Page{})
	docsNoErr(t, err)
	logs, err := pub.ListLog(ctx, data.Page{})
	docsNoErr(t, err)
	if len(cat.Items) != 1 || len(logs.Items) != 1 {
		t.Fatalf("after replay catalog=%d log=%d, want 1 and 1", len(cat.Items), len(logs.Items))
	}
}

func docsApplyPolicyConflict(t *testing.T, factory docsFactory) {
	t.Helper()
	ctx := t.Context()
	pub, plat, _ := factory(t)
	docsNoErr(t, plat.PutSelectionPolicy(ctx, docsSamplePolicy(1, "a")))
	conflicting := docsSamplePolicy(1, "b")
	err := plat.ApplyLogged(ctx, data.Change{Policy: &conflicting}, docsSampleLog("l9", SampleMeta().AsOf, "x"))
	docsWantErr(t, err, data.ErrConflict)
	logs, err := pub.ListLog(ctx, data.Page{})
	docsNoErr(t, err)
	if len(logs.Items) != 0 {
		t.Fatalf("log = %+v, want the entry not written", logs.Items)
	}
}

func docsApplyBill(t *testing.T, factory docsFactory) {
	t.Helper()
	ctx := t.Context()
	pub, plat, _ := factory(t)
	b := data.Bill{Ref: SampleBill(1), Title: "applied", Meta: SampleMeta()}
	docsNoErr(t, plat.ApplyLogged(ctx, data.Change{Bill: &b}, docsSampleLog("l1", SampleMeta().AsOf, "x")))
	got, err := pub.GetBill(ctx, SampleBill(1))
	docsNoErr(t, err)
	if !reflect.DeepEqual(got, b) {
		t.Fatalf("GetBill = %+v, want %+v", got, b)
	}
}

func docsApplyPolicy(t *testing.T, factory docsFactory) {
	t.Helper()
	ctx := t.Context()
	pub, plat, _ := factory(t)
	p := docsSamplePolicy(1, "a")
	docsNoErr(t, plat.ApplyLogged(ctx, data.Change{Policy: &p}, docsSampleLog("l1", SampleMeta().AsOf, "x")))
	got, err := pub.GetSelectionPolicy(ctx)
	docsNoErr(t, err)
	if !reflect.DeepEqual(got, p) {
		t.Fatalf("GetSelectionPolicy = %+v, want %+v", got, p)
	}
}

func docsApplyExplanation(t *testing.T, factory docsFactory) {
	t.Helper()
	ctx := t.Context()
	pub, plat, _ := factory(t)
	x := docsSampleExplanation([]string{"s1"}, "p1", "applied")
	docsNoErr(t, plat.ApplyLogged(ctx, data.Change{Explanation: &x}, docsSampleLog("l1", SampleMeta().AsOf, "x")))
	got, err := pub.GetExplanations(ctx, x.Version)
	docsNoErr(t, err)
	if len(got) != 1 || !reflect.DeepEqual(got[0], x) {
		t.Fatalf("GetExplanations = %+v, want %+v", got, x)
	}
}
