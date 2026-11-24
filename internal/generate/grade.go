package generate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// errVerdictCount marks a grade whose reply has a different number of
// verdicts than the request has statements.
var errVerdictCount = errors.New("verdict count")

// gradeDoc is the model's answer to a grade request.
type gradeDoc struct {
	Statements []data.StatementGrade `json:"statements"`
	Neutral    bool                  `json:"neutral"`
}

type gradeStatement struct {
	Index        int      `json:"index"`
	Text         string   `json:"text"`
	Cited        string   `json:"cited"`
	ProvisionIDs []string `json:"provision_ids,omitempty"`
}

// gradeProvision is one cited provision of a grade request. It is sent once,
// however many statements cite it: a long section cited by every statement
// would otherwise repeat in each, and the grader then answers fewer verdicts
// than there are statements.
type gradeProvision struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// GradeRequest builds the request that asks a grader model to label each
// statement of an output supported, unsupported or contradicted by exactly
// the lines it cites. graderModel must differ from the generating model.
func GradeRequest(o data.GeneratedOutput, gc data.GenContext, graderModel string) (data.GenRequest, error) {
	if graderModel == "" || graderModel == o.Key.Model {
		return data.GenRequest{}, fmt.Errorf("grader model %q equals the generating model: %w", graderModel, data.ErrInvalid)
	}
	return statementsRequest(data.TaskGrade, "grade-", o, gc, graderModel)
}

// statementsRequest builds a grade-shaped request of the given task for the
// statements of o: each statement with its cited lines, and each cited
// provision once.
func statementsRequest(
	task, idPrefix string, o data.GeneratedOutput, gc data.GenContext, model string,
) (data.GenRequest, error) {
	pr, err := LoadPrompt(task)
	if err != nil {
		return data.GenRequest{}, err
	}
	doc := struct {
		Task       string           `json:"task"`
		Statements []gradeStatement `json:"statements"`
		Provisions []gradeProvision `json:"provisions,omitempty"`
	}{Task: task}
	sent := map[string]bool{}
	for i, s := range o.Statements {
		lines := s
		lines.ProvisionIDs = nil
		cited, _ := citedText(lines, gc)
		doc.Statements = append(doc.Statements, gradeStatement{
			Index: i, Text: s.Text, Cited: cited, ProvisionIDs: s.ProvisionIDs,
		})
		for _, id := range s.ProvisionIDs {
			if sent[id] {
				continue
			}
			sent[id] = true
			text, _ := lookupProvision(gc, id)
			doc.Provisions = append(doc.Provisions, gradeProvision{ID: id, Text: text})
		}
	}
	body, err := json.Marshal(doc)
	if err != nil {
		return data.GenRequest{}, fmt.Errorf("%s request: %w", task, err)
	}
	effort := "medium"
	if task == TaskPanel {
		effort = "high"
	}
	return data.GenRequest{
		ID: idPrefix + o.Key.ID(), Task: task, Instructions: pr.Instructions,
		Input: "<request>" + string(body) + "</request>", Schema: pr.Schema, Effort: effort,
		PromptVersion: pr.Version, Model: model,
	}, nil
}

// PanelVerdict is one panel model's answer on one statement. Check is set
// when a contradicted verdict fails the consistency check of ParseGrade (its
// quote is not in the cited text, or its reason concedes a match): the
// verdict is kept for the operator, with that warning.
type PanelVerdict struct {
	Label         string `json:"label"`
	Conflict      string `json:"conflict"`
	BillSays      string `json:"bill_says"`
	StatementSays string `json:"statement_says"`
	Difference    string `json:"difference"`
	Check         string `json:"check,omitempty"`
}

