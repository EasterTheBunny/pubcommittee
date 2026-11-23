package accuracy_test

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/accuracy"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

// sampleEntry is a synthetic ready-or-not output of one task with the given
// statements; target keeps keys of one task apart.
func sampleEntry(task, target, status string, stmts []data.Statement) accuracy.SampleEntry {
	return accuracy.SampleEntry{
		Output: data.GeneratedOutput{
			Key: data.GenKey{
				Task: task,
				Version: data.VersionRef{
					Bill:    data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: 1},
					Version: "ih",
				},
				Target:        target,
				PromptVersion: "p1",
				Model:         "m1",
			},
			Status:     status,
			Statements: stmts,
		},
		Ctx: data.GenContext{
			Lines: map[string]string{
				"s4-l12":      "line 12",
				"s4-l13":      "line 13",
				"s4-l14":      "line 14",
				"prev:s4-l13": "old line 13",
			},
			Provisions: map[string]string{"usc/26/1": "provision text"},
		},
	}
}

// stmts returns n statements named "<task> <target> <index>" citing s4-l12.
func stmts(task, target string, n int) []data.Statement {
	out := make([]data.Statement, 0, n)
	for i := range n {
		out = append(out, data.Statement{
			Text:    task + " " + target + " " + strconv.Itoa(i),
			Anchors: []string{"s4-l12"},
		})
	}
	return out
}

func TestBuildSampleCountAndDeterminism(t *testing.T) {
	t.Parallel()
	entries := []accuracy.SampleEntry{
		sampleEntry("bill-summary", "a", data.GenReady, stmts("bill-summary", "a", 3)),
		sampleEntry("bill-summary", "b", data.GenReady, stmts("bill-summary", "b", 3)),
		sampleEntry("version-changes", "c", data.GenReady, stmts("version-changes", "c", 4)),
	}
	reversed := []accuracy.SampleEntry{entries[2], entries[1], entries[0]}

	got := accuracy.BuildSample(entries, 5)
	if len(got) != 5 {
		t.Fatalf("BuildSample n=5 returned %d items", len(got))
	}
	again := accuracy.BuildSample(reversed, 5)
	if !reflect.DeepEqual(got, again) {
		t.Errorf("BuildSample depends on input order:\n%+v\n%+v", got, again)
	}
	if !reflect.DeepEqual(got, accuracy.BuildSample(entries, 5)) {
		t.Error("BuildSample is not stable across calls")
	}
	if all := accuracy.BuildSample(entries, 100); len(all) != 10 {
		t.Errorf("BuildSample with too few statements returned %d, want 10", len(all))
	}
}

func TestBuildSampleSkipsNotReady(t *testing.T) {
	t.Parallel()
	entries := []accuracy.SampleEntry{
		sampleEntry("bill-summary", "ready", data.GenReady, stmts("bill-summary", "ready", 1)),
		sampleEntry("bill-summary", "bad", data.GenUnverified, stmts("bill-summary", "bad", 2)),
		sampleEntry("bill-summary", "gone", data.GenWithheld, stmts("bill-summary", "gone", 2)),
	}
	got := accuracy.BuildSample(entries, 10)
	if len(got) != 1 {
		t.Fatalf("got %d items, want 1 ready statement", len(got))
	}
	if !strings.Contains(got[0].Key, "|ready|") {
		t.Errorf("picked key %q, want the ready entry", got[0].Key)
	}
}

func TestBuildSampleRoundRobin(t *testing.T) {
	t.Parallel()
	entries := []accuracy.SampleEntry{
		sampleEntry("version-changes", "x", data.GenReady, stmts("version-changes", "x", 3)),
		sampleEntry("bill-summary", "y", data.GenReady, stmts("bill-summary", "y", 3)),
	}
	got := accuracy.BuildSample(entries, 4)
	tasks := make([]string, 0, len(got))
	for _, it := range got {
		tasks = append(tasks, it.Task)
	}
	want := []string{"bill-summary", "version-changes", "bill-summary", "version-changes"}
	if !reflect.DeepEqual(tasks, want) {
		t.Errorf("task order = %v, want %v", tasks, want)
	}
	if got[0].Statement != "bill-summary y 0" || got[2].Statement != "bill-summary y 1" {
		t.Errorf("statements not in entry order: %q, %q", got[0].Statement, got[2].Statement)
	}
}

