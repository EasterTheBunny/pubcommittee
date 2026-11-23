package generate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// ModelOutput is the structured output of a generation task.
type ModelOutput struct {
	Headline   string           `json:"headline"`
	Statements []data.Statement `json:"statements"`
}

// ErrOutputInvalid means the model text is not a JSON document of the schema.
var ErrOutputInvalid = errors.New("model output does not match the schema")

// ParseOutput decodes the model text strictly: unknown fields, trailing
// data, an empty statement list, empty text or a bad role are errors.
func ParseOutput(text string) (ModelOutput, error) {
	var out ModelOutput
	dec := json.NewDecoder(strings.NewReader(text))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return ModelOutput{}, fmt.Errorf("%w: %w", ErrOutputInvalid, err)
	}
	if dec.More() {
		return ModelOutput{}, fmt.Errorf("%w: trailing data", ErrOutputInvalid)
	}
	if len(out.Statements) == 0 {
		return ModelOutput{}, fmt.Errorf("%w: no statements", ErrOutputInvalid)
	}
	for i, s := range out.Statements {
		if strings.TrimSpace(s.Text) == "" {
			return ModelOutput{}, fmt.Errorf("%w: statement %d is empty", ErrOutputInvalid, i)
		}
		if s.Role != "" && !slices.Contains([]string{data.RoleNow, data.RoleChanges, data.RoleWhy}, s.Role) {
			return ModelOutput{}, fmt.Errorf("%w: statement %d role %q", ErrOutputInvalid, i, s.Role)
		}
	}
	return out, nil
}

// expandAnchor lists the single-line keys of an anchor ("s4-l12-14" ->
// s4-l12, s4-l13, s4-l14), keeping a PrevPrefix. ok is false for a malformed
// anchor.
func expandAnchor(a string) (keys []string, ok bool) {
	prefix := ""
	if rest, found := strings.CutPrefix(a, PrevPrefix); found {
		prefix, a = PrevPrefix, rest
	}
	section, from, to, ok := data.ParseAnchorID(a)
	if !ok {
		return nil, false
	}
	for n := from; n <= to; n++ {
		keys = append(keys, prefix+data.AnchorID(section, n, n))
	}
	return keys, true
}

// Validate runs every deterministic check of the collect phase on a parsed
// output and returns its findings; none means the output may be published.
// Checks: anchors exist and every statement has one, provision ids exist,
// fact fidelity, amendment consistency (explanations), neutral wording.
func Validate(task string, out ModelOutput, gc data.GenContext) []string {
	var findings []string
	all := make([]string, 0, len(out.Statements)) // cited text of every statement, for the headline
	for i, s := range out.Statements {
		cited, fs := citedText(s, gc)
		for _, f := range fs {
			findings = append(findings, fmt.Sprintf("statement %d: %s", i+1, f))
		}
		all = append(all, cited)
		for _, f := range FactFidelity(withoutCitations(s.Text, gc), cited) {
			findings = append(findings, fmt.Sprintf("statement %d: fact not in cited lines: %s", i+1, f))
		}
		for _, w := range Neutrality(s.Text) {
			findings = append(findings, fmt.Sprintf("statement %d: evaluative wording %q", i+1, w))
		}
	}
	if out.Headline != "" {
		for _, f := range FactFidelity(withoutCitations(out.Headline, gc), strings.Join(all, "\n")) {
			findings = append(findings, "headline: fact not in cited lines: "+f)
		}
		for _, w := range Neutrality(out.Headline) {
			findings = append(findings, fmt.Sprintf("headline: evaluative wording %q", w))
		}
	}
	if task == data.TaskChangeExplanation || task == data.TaskBillExplanation {
		findings = append(findings, explanationFindings(out, gc)...)
	}
	return findings
}

// citedText returns the text a statement rests on (cited lines, provisions
// and the anchor and provision ids themselves) and the anchor findings.
func citedText(s data.Statement, gc data.GenContext) (string, []string) {
	var findings []string
	var b bytes.Buffer
	if len(s.Anchors) == 0 {
		findings = append(findings, "no anchor")
	}
	for _, a := range s.Anchors {
		b.WriteString(a + "\n")
		keys, ok := expandAnchor(a)
		if !ok {
			findings = append(findings, fmt.Sprintf("anchor %q is malformed", a))
			continue
		}
		for _, k := range keys {
			text, found := gc.Lines[k]
			if !found {
				findings = append(findings, fmt.Sprintf("anchor %q cites a line that is not in the input", a))
				break
			}
			b.WriteString(text + "\n")
		}
	}
	for _, id := range s.ProvisionIDs {
		b.WriteString(id + "\n")
		text, found := lookupProvision(gc, id)
		if !found {
			findings = append(findings, fmt.Sprintf("provision %q is not in the input", id))
		}
		b.WriteString(text + "\n")
	}
	return b.String(), findings
}

// explanationFindings checks the amendment consistency of the statements
// that describe the change against the lines they cite.
func explanationFindings(out ModelOutput, gc data.GenContext) []string {
	var changes, lines []string
	var ids []string
	hasChange := false
	for _, s := range out.Statements {
		if s.Role != data.RoleChanges {
			continue
		}
		hasChange = true
		changes = append(changes, s.Text)
		ids = append(ids, s.ProvisionIDs...)
		for _, a := range s.Anchors {
			keys, _ := expandAnchor(a)
			for _, k := range keys {
				lines = append(lines, gc.Lines[k])
			}
		}
	}
	var findings []string
	if !hasChange {
		findings = append(findings, "no statement with role changes")
	}
	for _, f := range AmendmentConsistency(strings.Join(changes, " "), strings.Join(lines, "\n"), ids) {
		findings = append(findings, "amendment consistency: "+f)
	}
	return findings
}
