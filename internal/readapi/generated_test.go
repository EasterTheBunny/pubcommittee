package readapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"
	"time"

	apiv1 "github.com/easterthebunny/pubcommittee/api/v1"
	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/billxml"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/readapi/seed"
	"github.com/easterthebunny/pubcommittee/internal/versiondiff"
)

// trackedVersions returns the versions of the tracked seed bill, oldest first.
func trackedVersions(t *testing.T, store *memory.Store) []data.VersionSummary {
	t.Helper()
	ref, err := data.ParseBillID(seed.TrackedBill)
	if err != nil {
		t.Fatal(err)
	}
	vs, err := store.ListVersions(context.Background(), ref)
	if err != nil || len(vs) == 0 {
		t.Fatalf("ListVersions = %d, %v", len(vs), err)
	}
	return vs
}

// publishOutput stores a ready output for task on v and makes it current
// for target, as the generation step would.
func publishOutput(t *testing.T, store *memory.Store, task string, v data.VersionRef, target, headline string) {
	t.Helper()
	ctx := context.Background()
	o := data.GeneratedOutput{
		Key: data.GenKey{
			Task: task, Version: v, Target: target, PromptVersion: "p1", Model: "fake",
		},
		Status:      data.GenReady,
		Headline:    headline,
		Statements:  []data.Statement{{Text: "The bill sets a new limit.", Anchors: []string{"1-l1"}}},
		ReviewState: data.ReviewUnreviewed,
		Meta: data.Meta{
			Version: 1, AsOf: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
			Provenance: data.ProvenanceAuto, ContentClass: data.ContentPlatform,
		},
	}
	gs := store.PlatformPorts().GeneratedStore
	if err := gs.PutGenerated(ctx, o); err != nil {
		t.Fatal(err)
	}
	if err := gs.SetCurrent(ctx, o); err != nil {
		t.Fatal(err)
	}
}

// checkProblem asserts a 404 problem+json reply.
func checkProblem(t *testing.T, u string, got reply) {
	t.Helper()
	if got.status != http.StatusNotFound {
		t.Errorf("%s: status %d, want 404", u, got.status)
	}
	if ct := got.header.Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("%s: Content-Type %q, want application/problem+json", u, ct)
	}
}

// checkGeneratedStatement asserts the one statement stored by publishOutput.
func checkGeneratedStatement(t *testing.T, got []apiv1.Statement) {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("statements = %d, want 1", len(got))
	}
	st := got[0]
	if st.Text != "The bill sets a new limit." || !slices.Equal(st.Anchors, []string{"1-l1"}) {
		t.Errorf("statement = %+v", st)
	}
	if st.ContentClass != apiv1.StatementContentClassPlatform {
		t.Errorf("statement content_class %q, want platform", st.ContentClass)
	}
}

func TestGeneratedSummariesNotFoundBeforeOutput(t *testing.T) {
	t.Parallel()
	_, base := citationsStore(t)
	for _, u := range []string{
		base + "/v1/bills/" + seed.TrackedBill + "/v/1/summary",
		base + "/v1/bills/" + seed.TrackedBill + "/v/1/changes",
	} {
		checkProblem(t, u, do(t, http.MethodGet, u, nil))
	}
}

