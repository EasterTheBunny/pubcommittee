package generate

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

// gradeCited is the cited text of one statement, from H.R. 3062 (rh), s2-l35.
const gradeCited = "s2-l35\n(1) Effective date Subsections (a) through (e), and the amendments made by such " +
	"subsections, shall take effect on the date that is 1 year after the date of enactment of this Act.\n"

func gradeReply(t *testing.T, label, reason, conflict string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"statements": []map[string]any{{"index": 0, "label": label, "reason": reason, "conflict": conflict}},
		"neutral":    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

type verdictCase struct {
	name, label, reason, conflict string
	ok                            bool
}

func verdictCases() []verdictCase {
	return []verdictCase{
		{"supported, no conflict", data.GradeSupported, "The line says so.", "", true},
		{"unsupported, no conflict", data.GradeUnsupported, "'New' is not stated.", "", true},
		{"supported quoting a conflict", data.GradeSupported, "Matches.", "1 year", false},
		{
			"contradicted with an exact quote", data.GradeContradicted,
			"The statement says 2 years; the line says 1 year.", "1 year after the date of enactment", true,
		},
		{
			"contradicted, quote differs in case, spacing and quote marks", data.GradeContradicted,
			"The statement says 2 years.", "Shall  take effect on the date\nthat is 1 YEAR", true,
		},
		{"contradicted without a quote", data.GradeContradicted, "The lines do not say only new ones.", "", false},
		{"contradicted, quote not cited", data.GradeContradicted, "Wrong deadline.", "not later than 120 days", false},
		// From the H.R. 3062 grade the operator sample exposed (09ad#5): the
		// reason itself finds the statement consistent.
		{
			"contradicted, reason concedes", data.GradeContradicted,
			"The cited line matches; this is actually consistent.", "1 year after the date of enactment", false,
		},
		{
			"contradicted, reason finds no conflict", data.GradeContradicted,
			"The dates are right, so the only conflict is none.", "Subsections (a) through (e)", false,
		},
	}
}

func TestParseGradeConsistency(t *testing.T) {
	t.Parallel()
	for _, c := range verdictCases() {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g, err := ParseGrade(gradeReply(t, c.label, c.reason, c.conflict), []string{gradeCited})
			if c.ok {
				if err != nil {
					t.Fatalf("ParseGrade: %v", err)
				}
				if g.Statements[0].Conflict != c.conflict {
					t.Errorf("Conflict = %q, want %q", g.Statements[0].Conflict, c.conflict)
				}
				return
			}
			if !errors.Is(err, ErrOutputInvalid) {
				t.Fatalf("ParseGrade error = %v, want ErrOutputInvalid", err)
			}
		})
	}
}

func TestParseGradeCountsVerdicts(t *testing.T) {
	t.Parallel()
	reply := gradeReply(t, data.GradeSupported, "ok", "")
	if _, err := ParseGrade(reply, []string{gradeCited, gradeCited}); !errors.Is(err, ErrOutputInvalid) {
		t.Fatalf("one verdict for two statements: error = %v, want ErrOutputInvalid", err)
	}
}

