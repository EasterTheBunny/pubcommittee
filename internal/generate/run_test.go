package generate_test

import (
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/adapters/fakegen"
	"github.com/easterthebunny/pubcommittee/internal/adapters/fixturesrc"
	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/generate"
	"github.com/easterthebunny/pubcommittee/internal/ingest"
	"github.com/easterthebunny/pubcommittee/internal/lawingest"
)

// fixedNow is the clock every test uses, so stored times are stable.
func fixedNow() time.Time {
	return time.Date(2026, time.November, 22, 12, 0, 0, 0, time.UTC)
}

func hr(n int) data.BillRef {
	return data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: n}
}

// seeded returns a memory store holding the three exit bills with their
// citations and comparisons, as job-ingest --law and --track leave them.
func seeded(t *testing.T) *memory.Store {
	t.Helper()
	store := memory.New()
	src, err := fixturesrc.New(fixturesrc.Config{FS: os.DirFS("../../fixtures"), Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}
	titles, err := src.LawTitles(t.Context(), data.ReleasePoint{ID: "119-142"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lawingest.Step(t.Context(), src, store, store, titles); err != nil {
		t.Fatal(err)
	}
	steps, err := ingest.WithAnalysis(ingest.DefaultSteps())
	if err != nil {
		t.Fatal(err)
	}
	d := ingest.Deps{
		Ports: store.PlatformPorts(), Fetcher: src, Source: "fixtures/119",
		Log: slog.New(slog.DiscardHandler), Now: fixedNow,
	}
	for _, n := range []int{4313, 3062, 3427} {
		if rep, err := ingest.Pull(t.Context(), d, ingest.Options{Track: true}, steps, hr(n)); err != nil || rep.Failed() {
			t.Fatalf("pull %d: %v %+v", n, err, rep.Failures)
		}
	}
	return store
}

func deps(store *memory.Store, g data.Generator) generate.Deps {
	return generate.Deps{
		Ports: store.PlatformPorts(), Gen: g, Cfg: generate.DefaultConfig(),
		Now: fixedNow, Log: slog.New(slog.DiscardHandler),
	}
}

func TestSubmitCollectEndToEnd(t *testing.T) {
	t.Parallel()
	store := seeded(t)
	d := deps(store, fakegen.New(fakegen.Config{}))
	rep := checkFirstSubmit(t, d)
	// Nothing is stored until collect; a second submit adds nothing (the keys are pending).
	checkSubmitSkipsAll(t, d, "second", rep)
	checkCollect(t, d, rep)
	// After collect a re-run submits nothing: every key has a stored result.
	checkSubmitSkipsAll(t, d, "third", rep)
	checkPublished(t, store)
}

// checkFirstSubmit runs the first submit and checks that every planned task is submitted.
func checkFirstSubmit(t *testing.T, d generate.Deps) generate.SubmitReport {
	t.Helper()
	rep, err := generate.Submit(t.Context(), d, hr(4313), false)
	if err != nil || rep.Submitted == 0 || rep.Submitted != rep.Planned || rep.BatchID == "" {
		t.Fatalf("Submit = %+v, %v", rep, err)
	}
	return rep
}

// checkSubmitSkipsAll runs a submit and checks that it submits nothing and skips every planned task.
func checkSubmitSkipsAll(t *testing.T, d generate.Deps, label string, rep generate.SubmitReport) {
	t.Helper()
	again, err := generate.Submit(t.Context(), d, hr(4313), false)
	if err != nil || again.Submitted != 0 || again.Skipped != rep.Planned {
		t.Fatalf("%s Submit = %+v, %v", label, again, err)
	}
}

// checkCollect runs collect and checks that the one batch is ready for every submitted task.
func checkCollect(t *testing.T, d generate.Deps, rep generate.SubmitReport) {
	t.Helper()
	col, err := generate.Collect(t.Context(), d)
	if err != nil || col.Batches != 1 || col.ByStatus[data.GenReady] != rep.Submitted {
		t.Fatalf("Collect = %+v, %v (planned %d)", col, err, rep.Submitted)
	}
}

func checkPublished(t *testing.T, store *memory.Store) {
	t.Helper()
	vs, err := store.ListVersions(t.Context(), hr(4313))
	if err != nil || len(vs) != 4 {
		t.Fatalf("versions = %d, %v", len(vs), err)
	}
	changes := checkSummaries(t, store, vs)
	if changes < 2 {
		t.Errorf("change summaries = %d, want at least 2", changes)
	}
	checkExplanations(t, store, vs[0].Ref)
}

// checkSummaries checks the published summary of every version and returns
// how many versions have a change summary against the version before them.
func checkSummaries(t *testing.T, store *memory.Store, vs []data.VersionSummary) int {
	t.Helper()
	changes := 0
	for i, v := range vs {
		checkSummary(t, store, v.Ref)
		if i == 0 {
			continue
		}
		// A pair whose text is identical has no change summary.
		if hasChangeSummary(t, store, v.Ref, vs[i-1].Ref.Version) {
			changes++
		}
	}
	return changes
}

// checkSummary checks that the summary of one version is ready, unreviewed, and automatic platform content.
func checkSummary(t *testing.T, store *memory.Store, ref data.VersionRef) {
	t.Helper()
	out, err := store.PublicReadPorts().Generated.GetCurrent(t.Context(), data.TaskBillSummary, ref, "")
	if err != nil || out.Status != data.GenReady || out.ReviewState != data.ReviewUnreviewed ||
		out.Meta.Provenance != data.ProvenanceAuto || out.Meta.ContentClass != data.ContentPlatform {
		t.Fatalf("summary of %s = %+v, %v", ref, out, err)
	}
}

// hasChangeSummary reports whether a change summary is stored for ref against the prior version.
func hasChangeSummary(t *testing.T, store *memory.Store, ref data.VersionRef, prior string) bool {
	t.Helper()
	_, err := store.PublicReadPorts().Generated.GetCurrent(t.Context(), data.TaskVersionChanges, ref, prior)
	return err == nil
}

// checkExplanations checks that the stored explanations of a version are automatic and say what changes.
func checkExplanations(t *testing.T, store *memory.Store, ref data.VersionRef) {
	t.Helper()
	exps, err := store.GetExplanations(t.Context(), ref)
	if err != nil || len(exps) == 0 {
		t.Fatalf("explanations = %d, %v", len(exps), err)
	}
	for _, e := range exps {
		if e.Meta.Provenance != data.ProvenanceAuto || e.WhatChanges == "" {
			t.Errorf("explanation %v = %+v", e.Section, e)
		}
	}
}

// collectWith runs submit and collect on a seeded store, with the fakegen result
// for every planned request passed through mutate, to plant an output a model could produce.
func collectWith(t *testing.T, mutate func(r data.GenRequest, base data.GenResult) data.GenResult) (generate.CollectReport, *memory.Store) {
	t.Helper()
	store := seeded(t)
	base := fakegen.New(fakegen.Config{})
	d := deps(store, base)
	reqs := plannedRequests(t, store, d)
	over := map[string]data.GenResult{}
	for _, r := range reqs {
		res, err := base.Generate(t.Context(), r)
		if err != nil {
			t.Fatal(err)
		}
		over[r.ID] = mutate(r, res)
	}
	d.Gen = fakegen.New(fakegen.Config{Overrides: over})
	if _, err := generate.Submit(t.Context(), d, hr(4313), false); err != nil {
		t.Fatal(err)
	}
	col, err := generate.Collect(t.Context(), d)
	if err != nil {
		t.Fatal(err)
	}
	return col, store
}

func plannedRequests(t *testing.T, store *memory.Store, d generate.Deps) []data.GenRequest {
	t.Helper()
	vs, _ := store.ListVersions(t.Context(), hr(4313))
	var out []data.GenRequest
	var prev *data.OfficialVersion
	for _, s := range vs {
		v, err := store.GetVersion(t.Context(), s.Ref)
		if err != nil {
			t.Fatal(err)
		}
		tasks, _, err := generate.PlanVersion(t.Context(), d.Ports, v, prev, d.Cfg)
		if err != nil {
			t.Fatal(err)
		}
		for _, tk := range tasks {
			out = append(out, tk.Req)
		}
		prev = &v
	}
	return out
}

func TestInvalidAnchorAndInventedNumberAreNotPublished(t *testing.T) {
	t.Parallel()
	col, store := collectWith(t, func(r data.GenRequest, base data.GenResult) data.GenResult {
		if r.Task != data.TaskBillSummary {
			return base
		}
		// Every summary: one statement citing a line that does not exist, with an invented number.
		base.Text = `{"headline":"h","statements":[{"text":"The bill spends $987654321.","anchors":["s99-l99"],"provision_ids":[]}]}`
		return base
	})
	if col.ByStatus[data.GenUnverified] != 4 {
		t.Fatalf("by status = %v, want 4 unverified summaries", col.ByStatus)
	}
	vs, _ := store.ListVersions(t.Context(), hr(4313))
	if _, err := store.PublicReadPorts().Generated.GetCurrent(t.Context(), data.TaskBillSummary, vs[0].Ref, ""); err == nil {
		t.Fatal("an unverified summary is published")
	}
}

func TestRefusalTruncationAndGarbageAreRecordedNotPublished(t *testing.T) {
	t.Parallel()
	i := 0
	col, _ := collectWith(t, func(r data.GenRequest, base data.GenResult) data.GenResult {
		if r.Task != data.TaskBillSummary {
			return base
		}
		i++
		switch i {
		case 1:
			return data.GenResult{ID: base.ID, Outcome: "refusal", Stop: "refusal", Model: "m"}
		case 2:
			return data.GenResult{ID: base.ID, Outcome: "truncated", Stop: "max_tokens", Model: "m"}
		}
		base.Text = "not json"
		return base
	})
	if col.ByStatus[data.GenRefusal] != 1 || col.ByStatus[data.GenTruncated] != 1 || col.ByStatus[data.GenInvalid] != 2 {
		t.Fatalf("by status = %v", col.ByStatus)
	}
}

func TestPromptEditChangesKeys(t *testing.T) {
	t.Parallel()
	a, err := generate.LoadPrompt(data.TaskBillSummary)
	if err != nil || !strings.HasPrefix(a.Version, "v1-") {
		t.Fatalf("LoadPrompt = %+v, %v", a, err)
	}
	b, _ := generate.LoadPrompt(data.TaskVersionChanges)
	if a.Version == b.Version {
		t.Fatal("different tasks share a prompt version")
	}
}

func TestRegenerate(t *testing.T) {
	t.Parallel()
	store := seeded(t)
	d := deps(store, fakegen.New(fakegen.Config{}))
	status, err := generate.Regenerate(t.Context(), d, hr(4313), data.TaskBillSummary, "ih", "")
	if err != nil || status != data.GenReady {
		t.Fatalf("Regenerate = %q, %v", status, err)
	}
	// Outputs are write-once: the same prompt version and model conflict.
	if _, err := generate.Regenerate(t.Context(), d, hr(4313), data.TaskBillSummary, "ih", ""); !errors.Is(err, data.ErrConflict) {
		t.Fatalf("second Regenerate: err = %v, want ErrConflict", err)
	}
	if _, err := generate.Regenerate(t.Context(), d, hr(4313), data.TaskBillSummary, "zz", ""); !errors.Is(err, data.ErrNotFound) {
		t.Fatalf("unknown version: err = %v, want ErrNotFound", err)
	}
	d.Gen = nil
	if _, err := generate.Regenerate(t.Context(), d, hr(4313), data.TaskBillSummary, "ih", ""); !errors.Is(err, data.ErrUnavailable) {
		t.Fatalf("no generator: err = %v, want ErrUnavailable", err)
	}
}