func TestBillSummaryFromStoredOutput(t *testing.T) {
	t.Parallel()
	store, base := citationsStore(t)
	vs := trackedVersions(t, store)
	publishOutput(t, store, data.TaskBillSummary, vs[0].Ref, "", "A short headline.")

	u := base + "/v1/bills/" + seed.TrackedBill + "/v/1/summary"
	got := do(t, http.MethodGet, u, nil)
	if got.status != http.StatusOK {
		t.Fatalf("status %d: %s", got.status, got.body)
	}
	var doc apiv1.BillSummaryDoc
	if err := json.Unmarshal(got.body, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Bill != seed.TrackedBill || doc.N != 1 || doc.Headline != "A short headline." {
		t.Errorf("bill %q n %d headline %q", doc.Bill, doc.N, doc.Headline)
	}
	checkGeneratedStatement(t, doc.Statements)
	if doc.ContentClass != apiv1.TrustedContentClass("platform") || doc.Provenance != apiv1.Provenance("auto") {
		t.Errorf("content_class %q provenance %q", doc.ContentClass, doc.Provenance)
	}
	if doc.Flags == nil || !slices.Equal(*doc.Flags, []apiv1.BillSummaryDocFlags{apiv1.BillSummaryDocFlagsAuto}) {
		t.Errorf("flags = %v, want [auto]", doc.Flags)
	}
}

func TestVersionChangesFromStoredOutput(t *testing.T) {
	t.Parallel()
	store, base := citationsStore(t)
	vs := trackedVersions(t, store)
	// The tracked seed bill has two versions. A bill with one version has no
	// change summary to store, so this test needs at least two.
	if len(vs) < 2 {
		t.Skip("seed bill has fewer than two versions")
	}
	first := base + "/v1/bills/" + seed.TrackedBill + "/v/1/changes"
	checkProblem(t, first, do(t, http.MethodGet, first, nil))

	publishOutput(t, store, data.TaskVersionChanges, vs[1].Ref, vs[0].Ref.Version, "The second version changes the limit.")

	u := base + "/v1/bills/" + seed.TrackedBill + "/v/2/changes"
	got := do(t, http.MethodGet, u, nil)
	if got.status != http.StatusOK {
		t.Fatalf("status %d: %s", got.status, got.body)
	}
	var doc apiv1.VersionChangeSummary
	if err := json.Unmarshal(got.body, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Bill != seed.TrackedBill || doc.FromN != 1 || doc.ToN != 2 {
		t.Errorf("bill %q from_n %d to_n %d; want %q 1 2", doc.Bill, doc.FromN, doc.ToN, seed.TrackedBill)
	}
	if doc.Headline != "The second version changes the limit." {
		t.Errorf("headline %q", doc.Headline)
	}
	checkGeneratedStatement(t, doc.Statements)
	if doc.ContentClass != apiv1.TrustedContentClass("platform") || doc.Provenance != apiv1.Provenance("auto") {
		t.Errorf("content_class %q provenance %q", doc.ContentClass, doc.Provenance)
	}
	checkProblem(t, first, do(t, http.MethodGet, first, nil))
}

// A version whose text equals the previous one gets the fixed no-differences
// answer from its stored comparison; one with changes and no published
// summary stays not found.
func TestVersionChangesNoDifferences(t *testing.T) {
	t.Parallel()
	store, base := citationsStore(t)
	vs := trackedVersions(t, store)
	if len(vs) < 2 {
		t.Skip("seed bill has fewer than two versions")
	}
	u := base + "/v1/bills/" + seed.TrackedBill + "/v/2/changes"
	checkProblem(t, u, do(t, http.MethodGet, u, nil))

	asOf := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	putUnchangedComparison(t, store, vs[0].Ref, vs[1].Ref, asOf)
	got := do(t, http.MethodGet, u, nil)
	if got.status != http.StatusOK {
		t.Fatalf("status %d: %s", got.status, got.body)
	}
	var doc apiv1.VersionChangeSummary
	if err := json.Unmarshal(got.body, &doc); err != nil {
		t.Fatal(err)
	}
	checkNoDifferences(t, doc, asOf)
}

// putUnchangedComparison stores a comparison of from and to with no changes.
func putUnchangedComparison(t *testing.T, store *memory.Store, from, to data.VersionRef, asOf time.Time) {
	t.Helper()
	if err := store.PutComparison(context.Background(), data.Comparison{
		Ref: data.ComparisonRef{
			Bill: from.Bill, From: from.Version, To: to.Version, Method: versiondiff.Method(billxml.ParserVersion),
		},
		Sections: []data.SectionComparison{{Path: []string{"1"}, Status: "unchanged"}},
		Meta:     data.Meta{Version: 1, AsOf: asOf, Provenance: data.ProvenanceSource, ContentClass: data.ContentOfficial},
	}); err != nil {
		t.Fatal(err)
	}
}

// checkNoDifferences asserts the fixed answer for version 2 against version 1.
func checkNoDifferences(t *testing.T, doc apiv1.VersionChangeSummary, asOf time.Time) {
	t.Helper()
	if doc.FromN != 1 || doc.ToN != 2 || doc.Headline != "No text changes from version 1." || len(doc.Statements) != 0 {
		t.Errorf("doc = %+v", doc)
	}
	want := []apiv1.VersionChangeSummaryFlags{apiv1.VersionChangeSummaryFlagsNoDifferences}
	if doc.Flags == nil || !slices.Equal(*doc.Flags, want) {
		t.Errorf("flags = %v, want %v", doc.Flags, want)
	}
	if doc.ContentClass != apiv1.TrustedContentClass(data.ContentPlatform) || !doc.AsOf.Equal(asOf) {
		t.Errorf("content_class %q as_of %v", doc.ContentClass, doc.AsOf)
	}
}

// A summary the grader marked contradicted is withheld: the route answers not found.
func TestBillSummaryWithheldWhenContradicted(t *testing.T) {
	t.Parallel()
	store, base := citationsStore(t)
	vs := trackedVersions(t, store)
	publishOutput(t, store, data.TaskBillSummary, vs[0].Ref, "", "A short headline.")
	key := data.GenKey{
		Task: data.TaskBillSummary, Version: vs[0].Ref, PromptVersion: "p1", Model: "fake",
	}
	if err := store.PlatformPorts().GeneratedStore.PutGrade(context.Background(), data.Grade{
		Key: key, GraderModel: "grader", PromptVersion: "g1", Neutral: true,
		Statements: []data.StatementGrade{{Index: 0, Label: data.GradeContradicted}},
	}); err != nil {
		t.Fatal(err)
	}
	got := do(t, http.MethodGet, base+"/v1/bills/"+seed.TrackedBill+"/v/1/summary", nil)
	if got.status != http.StatusNotFound {
		t.Fatalf("status %d: %s", got.status, got.body)
	}
}