func TestGradePromptRequiresQuote(t *testing.T) {
	t.Parallel()
	pr, err := LoadPrompt(data.TaskGrade)
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties struct {
			Statements struct {
				Items struct {
					Required []string `json:"required"`
				} `json:"items"`
			} `json:"statements"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(pr.Schema, &schema); err != nil {
		t.Fatalf("grade schema: %v", err)
	}
	found := false
	for _, r := range schema.Properties.Statements.Items.Required {
		found = found || r == "conflict"
	}
	if !found {
		t.Error("grade schema does not require conflict")
	}
}

// TestGradeRequestSendsProvisionOnce checks that a provision cited by every
// statement appears once in the request, while each statement's cited text
// for the quote check still holds the provision text.
func TestGradeRequestSendsProvisionOnce(t *testing.T) {
	t.Parallel()
	const section = "Definitions: a long section of the cited law."
	gc := data.GenContext{
		Lines:      map[string]string{"s2-l1": "line one", "s2-l2": "line two"},
		Provisions: map[string]string{"usc/16/796": section},
	}
	o := data.GeneratedOutput{Key: data.GenKey{Model: "gen-model"}}
	for _, a := range []string{"s2-l1", "s2-l2", "s2-l1"} {
		o.Statements = append(o.Statements, data.Statement{Text: "x", Anchors: []string{a}, ProvisionIDs: []string{"usc/16/796"}})
	}
	req, err := GradeRequest(o, gc, "grader-model")
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(req.Input, section); n != 1 {
		t.Errorf("provision text appears %d times in the request, want 1", n)
	}
	for i, c := range GradeCited(o, gc) {
		if !strings.Contains(c, section) {
			t.Errorf("statement %d: cited text lacks the provision", i)
		}
	}
}

// oneVerdictGen answers every grade request with a single supported verdict
// at index 0, as the grader did for an output with four statements.
type oneVerdictGen struct{ ids []string }

func (g *oneVerdictGen) Submit(context.Context, []data.GenRequest) (string, error) { return "", nil }

func (g *oneVerdictGen) Collect(context.Context, string) ([]data.GenResult, bool, error) {
	return nil, true, nil
}

func (g *oneVerdictGen) Generate(_ context.Context, req data.GenRequest) (data.GenResult, error) {
	g.ids = append(g.ids, req.ID)
	text := `{"statements":[{"index":0,"label":"supported","reason":"Says so.","conflict":""}],"neutral":true}`
	return data.GenResult{ID: req.ID, Outcome: "ok", Text: text}, nil
}

// TestGradeOutputGradesEachOnCountMismatch checks the fallback: a reply with
// too few verdicts makes each statement be graded alone, and the joined grade
// has one verdict per statement, in order.
func TestGradeOutputGradesEachOnCountMismatch(t *testing.T) {
	t.Parallel()
	gc := data.GenContext{Lines: map[string]string{"s2-l1": "line one", "s2-l2": "line two"}}
	o := data.GeneratedOutput{Key: data.GenKey{
		Task: data.TaskChangeExplanation, Model: "gen-model", PromptVersion: "v1-test",
		Version: data.VersionRef{Bill: data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: 3062}, Version: "rh"},
	}}
	for _, a := range []string{"s2-l1", "s2-l2", "s2-l1"} {
		o.Statements = append(o.Statements, data.Statement{Text: "x", Anchors: []string{a}})
	}
	store := memory.New()
	d := Deps{
		Ports: store.PlatformPorts(), Now: func() time.Time { return time.Unix(0, 0) },
		Log: slog.New(slog.DiscardHandler),
	}
	gen := &oneVerdictGen{}
	g, err := GradeOutput(t.Context(), d, gen, "grader-model", o, gc)
	if err != nil {
		t.Fatalf("GradeOutput: %v", err)
	}
	if len(gen.ids) != 1+len(o.Statements) {
		t.Errorf("requests = %v, want one whole and one per statement", gen.ids)
	}
	if len(g.Statements) != len(o.Statements) {
		t.Fatalf("verdicts = %d, want %d", len(g.Statements), len(o.Statements))
	}
	for i, v := range g.Statements {
		if v.Index != i {
			t.Errorf("verdict %d has index %d", i, v.Index)
		}
	}
}

func TestParsePanel(t *testing.T) {
	t.Parallel()
	ok := `{"label":"contradicted","conflict":"1 year after the date of enactment",` +
		`"bill_says":"b","statement_says":"s","difference":"2 years against 1 year"}`
	v, err := ParsePanel(ok, gradeCited)
	if err != nil || v.Check != "" {
		t.Fatalf("valid contradicted: %+v %v", v, err)
	}
	bad := `{"label":"contradicted","conflict":"120 days","bill_says":"b","statement_says":"s","difference":"d"}`
	if v, err := ParsePanel(bad, gradeCited); err != nil || v.Check == "" {
		t.Errorf("unquoted conflict kept without a warning: %+v %v", v, err)
	}
	if _, err := ParsePanel(`{"label":"maybe","conflict":"","bill_says":"","statement_says":"","difference":""}`,
		gradeCited); !errors.Is(err, ErrOutputInvalid) {
		t.Errorf("unknown label accepted: %v", err)
	}
}
