package ingest_test

import (
	"strings"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/ingest"
	"github.com/easterthebunny/pubcommittee/internal/lawingest"
)

func analysisSteps(t *testing.T) ingest.Pipeline {
	t.Helper()
	p, err := ingest.WithAnalysis(ingest.DefaultSteps())
	if err != nil {
		t.Fatalf("WithAnalysis: %v", err)
	}
	return p
}

func TestAnalysisStepsStoreCitationsAndComparisons(t *testing.T) {
	t.Parallel()
	store := memory.New()
	src := fixtureSource(t, fixedTime())
	d := deps(store, src, fixedTime())
	opts := ingest.Options{Track: true}

	// No release point stored yet: citations are skipped with a note, comparisons still run.
	rep, err := ingest.Pull(t.Context(), d, opts, analysisSteps(t), bill(4313))
	if err != nil || rep.Failed() {
		t.Fatalf("first pull: %v %+v", err, rep.Failures)
	}
	if rep.CitationSets != 0 || len(rep.Notes) != 1 || !strings.Contains(rep.Notes[0], "no existing-law release point") {
		t.Errorf("first pull: sets=%d notes=%v", rep.CitationSets, rep.Notes)
	}
	if rep.Comparisons != 5 {
		t.Errorf("comparisons = %d, want 5 (3 consecutive pairs plus 2 against the latest)", rep.Comparisons)
	}

	titles, err := src.LawTitles(t.Context(), data.ReleasePoint{ID: "119-142"})
	if err != nil {
		t.Fatalf("LawTitles: %v", err)
	}
	if _, err := lawingest.Step(t.Context(), src, store, store, titles); err != nil {
		t.Fatalf("law step: %v", err)
	}

	// A repeat pull catches up: every stored version gets a set; running again changes nothing.
	for range 2 {
		checkCaughtUp(t, d, opts)
	}
}

func checkCaughtUp(t *testing.T, d ingest.Deps, opts ingest.Options) {
	t.Helper()
	rep, err := ingest.Pull(t.Context(), d, opts, analysisSteps(t), bill(4313))
	if err != nil || rep.Failed() {
		t.Fatalf("pull: %v %+v", err, rep.Failures)
	}
	if rep.CitationSets != 4 || rep.Comparisons != 5 || len(rep.Notes) != 0 {
		t.Errorf("pull: sets=%d comparisons=%d notes=%v", rep.CitationSets, rep.Comparisons, rep.Notes)
	}
}

func TestAnalysisSkippedForUntrackedAndDryRun(t *testing.T) {
	t.Parallel()
	store := memory.New()
	d := deps(store, fixtureSource(t, fixedTime()), fixedTime())
	rep, err := ingest.Pull(t.Context(), d, ingest.Options{DryRun: true, Track: true}, analysisSteps(t), bill(4313))
	if err != nil || rep.CitationSets != 0 || rep.Comparisons != 0 || len(rep.Notes) != 0 {
		t.Errorf("dry run: %v %+v", err, rep)
	}
}

func TestAnalyzeRunsOverStoredData(t *testing.T) {
	t.Parallel()
	store := memory.New()
	d := deps(store, fixtureSource(t, fixedTime()), fixedTime())
	// Store the versions with no analysis steps, then analyze without fetching.
	if _, err := ingest.Pull(t.Context(), d, ingest.Options{Track: true}, ingest.DefaultSteps(), bill(4313)); err != nil {
		t.Fatal(err)
	}
	rep, err := ingest.Analyze(t.Context(), d, ingest.Options{}, bill(4313))
	if err != nil || rep.Failed() || rep.Comparisons != 5 {
		t.Fatalf("Analyze = %+v, %v", rep, err)
	}
}
