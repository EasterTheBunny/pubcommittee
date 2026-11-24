// Package fakegen is a deterministic data.Generator that calls no model and
// no network. It builds each output from the request itself (verbatim lines
// of the cited text), so outputs pass the accuracy checks and CI exercises
// every generation path. It stands in for a model only: its grader and CRS
// answers are harness checks, never measurements of model quality.
package fakegen

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// Model is the model id fakegen reports.
const Model = "fakegen"

// maxLines caps the lines one fake output quotes.
const maxLines = 4

// prevPrefix marks anchors of the earlier version in a change summary request.
const prevPrefix = "prev:"

// FNV-1a 64-bit offset basis and prime, used by contentID.
const (
	fnvOffset64 uint64 = 14695981039346656037
	fnvPrime64  uint64 = 1099511628211
)

// charsPerToken is the rough token estimate: one token per four bytes.
const charsPerToken = 4

// maxProvisionChars caps the text a provision statement quotes.
const maxProvisionChars = 200

// minLongWord is the length above which a word counts as long in a CRS check.
const minLongWord = 5

// Keys of the JSON objects fakegen returns for summaries and explanations.
const (
	keyHeadline   = "headline"
	keyStatements = "statements"
)

// Config configures a Gen. Files persists batches across processes (so a
// submit run and a later collect run see the same batch); nil keeps them in
// memory. Overrides replaces the result of a request by its ID, for tests of
// refusal, truncation and planted errors.
type Config struct {
	Files     data.FileStore
	Overrides map[string]data.GenResult
}

// Gen implements data.Generator.
type Gen struct {
	cfg Config

	mu    sync.Mutex
	next  int
	local map[string][]data.GenResult
}

var _ data.Generator = (*Gen)(nil)

// New returns a Gen. It does no work and needs no network.
func New(cfg Config) *Gen { return &Gen{cfg: cfg, local: map[string][]data.GenResult{}} }

// Submit runs every request at once and stores the results as a finished batch.
func (g *Gen) Submit(ctx context.Context, reqs []data.GenRequest) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("submit: %w", err)
	}
	if len(reqs) == 0 {
		return "", fmt.Errorf("submit: no requests: %w", data.ErrInvalid)
	}
	seen := map[string]bool{}
	results := make([]data.GenResult, 0, len(reqs))
	for _, r := range reqs {
		if err := validate(r); err != nil {
			return "", fmt.Errorf("submit: %w", err)
		}
		if seen[r.ID] {
			return "", fmt.Errorf("submit: duplicate request ID %q: %w", r.ID, data.ErrInvalid)
		}
		seen[r.ID] = true
		results = append(results, g.result(r))
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.next++
	id := fmt.Sprintf("fake-%d", g.next)
	if g.cfg.Files == nil {
		g.local[id] = results
		return id, nil
	}
	// A later process starts counting at 1 again: derive the id from the
	// content so batches of different runs never share a file.
	id = "fake-" + contentID(results)
	body, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("submit: encode: %w: %w", data.ErrInvalid, err)
	}
	if _, err := g.cfg.Files.PutReplaceable(ctx, "fakegen/"+id+".json", body); err != nil {
		return "", fmt.Errorf("submit: store batch: %w", err)
	}
	return id, nil
}

// Collect returns the results of a batch; fake batches are always done.
func (g *Gen) Collect(ctx context.Context, batchID string) ([]data.GenResult, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, fmt.Errorf("collect: %w", err)
	}
	if g.cfg.Files == nil {
		g.mu.Lock()
		defer g.mu.Unlock()
		res, ok := g.local[batchID]
		if !ok {
			return nil, false, fmt.Errorf("collect batch %s: %w", batchID, data.ErrNotFound)
		}
		return append([]data.GenResult(nil), res...), true, nil
	}
	body, err := g.cfg.Files.GetFile(ctx, "fakegen/"+batchID+".json")
	if err != nil {
		return nil, false, fmt.Errorf("collect batch %s: %w", batchID, err)
	}
	var res []data.GenResult
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, false, fmt.Errorf("collect batch %s: decode: %w: %w", batchID, data.ErrInvalid, err)
	}
	return res, true, nil
}

