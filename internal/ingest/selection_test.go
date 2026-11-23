package ingest_test

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/ingest"
	"github.com/easterthebunny/pubcommittee/internal/selection"
)

// catalogRefs lists the bills of the 200-entry catalog sample.
func catalogRefs(t *testing.T) []data.BillRef {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "fixtures", "catalog", "us-*.xml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("catalog fixtures: %v (%d files)", err, len(files))
	}
	refs := make([]data.BillRef, 0, len(files))
	for _, f := range files {
		ref, err := data.ParseBillID(strings.TrimSuffix(filepath.Base(f), ".xml"))
		if err != nil {
			t.Fatalf("fixture name %s: %v", f, err)
		}
		refs = append(refs, ref)
	}
	return refs
}

// storeCatalog pulls the whole sample into a fresh store (catalog entries only,
// nothing tracked) and publishes policy v1.
func storeCatalog(t *testing.T) (*memory.Store, ingest.Deps, data.SelectionPolicy) {
	t.Helper()
	store := memory.New()
	d := deps(store, fixtureSource(t, fixedTime()), fixedTime())
	for _, ref := range catalogRefs(t) {
		pull(t, d, ingest.Options{}, ref)
	}
	p, err := selection.V1(fixedTime())
	if err != nil {
		t.Fatal(err)
	}
	ports := store.PlatformPorts()
	if err := selection.Publish(t.Context(), struct {
		data.PublicDocs
		data.PlatformDocs
	}{ports.PublicDocs, ports.PlatformDocs}, p, "initial", fixedTime()); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	return store, d, p
}

func allEntries(t *testing.T, store *memory.Store) []data.CatalogEntry {
	t.Helper()
	cat, err := ingest.LoadCatalog(t.Context(), store)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]data.CatalogEntry, 0, len(cat))
	for _, e := range cat {
		out = append(out, e)
	}
	return out
}

func TestEvaluateAllOverCatalogSample(t *testing.T) {
	t.Parallel()
	store, d, p := storeCatalog(t)
	rep, err := ingest.EvaluateAll(t.Context(), d, p, false)
	if err != nil {
		t.Fatalf("EvaluateAll: %v", err)
	}
	entries := allEntries(t, store)
	if len(entries) != 200 || rep.Evaluated != 200 {
		t.Fatalf("entries = %d, evaluated = %d, want 200", len(entries), rep.Evaluated)
	}
	statuses := map[string]int{}
	for _, e := range entries {
		statuses[e.MarkupStatus]++
		checkEntry(t, e)
	}
	t.Logf("markup statuses over the sample: %v; newly tracked: %v", statuses, rep.NewlyTracked)
	if statuses[data.MarkupNotACandidate] == 0 || statuses[data.MarkupEligible] == 0 {
		t.Errorf("sample shows no variety of statuses: %v", statuses)
	}
	if len(rep.NewlyTracked) != statuses[data.MarkupInReview] {
		t.Errorf("newly tracked = %d, in review = %d", len(rep.NewlyTracked), statuses[data.MarkupInReview])
	}
	checkBreakdown(t, store, statuses[data.MarkupInReview])
	// A second run changes nothing.
	rep2, err := ingest.EvaluateAll(t.Context(), d, p, false)
	if err != nil || len(rep2.Changed) != 0 {
		t.Errorf("second run changed %v (%v)", rep2.Changed, err)
	}
}

// checkBreakdown reads the published breakdown and compares its tracked count.
func checkBreakdown(t *testing.T, store *memory.Store, tracked int) {
	t.Helper()
	raw, err := store.GetFile(t.Context(), selection.BreakdownPath)
	if err != nil {
		t.Fatalf("breakdown not published: %v", err)
	}
	var b selection.Breakdown
	if err := json.Unmarshal(raw, &b); err != nil || b.Tracked != tracked || b.PolicyVersion != 1 {
		t.Errorf("breakdown = %+v (%v)", b, err)
	}
}

func TestNarrowerPolicyKeepsTrackedBills(t *testing.T) {
	t.Parallel()
	store, d, p := storeCatalog(t)
	// Track every bill the sample's committee action allows, so the narrow policy has work to do.
	general := p
	general.Mode = data.PolicyModeGeneral
	if _, err := ingest.EvaluateAll(t.Context(), d, general, false); err != nil {
		t.Fatal(err)
	}
	before := allEntries(t, store)
	trackedBefore := map[data.BillRef]bool{}
	for _, e := range before {
		if e.Tracked {
			trackedBefore[e.Bill] = true
		}
	}
	if len(trackedBefore) == 0 {
		t.Fatal("general mode tracked nothing in the sample; the check below proves nothing")
	}
	if _, err := ingest.EvaluateAll(t.Context(), d, narrowed(p), false); err != nil {
		t.Fatal(err)
	}
	for _, e := range allEntries(t, store) {
		if trackedBefore[e.Bill] && (!e.Tracked || e.PolicyVersion != 1) {
			t.Errorf("%s lost tracking or its policy version under v2: tracked=%v version=%d", e.Bill, e.Tracked, e.PolicyVersion)
		}
	}
}