// PanelRequest builds the panel request for statement i of o, for a model
// that may be the generating one: the panel is a second opinion for the
// operator, not a grade.
func PanelRequest(o data.GeneratedOutput, gc data.GenContext, i int, model string) (data.GenRequest, error) {
	if i < 0 || i >= len(o.Statements) {
		return data.GenRequest{}, fmt.Errorf("panel statement %d: %w", i, data.ErrInvalid)
	}
	one := o
	one.Statements = []data.Statement{o.Statements[i]}
	req, err := statementsRequest(TaskPanel, "panel-", one, gc, model)
	if err != nil {
		return data.GenRequest{}, err
	}
	req.ID += fmt.Sprintf("-s%d-%s", i, model)
	return req, nil
}

// ParsePanel reads a panel answer for a statement whose cited text is cited
// (GradeCited of the statement).
func ParsePanel(text, cited string) (PanelVerdict, error) {
	var v PanelVerdict
	dec := json.NewDecoder(strings.NewReader(text))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return PanelVerdict{}, fmt.Errorf("%w: %w", ErrOutputInvalid, err)
	}
	switch v.Label {
	case data.GradeSupported, data.GradeUnsupported, data.GradeContradicted:
	default:
		return PanelVerdict{}, fmt.Errorf("%w: label %q", ErrOutputInvalid, v.Label)
	}
	sg := data.StatementGrade{Label: v.Label, Reason: v.Difference, Conflict: v.Conflict}
	if err := checkVerdict(sg, cited); err != nil {
		v.Check = err.Error()
	}
	return v, nil
}

// GradeCited returns, per statement of o, the cited text the grader compares
// it against: its cited lines and the text of its provisions, which
// GradeRequest sends once each in `provisions`.
func GradeCited(o data.GeneratedOutput, gc data.GenContext) []string {
	cited := make([]string, len(o.Statements))
	for i, s := range o.Statements {
		cited[i], _ = citedText(s, gc)
	}
	return cited
}

// concedes matches a reason that finds the statement in agreement with its
// lines. It is a backstop beside the quote check: a contradicted verdict whose
// reason says this is inconsistent and is refused.
var concedes = regexp.MustCompile(`(?i)` + strings.Join([]string{
	`\b(actually|is|are|fully|entirely) consistent\b`,
	`\bno (real |actual )?conflict\b`,
	`\bconflict is none\b`,
	`\bwhich matches\b`,
	`\bmatches too\b`,
	`\bthis is fine\b`,
}, "|"))

// ParseGrade reads the grader's answer for an output whose statements cite
// the given texts (GradeCited): every statement needs exactly one verdict with
// a known label, and every contradicted verdict must be consistent: it quotes
// in conflict words that appear in that statement's cited text, and its reason
// does not concede that the statement matches. An inconsistent verdict makes
// the whole grade invalid, so it is never stored or counted.
func ParseGrade(text string, cited []string) (data.Grade, error) {
	n := len(cited)
	var doc gradeDoc
	dec := json.NewDecoder(strings.NewReader(text))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		return data.Grade{}, fmt.Errorf("%w: %w", ErrOutputInvalid, err)
	}
	if len(doc.Statements) != n {
		return data.Grade{}, fmt.Errorf("%w: %w: %d verdicts for %d statements",
			ErrOutputInvalid, errVerdictCount, len(doc.Statements), n)
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
		if err := checkVerdict(g, cited[g.Index]); err != nil {
			return data.Grade{}, err
		}
	}
	return data.Grade{Statements: doc.Statements, Neutral: doc.Neutral}, nil
}

