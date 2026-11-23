package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// gradeDoc is the model's answer to a grade request.
type gradeDoc struct {
	Statements []data.StatementGrade `json:"statements"`
	Neutral    bool                  `json:"neutral"`
}

type gradeStatement struct {
	Index int    `json:"index"`
	Text  string `json:"text"`
	Cited string `json:"cited"`
}

// GradeRequest builds the request that asks a grader model to label each
// statement of an output supported, unsupported or contradicted by exactly
// the lines it cites. graderModel must differ from the generating model.
func GradeRequest(o data.GeneratedOutput, gc data.GenContext, graderModel string) (data.GenRequest, error) {
	if graderModel == "" || graderModel == o.Key.Model {
		return data.GenRequest{}, fmt.Errorf("grader model %q equals the generating model: %w", graderModel, data.ErrInvalid)
	}
	pr, err := LoadPrompt(data.TaskGrade)
	if err != nil {
		return data.GenRequest{}, err
	}
	doc := struct {
		Task       string           `json:"task"`
		Statements []gradeStatement `json:"statements"`
	}{Task: data.TaskGrade}
	for i, s := range o.Statements {
		cited, _ := citedText(s, gc)
		doc.Statements = append(doc.Statements, gradeStatement{Index: i, Text: s.Text, Cited: cited})
	}
	body, err := json.Marshal(doc)
	if err != nil {
		return data.GenRequest{}, fmt.Errorf("grade request: %w", err)
	}
	return data.GenRequest{
		ID: "grade-" + o.Key.ID(), Task: data.TaskGrade, Instructions: pr.Instructions,
		Input: "<request>" + string(body) + "</request>", Schema: pr.Schema, Effort: "medium",
		PromptVersion: pr.Version, Model: graderModel,
	}, nil
}

// ParseGrade reads the grader's answer for an output of n statements: every
// statement needs exactly one verdict with a known label.
func ParseGrade(text string, n int) (data.Grade, error) {
	var doc gradeDoc
	dec := json.NewDecoder(strings.NewReader(text))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		return data.Grade{}, fmt.Errorf("%w: %w", ErrOutputInvalid, err)
	}
	if len(doc.Statements) != n {
		return data.Grade{}, fmt.Errorf("%w: %d verdicts for %d statements", ErrOutputInvalid, len(doc.Statements), n)
	}
	seen := map[int]bool{}
	for _, g := range doc.Statements {
		switch g.Label {
		case data.GradeSupported, data.GradeUnsupported, data.GradeContradicted:
		default:
			return data.Grade{}, fmt.Errorf("%w: label %q", ErrOutputInvalid, g.Label)
		}
		if g.Index < 0 || g.Index >= n || seen[g.Index] {
			return data.Grade{}, fmt.Errorf("%w: verdict index %d", ErrOutputInvalid, g.Index)
		}
		seen[g.Index] = true
	}
	return data.Grade{Statements: doc.Statements, Neutral: doc.Neutral}, nil
}

// GradeOutput runs the grader on one stored output and stores the grade
// beside it (generated/{task}/{bill}/v/{n}/{key}.grade.json) with the grader
// model and prompt version. It is a live run when gen is a model: callers
// only reach it from the accuracy report, never from CI.
func GradeOutput(
	ctx context.Context, d Deps, gen data.Generator, graderModel string, o data.GeneratedOutput, gc data.GenContext,
) (data.Grade, error) {
	req, err := GradeRequest(o, gc, graderModel)
	if err != nil {
		return data.Grade{}, err
	}
	res, err := gen.Generate(ctx, req)
	if err != nil {
		return data.Grade{}, fmt.Errorf("grade %s: %w", o.Key, err)
	}
	if res.Outcome != "ok" {
		return data.Grade{}, fmt.Errorf("grade %s: outcome %s: %w", o.Key, res.Outcome, ErrOutputInvalid)
	}
	g, err := ParseGrade(res.Text, len(o.Statements))
	if err != nil {
		return data.Grade{}, fmt.Errorf("grade %s: %w", o.Key, err)
	}
	g.Key, g.GraderModel, g.PromptVersion, g.At = o.Key, graderModel, req.PromptVersion, d.Now().UTC()
	if err := d.Ports.GeneratedStore.PutGrade(ctx, g); err != nil {
		return data.Grade{}, fmt.Errorf("store grade %s: %w", o.Key, err)
	}
	return g, nil
}