// evaluatorSteps returns the default pipeline with the policy step of policy p over the stored catalog.
func evaluatorSteps(t *testing.T, store *memory.Store, p data.SelectionPolicy) ingest.Pipeline {
	t.Helper()
	cat, err := ingest.LoadCatalog(t.Context(), store)
	if err != nil {
		t.Fatal(err)
	}
	steps, err := ingest.DefaultSteps().Replace(ingest.StepPolicy, ingest.PolicyStep(ingest.PolicyEvaluator{Policy: p, Catalog: cat}))
	if err != nil {
		t.Fatal(err)
	}
	return steps
}

// narrowed returns policy version 2 with a single narrow topic.
func narrowed(p data.SelectionPolicy) data.SelectionPolicy {
	p.Version = 2
	p.Topics = map[string]data.PolicyTopic{"healthcare": {Subjects: []string{"Medicare"}}}
	return p
}

func TestPullWithPolicyEvaluator(t *testing.T) {
	t.Parallel()
	store, d, p := storeCatalog(t)
	// hr4313 is not in the catalog sample; --track records the manual reason beside any policy reasons.
	rep, err := ingest.Pull(t.Context(), d, ingest.Options{Track: true}, evaluatorSteps(t, store, p), bill(4313))
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	e := entryOf(t, store, bill(4313))
	if !rep.Tracked || !e.Tracked || !slices.Contains(e.Reasons, ingest.ReasonManual) || e.PolicyVersion != 1 ||
		e.TrackedSince != fixedTime().Format("2006-01-02") {
		t.Errorf("entry = tracked %v reasons %v version %d", e.Tracked, e.Reasons, e.PolicyVersion)
	}
	checkLaterPullKeeps(t, store, d, p, e)
}

// checkLaterPullKeeps pulls e's bill again without --track under a narrower
// policy, a day later, and checks the entry is unchanged.
func checkLaterPullKeeps(t *testing.T, store *memory.Store, d ingest.Deps, p data.SelectionPolicy, e data.CatalogEntry) {
	t.Helper()
	d2 := d
	d2.Now = func() time.Time { return fixedTime().Add(24 * time.Hour) }
	if _, err := ingest.Pull(t.Context(), d2, ingest.Options{}, evaluatorSteps(t, store, narrowed(p)), e.Bill); err != nil {
		t.Fatalf("second Pull: %v", err)
	}
	e2 := entryOf(t, store, e.Bill)
	if !e2.Tracked || e2.PolicyVersion != 1 || e2.TrackedSince != e.TrackedSince || !slices.Equal(e2.Reasons, e.Reasons) {
		t.Errorf("second entry = %+v", e2)
	}
}

func TestTrackIgnoredForExcludedKind(t *testing.T) {
	t.Parallel()
	store, d, p := storeCatalog(t)
	steps := evaluatorSteps(t, store, p)
	var hres data.BillRef
	for _, r := range catalogRefs(t) {
		if r.Type == "hres" {
			hres = r
			break
		}
	}
	rep, err := ingest.Pull(t.Context(), d, ingest.Options{Track: true}, steps, hres)
	if err != nil || rep.Tracked {
		t.Fatalf("Pull = %+v, %v; want not tracked", rep, err)
	}
	if e := entryOf(t, store, hres); e.MarkupStatus != data.MarkupNotACandidate || e.ExclusionReason == "" {
		t.Errorf("entry = %+v", e)
	}
}

// entryOf returns the stored catalog entry of ref (all pages).
func entryOf(t *testing.T, store *memory.Store, ref data.BillRef) data.CatalogEntry {
	t.Helper()
	cat, err := ingest.LoadCatalog(t.Context(), store)
	if err != nil {
		t.Fatal(err)
	}
	e, ok := cat[ref]
	if !ok {
		t.Fatalf("no catalog entry for %s", ref)
	}
	return e
}

// checkEntry fails the test when an evaluated entry breaks the rules of its markup status.
func checkEntry(t *testing.T, e data.CatalogEntry) {
	t.Helper()
	var problem string
	switch e.MarkupStatus {
	case data.MarkupInReview:
		problem = trackedProblem(e)
	case data.MarkupEligible:
		if e.Tracked || e.ExclusionReason != "" {
			problem = "eligible but tracked or excluded"
		}
	case data.MarkupNotACandidate:
		if e.Tracked || e.ExclusionReason == "" {
			problem = "not a candidate without a reason, or tracked"
		}
	default:
		problem = "unknown markup status " + e.MarkupStatus
	}
	if problem != "" {
		t.Errorf("%s: %s: %+v", e.Bill, problem, e)
	}
}

// trackedProblem says what a tracked entry lacks, or "".
func trackedProblem(e data.CatalogEntry) string {
	if !e.Tracked || len(e.Reasons) == 0 || e.PolicyVersion != 1 || e.TrackedSince == "" {
		return "tracked without reasons, policy version or date"
	}
	return ""
}