// GradeOutput runs the grader on one stored output and stores the grade
// beside it (generated/{task}/{bill}/v/{n}/{key}.grade.json) with the grader
// model and prompt version. It is a live run when gen is a model: callers
// only reach it from the accuracy report, never from CI. When the reply has
// the wrong number of verdicts, each statement is graded in a request of its
// own, and the verdicts are joined; the grade is neutral only if every
// request found it neutral.
func GradeOutput(
	ctx context.Context, d Deps, gen data.Generator, graderModel string, o data.GeneratedOutput, gc data.GenContext,
) (data.Grade, error) {
	g, version, err := gradeOnce(ctx, gen, graderModel, o, gc, "")
	if errors.Is(err, errVerdictCount) && len(o.Statements) > 1 {
		d.Log.Warn("grade has the wrong verdict count; grading each statement alone",
			"output", o.Key.String(), "error", err.Error())
		g, version, err = gradeEach(ctx, gen, graderModel, o, gc)
	}
	if err != nil {
		return data.Grade{}, fmt.Errorf("grade %s: %w", o.Key, err)
	}
	g.Key, g.GraderModel, g.PromptVersion, g.At = o.Key, graderModel, version, d.Now().UTC()
	if err := d.Ports.GeneratedStore.PutGrade(ctx, g); err != nil {
		return data.Grade{}, fmt.Errorf("store grade %s: %w", o.Key, err)
	}
	return g, nil
}

// gradeOnce sends one grade request for the statements of o; suffix keeps
// the request ids of a per-statement grade distinct.
func gradeOnce(
	ctx context.Context, gen data.Generator, graderModel string, o data.GeneratedOutput, gc data.GenContext, suffix string,
) (data.Grade, string, error) {
	req, err := GradeRequest(o, gc, graderModel)
	if err != nil {
		return data.Grade{}, "", err
	}
	req.ID += suffix
	res, err := gen.Generate(ctx, req)
	if err != nil {
		return data.Grade{}, "", fmt.Errorf("generate %s: %w", req.ID, err)
	}
	if res.Outcome != "ok" {
		return data.Grade{}, "", fmt.Errorf("outcome %s: %w", res.Outcome, ErrOutputInvalid)
	}
	g, err := ParseGrade(res.Text, GradeCited(o, gc))
	return g, req.PromptVersion, err
}

// gradeEach grades every statement of o in a request of its own.
func gradeEach(
	ctx context.Context, gen data.Generator, graderModel string, o data.GeneratedOutput, gc data.GenContext,
) (data.Grade, string, error) {
	joined := data.Grade{Neutral: true}
	var version string
	for i, s := range o.Statements {
		one := o
		one.Statements = []data.Statement{s}
		g, v, err := gradeOnce(ctx, gen, graderModel, one, gc, fmt.Sprintf("-s%d", i))
		if err != nil {
			return data.Grade{}, "", fmt.Errorf("statement %d: %w", i, err)
		}
		verdict := g.Statements[0]
		verdict.Index = i
		joined.Statements = append(joined.Statements, verdict)
		joined.Neutral = joined.Neutral && g.Neutral
		version = v
	}
	return joined, version, nil
}

// checkVerdict refuses a contradicted verdict that does not quote the
// conflicting cited words, quotes words the statement does not cite, or
// gives a reason that finds no conflict. Other labels must leave conflict empty.
func checkVerdict(g data.StatementGrade, cited string) error {
	if g.Label != data.GradeContradicted {
		if strings.TrimSpace(g.Conflict) != "" {
			return fmt.Errorf("%w: verdict %d is %s but quotes a conflict", ErrOutputInvalid, g.Index, g.Label)
		}
		return nil
	}
	quote := normalizeQuote(g.Conflict)
	if quote == "" {
		return fmt.Errorf("%w: contradicted verdict %d quotes no conflicting text", ErrOutputInvalid, g.Index)
	}
	if !strings.Contains(normalizeQuote(cited), quote) {
		return fmt.Errorf("%w: contradicted verdict %d quotes text its statement does not cite", ErrOutputInvalid, g.Index)
	}
	if concedes.MatchString(g.Reason) {
		return fmt.Errorf("%w: contradicted verdict %d gives a reason that finds no conflict", ErrOutputInvalid, g.Index)
	}
	return nil
}

// normalizeQuote compares quotes without regard to case, runs of white space,
// or typographic quote marks, which models often change when copying.
func normalizeQuote(s string) string {
	s = strings.NewReplacer("\u201c", "\"", "\u201d", "\"", "\u2018", "'", "\u2019", "'").Replace(s)
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}