// Generate runs one request synchronously.
func (g *Gen) Generate(ctx context.Context, req data.GenRequest) (data.GenResult, error) {
	if err := ctx.Err(); err != nil {
		return data.GenResult{}, fmt.Errorf("generate: %w", err)
	}
	if err := validate(req); err != nil {
		return data.GenResult{}, fmt.Errorf("generate: %w", err)
	}
	return g.result(req), nil
}

func validate(r data.GenRequest) error {
	if r.ID == "" || r.Task == "" {
		return fmt.Errorf("request ID %q task %q: %w", r.ID, r.Task, data.ErrInvalid)
	}
	return nil
}

func contentID(results []data.GenResult) string {
	h := fnvOffset64
	for _, r := range results {
		for _, c := range []byte(r.ID + r.Text) {
			h = (h ^ uint64(c)) * fnvPrime64
		}
	}
	return fmt.Sprintf("%016x", h)
}

func (g *Gen) result(r data.GenRequest) data.GenResult {
	if o, ok := g.cfg.Overrides[r.ID]; ok {
		o.ID = r.ID
		return o
	}
	text := synth(r)
	return data.GenResult{
		ID: r.ID, Outcome: "ok", Text: text, Model: Model, Stop: "end_turn",
		TokensIn:  (len(r.Source) + len(r.Input) + len(r.Instructions)) / charsPerToken,
		TokensOut: len(text) / charsPerToken,
	}
}

// reqDoc is the part of the request document fakegen reads.
type reqDoc struct {
	Task       string   `json:"task"`
	Anchors    []string `json:"anchors"`
	Provisions []struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	} `json:"provisions"`
	Changes []struct {
		Kind   string   `json:"kind"`
		Before []string `json:"before"`
		After  []string `json:"after"`
	} `json:"changes"`
	Statements []struct {
		Index int    `json:"index"`
		Text  string `json:"text"`
		Cited string `json:"cited"`
	} `json:"statements"`
	Reference string `json:"reference"`
	Generated string `json:"generated"`
}

type stmt struct {
	Role         string   `json:"role,omitempty"`
	Text         string   `json:"text"`
	Anchors      []string `json:"anchors"`
	ProvisionIDs []string `json:"provision_ids"`
}

func requestOf(input string) (reqDoc, bool) {
	_, rest, ok := strings.Cut(input, "<request>")
	if !ok {
		return reqDoc{}, false
	}
	body, _, _ := strings.Cut(rest, "</request>")
	var d reqDoc
	return d, json.Unmarshal([]byte(body), &d) == nil
}

// source is the rendered bill text of a request: anchors in document order
// and the text of each.
type source struct {
	order []string
	text  map[string]string
}

func parseSource(src string) source {
	re := regexp.MustCompile(`(?m)^\[(\S+)\] (.*)$`)
	out := source{text: map[string]string{}}
	for _, m := range re.FindAllStringSubmatch(src, -1) {
		out.order = append(out.order, m[1])
		out.text[m[1]] = m[2]
	}
	return out
}

// synth builds the output text of a request; outputs of unknown shape are a
// single anchorless statement, which the collect phase rejects.
func synth(r data.GenRequest) string {
	d, ok := requestOf(r.Input)
	if !ok {
		fake := stmt{Text: "fake output", Anchors: []string{}, ProvisionIDs: []string{}}
		return marshal(map[string]any{keyHeadline: "", keyStatements: []stmt{fake}})
	}
	lines := parseSource(r.Source)
	switch d.Task {
	case data.TaskBillSummary:
		return summary(lines)
	case data.TaskVersionChanges:
		return changes(d, lines)
	case data.TaskChangeExplanation, data.TaskBillExplanation:
		return explanation(d, lines)
	case data.TaskGrade:
		return grade(d)
	case "crs":
		return crs(d)
	}
	return marshal(map[string]any{keyHeadline: "", keyStatements: []stmt{}})
}

