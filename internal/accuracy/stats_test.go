package accuracy_test

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/accuracy"
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

// firstKey returns the first planned key of bill 4313.
func firstKey(t *testing.T, d generate.Deps) data.GenKey {
	t.Helper()
	tasks, _, err := generate.PlanAll(t.Context(), d, hr(4313))
	if err != nil || len(tasks) == 0 {
		t.Fatalf("PlanAll = %d tasks, %v", len(tasks), err)
	}
	return tasks[0].Key
}

func TestCollectReadyBillSummary(t *testing.T) {
	t.Parallel()
	store := seeded(t)
	d := deps(store, fakegen.New(fakegen.Config{}))
	rep, err := generate.Submit(t.Context(), d, hr(4313), false)
	if err != nil || rep.Submitted == 0 {
		t.Fatalf("Submit = %+v, %v", rep, err)
	}
	if _, err := generate.Collect(t.Context(), d); err != nil {
		t.Fatalf("Collect: %v", err)
	}
	stats, err := accuracy.Collect(t.Context(), d, []data.BillRef{hr(4313)})
	if err != nil {
		t.Fatalf("accuracy.Collect: %v", err)
	}
	s := stats[data.TaskBillSummary]
	if s == nil {
		t.Fatalf("no %s stats in %d tasks", data.TaskBillSummary, len(stats))
	}
	if s.Outputs != 4 || s.ByStatus[data.GenReady] != s.Outputs {
		t.Errorf("Outputs = %d, ByStatus = %v; want 4 all ready", s.Outputs, s.ByStatus)
	}
	if len(s.Failures) != 0 {
		t.Errorf("Failures = %v, want none", s.Failures)
	}
	if s.Graded != 0 {
		t.Errorf("Graded = %d, want 0", s.Graded)
	}
}

func TestCollectCountsFindings(t *testing.T) {
	t.Parallel()
	store := seeded(t)
	d := deps(store, fakegen.New(fakegen.Config{}))
	key := firstKey(t, d)
	out := data.GeneratedOutput{
		Key:      key,
		Status:   data.GenUnverified,
		Findings: []string{"statement 1: fact not in cited lines: number 7"},
	}
	if err := d.Ports.GeneratedStore.PutGenerated(t.Context(), out); err != nil {
		t.Fatalf("PutGenerated: %v", err)
	}
	stats, err := accuracy.Collect(t.Context(), d, []data.BillRef{hr(4313)})
	if err != nil {
		t.Fatalf("accuracy.Collect: %v", err)
	}
	s := stats[key.Task]
	if s == nil || s.Outputs != 1 || s.Failures[accuracy.CheckFidelity] != 1 {
		t.Errorf("stats[%s] = %+v, want one output with one %s failure", key.Task, s, accuracy.CheckFidelity)
	}
}

func TestCollectCountsGradeLabels(t *testing.T) {
	t.Parallel()
	store := seeded(t)
	d := deps(store, fakegen.New(fakegen.Config{}))
	key := firstKey(t, d)
	out := data.GeneratedOutput{
		Key:        key,
		Status:     data.GenReady,
		Statements: []data.Statement{{Text: "first"}, {Text: "second"}},
	}
	if err := d.Ports.GeneratedStore.PutGenerated(t.Context(), out); err != nil {
		t.Fatalf("PutGenerated: %v", err)
	}
	g := data.Grade{
		Key:           key,
		GraderModel:   fakegen.Model,
		PromptVersion: "test",
		Statements: []data.StatementGrade{
			{Index: 0, Label: data.GradeSupported},
			{Index: 1, Label: data.GradeContradicted},
		},
		Neutral: false,
		At:      fixedNow(),
	}
	if err := d.Ports.GeneratedStore.PutGrade(t.Context(), g); err != nil {
		t.Fatalf("PutGrade: %v", err)
	}
	stats, err := accuracy.Collect(t.Context(), d, []data.BillRef{hr(4313)})
	if err != nil {
		t.Fatalf("accuracy.Collect: %v", err)
	}
	s := stats[key.Task]
	if s == nil {
		t.Fatalf("no stats for %s", key.Task)
	}
	if s.Statements != 2 || s.Graded != 2 || s.NotNeutral != 1 {
		t.Errorf("Statements, Graded, NotNeutral = %d, %d, %d; want 2, 2, 1", s.Statements, s.Graded, s.NotNeutral)
	}
	if s.Labels[data.GradeSupported] != 1 || s.Labels[data.GradeContradicted] != 1 {
		t.Errorf("Labels = %v, want one supported and one contradicted", s.Labels)
	}
}

func TestRate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		n, total int
		want     float64
	}{
		{"half", 1, 2, 0.5},
		{"none", 0, 5, 0},
		{"all", 4, 4, 1},
		{"empty total", 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := accuracy.Rate(tt.n, tt.total); got != tt.want {
				t.Errorf("Rate(%d, %d) = %v, want %v", tt.n, tt.total, got, tt.want)
			}
		})
	}
}
