package accuracy_test

import (
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/accuracy"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

func graded(e accuracy.SampleEntry, labels ...string) accuracy.SampleEntry {
	g := data.Grade{}
	for i, l := range labels {
		g.Statements = append(g.Statements, data.StatementGrade{Index: i, Label: l})
	}
	e.Grade = &g
	return e
}

func TestBuildHardSampleStrata(t *testing.T) {
	t.Parallel()
	ready := graded(sampleEntry("change-explanation", "a", data.GenReady, stmts("change-explanation", "a", 2)),
		data.GradeSupported, data.GradeContradicted)
	bad := sampleEntry("change-explanation", "b", data.GenUnverified, []data.Statement{
		{Text: "placeholder", Anchors: []string{"s4-l12"}},
		{Text: "The bill raises the figure by a stated amount", Anchors: []string{"s4-l12"}},
	})
	bad.Output.Findings = []string{"statement 1: fact not in cited lines: number 7"}
	entries := []accuracy.SampleEntry{ready, bad}
	mix := accuracy.HardMix{Contradicted: 5, Rejected: 5, Control: 5}

	items := accuracy.BuildHardSample(entries, mix, nil)
	got := map[string]int{}
	for _, it := range items {
		got[it.Stratum]++
		if it.Stratum == accuracy.StratumRejected && len(it.ValidatorFindings) != 1 {
			t.Errorf("rejected item lacks findings: %+v", it)
		}
	}
	want := map[string]int{accuracy.StratumContradicted: 1, accuracy.StratumControl: 1, accuracy.StratumRejected: 1}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("stratum %s = %d, want %d (all: %v)", k, got[k], v, got)
		}
	}

	excl := map[string]bool{items[0].ID: true}
	if n := len(accuracy.BuildHardSample(entries, mix, excl)); n != 2 {
		t.Errorf("after exclusion got %d items, want 2", n)
	}
}