func marshal(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func summary(lines source) string {
	var out []stmt
	for _, a := range lines.order {
		if len(lines.text[a]) < 30 || strings.HasPrefix(a, prevPrefix) {
			continue
		}
		out = append(out, stmt{Text: lines.text[a], Anchors: []string{a}, ProvisionIDs: []string{}})
		if len(out) == maxLines {
			break
		}
	}
	return marshal(map[string]any{keyHeadline: "Summary of the bill text.", keyStatements: out})
}

func changes(d reqDoc, lines source) string {
	out := []stmt{}
	for _, c := range d.Changes {
		var anchors []string
		var text string
		switch {
		case len(c.After) > 0:
			anchors, text = c.After[:1], "Changed text: "+lines.text[c.After[0]]
		case len(c.Before) > 0:
			anchors, text = c.Before[:1], "Removed text: "+lines.text[c.Before[0]]
		default:
			continue
		}
		out = append(out, stmt{Text: text, Anchors: anchors, ProvisionIDs: []string{}})
		if len(out) == maxLines {
			break
		}
	}
	return marshal(map[string]any{keyHeadline: "Changes from the previous version.", keyStatements: out})
}

func explanation(d reqDoc, lines source) string {
	out := make([]stmt, 0, len(d.Provisions)+1)
	anchors := d.Anchors
	if len(anchors) > maxLines {
		anchors = anchors[:maxLines]
	}
	ids := make([]string, 0, len(d.Provisions))
	for _, p := range d.Provisions {
		ids = append(ids, p.ID)
		out = append(out, stmt{
			Role:         data.RoleNow,
			Text:         firstChars(p.Text, maxProvisionChars),
			Anchors:      anchors[:min(1, len(anchors))],
			ProvisionIDs: []string{p.ID},
		})
	}
	parts := make([]string, 0, len(anchors))
	for _, a := range anchors {
		parts = append(parts, lines.text[a])
	}
	out = append(out, stmt{
		Role:         data.RoleChanges,
		Text:         strings.Join(parts, " "),
		Anchors:      anchors,
		ProvisionIDs: nonNil(ids),
	})
	return marshal(map[string]any{keyHeadline: "", keyStatements: out})
}

func firstChars(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// grade answers a grade request: a statement is supported when every number
// in it appears in the cited text, unsupported otherwise (a number the lines
// do not state). It quotes no conflict, so it never says contradicted. It is a
// harness check, not a model.
func grade(d reqDoc) string {
	num := regexp.MustCompile(`\d[\d,.]*`)
	type sg struct {
		Index    int    `json:"index"`
		Label    string `json:"label"`
		Reason   string `json:"reason"`
		Conflict string `json:"conflict"`
	}
	out := make([]sg, 0, len(d.Statements))
	for _, s := range d.Statements {
		label, reason := data.GradeSupported, "every number appears in the cited lines"
		for _, n := range num.FindAllString(s.Text, -1) {
			if !strings.Contains(s.Cited, strings.TrimRight(n, ".,")) {
				label, reason = data.GradeUnsupported, "number "+n+" is not in the cited lines"
				break
			}
		}
		out = append(out, sg{Index: s.Index, Label: label, Reason: reason})
	}
	neutral := true
	for _, s := range d.Statements {
		if strings.Contains(strings.ToLower(s.Text), "unfortunately") {
			neutral = false
		}
	}
	return marshal(map[string]any{keyStatements: out, "neutral": neutral})
}

// crs answers a coverage request: a reference sentence is covered when most
// of its long words appear in the generated summary.
func crs(d reqDoc) string {
	type pt struct {
		Point   string `json:"point"`
		Verdict string `json:"verdict"`
	}
	gen := strings.ToLower(d.Generated)
	out := []pt{}
	for _, sent := range strings.FieldsFunc(d.Reference, func(r rune) bool { return r == '.' || r == '\n' }) {
		words := 0
		hit := 0
		for w := range strings.FieldsSeq(strings.ToLower(sent)) {
			if len(w) > minLongWord {
				words++
				if strings.Contains(gen, w) {
					hit++
				}
			}
		}
		if words == 0 {
			continue
		}
		v := "misses"
		if hit*2 >= words {
			v = "covers"
		}
		out = append(out, pt{Point: strings.TrimSpace(sent), Verdict: v})
	}
	return marshal(map[string]any{"points": out})
}
