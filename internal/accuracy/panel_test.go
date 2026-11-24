package accuracy

import (
	"context"
	"strings"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// labelGen answers every panel request with the label of its model.
type labelGen struct{ labels map[string]string }

func (g labelGen) Submit(context.Context, []data.GenRequest) (string, error) { return "", nil }

func (g labelGen) Collect(context.Context, string) ([]data.GenResult, bool, error) {
	return nil, true, nil
}

func (g labelGen) Generate(_ context.Context, req data.GenRequest) (data.GenResult, error) {
	text := `{"label":"` + g.labels[req.Model] + `","conflict":"","bill_says":"b","statement_says":"s","difference":""}`
	return data.GenResult{ID: req.ID, Outcome: "ok", Text: text}, nil
}

func panelEntry() SampleEntry {
	o := data.GeneratedOutput{
		Key: data.GenKey{Task: data.TaskChangeExplanation, Model: "gen", PromptVersion: "v1-test", Version: data.VersionRef{
			Bill: data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: 3062}, Version: "rh",
		}},
		Statements: []data.Statement{{Text: "x", Anchors: []string{"s2-l1"}}},
	}
	return SampleEntry{Output: o, Ctx: data.GenContext{Lines: map[string]string{"s2-l1": "line"}}}
}

func TestRunPanelGroups(t *testing.T) {
	t.Parallel()
	e := panelEntry()
	id := e.Output.Key.ID() + "#0"
	cases := []struct {
		grader, a, b, want string
	}{
		{data.GradeSupported, data.GradeSupported, data.GradeSupported, PanelSupported},
		{data.GradeUnsupported, data.GradeUnsupported, data.GradeContradicted, PanelMisstated},
		{data.GradeSupported, data.GradeUnsupported, data.GradeSupported, PanelSplit},
		{"", data.GradeUnsupported, data.GradeUnsupported, PanelMisstated},
	}
	for _, c := range cases {
		gen := labelGen{labels: map[string]string{"m-a": c.a, "m-b": c.b}}
		sample := []SampleItem{{ID: id, GraderLabel: c.grader}, {ID: "missing#0"}}
		items := RunPanel(t.Context(), gen, sample, []SampleEntry{e}, []string{"m-a", "m-b"})
		if items[0].Group != c.want {
			t.Errorf("%v: group %q, want %q", c, items[0].Group, c.want)
		}
		if items[1].Group != PanelNoVerdict || items[1].Opinions[0].Error == "" {
			t.Errorf("missing statement: %+v", items[1])
		}
	}
}

func TestPanelMarkdownShowsLabels(t *testing.T) {
	t.Parallel()
	e := panelEntry()
	gen := labelGen{labels: map[string]string{"m-a": data.GradeUnsupported}}
	items := RunPanel(t.Context(), gen, []SampleItem{{ID: e.Output.Key.ID() + "#0", GraderLabel: "supported"}},
		[]SampleEntry{e}, []string{"m-a"})
	md := PanelMarkdown(items, "grader", "gen")
	for _, want := range []string{"## Split", "**grader** (stored grade): supported", "**m-a**: unsupported", "Bill says: b"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown lacks %q", want)
		}
	}
}