func TestBuildSampleCitedRowsAndGraderLabel(t *testing.T) {
	t.Parallel()
	st := []data.Statement{{
		Text:         "The bill adds a fee.",
		Anchors:      []string{"s4-l12-14", "prev:s4-l13"},
		ProvisionIDs: []string{"usc/26/1"},
	}}
	e := sampleEntry("bill-summary", "z", data.GenReady, st)
	e.Grade = &data.Grade{Statements: []data.StatementGrade{{Index: 0, Label: data.GradeContradicted}}}

	got := accuracy.BuildSample([]accuracy.SampleEntry{e}, 1)
	if len(got) != 1 {
		t.Fatalf("got %d items, want 1", len(got))
	}
	it := got[0]
	wantCited := []string{
		"s4-l12: line 12",
		"s4-l13: line 13",
		"s4-l14: line 14",
		"prev:s4-l13: old line 13",
		"usc/26/1: provision text",
	}
	if !reflect.DeepEqual(it.Cited, wantCited) {
		t.Errorf("Cited = %q, want %q", it.Cited, wantCited)
	}
	if it.GraderLabel != data.GradeContradicted {
		t.Errorf("GraderLabel = %q, want contradicted", it.GraderLabel)
	}
	if it.OperatorLabel != "" {
		t.Errorf("OperatorLabel = %q, want empty", it.OperatorLabel)
	}
	if want := e.Output.Key.ID() + "#0"; it.ID != want {
		t.Errorf("ID = %q, want %q", it.ID, want)
	}
}

func TestSampleMarkdown(t *testing.T) {
	t.Parallel()
	items := []accuracy.SampleItem{{
		ID:          "abc#0",
		Task:        "bill-summary",
		Statement:   "The bill adds a fee.",
		Cited:       []string{"s4-l12: line 12 says fee"},
		GraderLabel: data.GradeContradicted,
	}}
	md := accuracy.SampleMarkdown(items)
	for _, want := range []string{
		"The bill adds a fee.",
		"s4-l12: line 12 says fee",
		"abc#0",
		"Your label (supported / unsupported / contradicted): ______",
		"operator_label",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("SampleMarkdown missing %q", want)
		}
	}
	if strings.Contains(md, "Grader label") {
		t.Error("SampleMarkdown shows the grader label")
	}
}

func TestAgreement(t *testing.T) {
	t.Parallel()
	const (
		sup = data.GradeSupported
		uns = data.GradeUnsupported
		con = data.GradeContradicted
	)
	tests := []struct {
		name  string
		items []accuracy.SampleItem
		want  accuracy.AgreementResult
	}{
		{name: "none", want: accuracy.AgreementResult{}},
		{
			name: "mixed",
			items: []accuracy.SampleItem{
				{OperatorLabel: sup, GraderLabel: sup},
				{OperatorLabel: con, GraderLabel: uns},
				{OperatorLabel: sup, GraderLabel: con},
				{OperatorLabel: con, GraderLabel: con},
				{OperatorLabel: "", GraderLabel: sup},
				{OperatorLabel: sup, GraderLabel: ""},
				{OperatorLabel: uns, GraderLabel: uns},
			},
			want: accuracy.AgreementResult{Compared: 5, Agreed: 3, Missed: 1, FalseAlarm: 1, Share: 0.6},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := accuracy.Agreement(tt.items); got != tt.want {
				t.Errorf("Agreement = %+v, want %+v", got, tt.want)
			}
		})
	}
}
