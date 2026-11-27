package generate_test

import (
	"errors"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/adapters/fakegen"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/generate"
)

// kindsByVersion counts the planned tasks of a plan by version code and kind.
func kindsByVersion(tasks []generate.Task) map[string]map[string]int {
	out := map[string]map[string]int{}
	for _, tk := range tasks {
		if out[tk.Key.Version.Version] == nil {
			out[tk.Key.Version.Version] = map[string]int{}
		}
		out[tk.Key.Version.Version][tk.Key.Task]++
	}
	return out
}

func explanationCount(kinds map[string]int) int {
	return kinds[data.TaskChangeExplanation] + kinds[data.TaskBillExplanation]
}

// planDry runs a dry-run submit of H.R. 4313 and returns its report.
func planDry(t *testing.T, d generate.Deps) generate.SubmitReport {
	t.Helper()
	rep, err := generate.Submit(t.Context(), d, hr(4313), true)
	if err != nil {
		t.Fatalf("Submit dry: %v", err)
	}
	return rep
}

func leftOutOf(rep generate.SubmitReport) map[string]int {
	out := map[string]int{}
	for _, l := range rep.LeftOut {
		out[l.Version] = l.Tasks
	}
	return out
}

// A bill pulled with several versions at once: explanations of the latest only, the left-out counts reported.
func TestSubmitExplainsOnlyTheLatestVersion(t *testing.T) {
	t.Parallel()
	store := seeded(t)
	d := deps(store, fakegen.New(fakegen.Config{}))
	all, _, err := generate.PlanAll(t.Context(), d, hr(4313))
	if err != nil {
		t.Fatal(err)
	}
	kinds := kindsByVersion(all)
	if explanationCount(kinds["ih"]) == 0 || explanationCount(kinds["rh"]) == 0 || explanationCount(kinds["eh"]) == 0 {
		t.Fatalf("PlanAll must stay complete (revalidate and regenerate use it): %v", kinds)
	}
	rep := planDry(t, d)
	left := leftOutOf(rep)
	if left["ih"] != explanationCount(kinds["ih"]) || left["rh"] != explanationCount(kinds["rh"]) || left["eh"] != 0 {
		t.Errorf("left out = %v, want ih %d, rh %d", left, explanationCount(kinds["ih"]), explanationCount(kinds["rh"]))
	}
	want := len(all) - left["ih"] - left["rh"]
	if rep.Planned != want {
		t.Errorf("planned = %d, want %d", rep.Planned, want)
	}
	checkEveryVersionSummarized(t, kinds)
}

// checkEveryVersionSummarized checks a bill summary for every version and a change summary for rh and eh.
func checkEveryVersionSummarized(t *testing.T, kinds map[string]map[string]int) {
	t.Helper()
	for _, v := range []string{"ih", "rh", "eh"} {
		if kinds[v][data.TaskBillSummary] != 1 {
			t.Errorf("%s: no bill summary planned: %v", v, kinds[v])
		}
	}
	if kinds["ih"][data.TaskVersionChanges] != 0 || kinds["rh"][data.TaskVersionChanges] != 1 {
		t.Errorf("version change summaries: %v", kinds)
	}
}

// A new latest version arrives after the old one was generated: the old keeps its outputs and gets nothing new.
func TestSupersededVersionKeepsStoredExplanations(t *testing.T) {
	t.Parallel()
	store := seeded(t)
	d := deps(store, fakegen.New(fakegen.Config{}))
	vs, err := store.ListVersions(t.Context(), hr(4313))
	if err != nil || len(vs) != 3 {
		t.Fatalf("versions = %d, %v", len(vs), err)
	}
	// Generate with rh explained, as if rh was the latest when it was pulled.
	d.ExplainVersion = "rh"
	if _, err := generate.Submit(t.Context(), d, hr(4313), false); err != nil {
		t.Fatal(err)
	}
	if _, err := generate.Collect(t.Context(), d); err != nil {
		t.Fatal(err)
	}
	exps, err := store.GetExplanations(t.Context(), vs[1].Ref)
	if err != nil || len(exps) == 0 {
		t.Fatalf("rh explanations = %d, %v", len(exps), err)
	}
	// Plan as the next run does: rh stays served, and nothing new is planned or counted for it.
	d.ExplainVersion = ""
	rep := planDry(t, d)
	if got := leftOutOf(rep)["rh"]; got != 0 {
		t.Errorf("rh left out = %d, want 0 (already stored)", got)
	}
	again, err := store.GetExplanations(t.Context(), vs[1].Ref)
	if err != nil || len(again) != len(exps) {
		t.Errorf("rh explanations after = %d, %v, want %d", len(again), err, len(exps))
	}
}

// A single-version bill is its own latest version: nothing is left out.
func TestSingleVersionBillIsExplained(t *testing.T) {
	t.Parallel()
	store := seeded(t)
	d := deps(store, fakegen.New(fakegen.Config{}))
	for _, n := range []int{3062, 3427} {
		vs, err := store.ListVersions(t.Context(), hr(n))
		if err != nil || len(vs) != 1 {
			continue
		}
		rep, err := generate.Submit(t.Context(), d, hr(n), true)
		if err != nil || len(rep.LeftOut) != 0 {
			t.Errorf("hr %d: report = %+v, %v", n, rep, err)
		}
	}
}

func TestExplainVersion(t *testing.T) {
	t.Parallel()
	store := seeded(t)
	d := deps(store, fakegen.New(fakegen.Config{}))
	base := planDry(t, d)

	d.ExplainVersion = "eh"
	if rep := planDry(t, d); rep.Planned != base.Planned || len(rep.LeftOut) != len(base.LeftOut) {
		t.Errorf("--explain-version of the latest adds tasks: %+v, base %+v", rep, base)
	}

	d.ExplainVersion = "ih"
	rep := planDry(t, d)
	left := leftOutOf(rep)
	if left["ih"] != 0 || left["rh"] != leftOutOf(base)["rh"] || rep.Planned != base.Planned+leftOutOf(base)["ih"] {
		t.Errorf("--explain-version ih: report %+v, base %+v", rep, base)
	}

	d.ExplainVersion = "zz"
	if _, err := generate.Submit(t.Context(), d, hr(4313), true); !errors.Is(err, generate.ErrUnknownVersion) {
		t.Errorf("--explain-version zz error = %v, want ErrUnknownVersion", err)
	}
}
