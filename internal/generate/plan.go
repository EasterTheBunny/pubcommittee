package generate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/easterthebunny/pubcommittee/internal/billxml"
	"github.com/easterthebunny/pubcommittee/internal/citations"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/versiondiff"
)

const effortMedium = "medium"

// Config chooses the model and the effort of each task.
type Config struct {
	Model   string
	Efforts map[string]string
}

// DefaultConfig is the M1 choice: the architecture's model, effort set
// explicitly to medium for every task (tuned later from the accuracy report).
func DefaultConfig() Config {
	return Config{
		Model: DefaultModel,
		Efforts: map[string]string{
			data.TaskBillSummary: effortMedium, data.TaskChangeExplanation: effortMedium,
			data.TaskBillExplanation: effortMedium, data.TaskVersionChanges: effortMedium,
		},
	}
}

// Task is one planned generation: its key, the model request, and the
// context the collect phase checks the output against.
type Task struct {
	Key data.GenKey
	Req data.GenRequest
	Ctx data.GenContext
}

// request is the volatile JSON document of one request.
type request struct {
	Task       string          `json:"task"`
	Anchors    []string        `json:"anchors,omitempty"`
	Section    string          `json:"section,omitempty"`
	Provisions []requestLaw    `json:"provisions,omitempty"`
	Changes    []requestChange `json:"changes,omitempty"`
	From       string          `json:"from,omitempty"`
	To         string          `json:"to,omitempty"`
}

type requestLaw struct {
	ID       string `json:"id"`
	Citation string `json:"citation"`
	Text     string `json:"text"`
}

type requestChange struct {
	Kind   string   `json:"kind"`
	Before []string `json:"before,omitempty"`
	After  []string `json:"after,omitempty"`
}

// PlanVersion builds every task of one stored version. prev is the previous
// version, nil for the first. Citations and comparisons come from the ports;
// ones that are not stored simply yield no tasks of their kind, with a note.
func PlanVersion(
	ctx context.Context, p data.PlatformPorts, ver data.OfficialVersion, prev *data.OfficialVersion, cfg Config,
) (tasks []Task, notes []string, err error) {
	b := builder{cfg: cfg, ver: ver}
	t, err := b.summary(ctx, p)
	if err != nil {
		return nil, nil, err
	}
	tasks = append(tasks, t)
	if prev != nil {
		ct, note, err := b.changes(ctx, p, *prev)
		if err != nil {
			return nil, nil, err
		}
		tasks = append(tasks, ct...)
		notes = append(notes, note...)
	}
	et, note, err := b.explanations(ctx, p)
	if err != nil {
		return nil, nil, err
	}
	return append(tasks, et...), append(notes, note...), nil
}

type builder struct {
	cfg Config
	ver data.OfficialVersion
}

// task assembles one Task: the cached prefix is the shared instructions plus
// the source; the volatile part is the task text and the request document.
func (b builder) task(name, target, source string, req request, gc data.GenContext) (Task, error) {
	pr, err := LoadPrompt(name)
	if err != nil {
		return Task{}, err
	}
	doc, err := json.Marshal(req)
	if err != nil {
		return Task{}, fmt.Errorf("request %s: %w", name, err)
	}
	key := data.GenKey{Task: name, Version: b.ver.Ref, Target: target, PromptVersion: pr.Version, Model: b.cfg.Model}
	return Task{
		Key: key,
		Req: data.GenRequest{
			ID: key.ID(), Task: name, Instructions: pr.Instructions, Source: source,
			Input:  "<task>\n" + pr.Task + "</task>\n<request>" + string(doc) + "</request>",
			Schema: pr.Schema, Effort: b.cfg.Efforts[name], PromptVersion: pr.Version, Model: b.cfg.Model,
		},
		Ctx: gc,
	}, nil
}

// summary plans the bill summary. The model is not given the existing-law texts, but it names the provisions the
// bill amends, so the context lists the provisions of the stored citation set (ids only, no text) for the check
// that a cited provision exists; their texts never count as support for a fact.
func (b builder) summary(ctx context.Context, p data.PlatformPorts) (Task, error) {
	gc := data.GenContext{Lines: map[string]string{}}
	lineMap(b.ver, "", gc.Lines)
	if set, note, err := citationSet(ctx, p, b.ver.Ref); err == nil && note == "" {
		gc.Provisions = make(map[string]string, len(set.Provisions))
		for _, cp := range set.Provisions {
			gc.Provisions[cp.ID] = ""
		}
	}
	src := renderSource("bill_source", b.ver, "")
	return b.task(data.TaskBillSummary, "", src, request{Task: data.TaskBillSummary}, gc)
}

func (b builder) changes(
	ctx context.Context, p data.PlatformPorts, prev data.OfficialVersion,
) ([]Task, []string, error) {
	if p.Comparisons == nil {
		return nil, []string{"version change summary skipped: no comparison store"}, nil
	}
	ref := data.ComparisonRef{
		Bill: b.ver.Ref.Bill, From: prev.Ref.Version, To: b.ver.Ref.Version,
		Method: versiondiff.Method(billxml.ParserVersion),
	}
	c, err := p.Comparisons.GetComparison(ctx, ref)
	if errors.Is(err, data.ErrNotFound) {
		return nil, []string{"version change summary skipped: no comparison of " + ref.String()}, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get comparison %s: %w", ref, err)
	}
	req := request{Task: data.TaskVersionChanges, From: prev.Ref.Version, To: b.ver.Ref.Version}
	for _, sc := range c.Sections {
		for _, ch := range sc.Changes {
			req.Changes = append(req.Changes, requestChange{
				Kind:   ch.Kind,
				Before: spanAnchors(PrevPrefix, ch.BeforeSection, ch.BeforeFrom, ch.BeforeTo),
				After:  spanAnchors("", ch.AfterSection, ch.AfterFrom, ch.AfterTo),
			})
		}
	}
	if len(req.Changes) == 0 {
		return nil, []string{"version change summary skipped: no differences"}, nil
	}
	gc := data.GenContext{Lines: map[string]string{}}
	lineMap(b.ver, "", gc.Lines)
	lineMap(prev, PrevPrefix, gc.Lines)
	source := renderSource("bill_source", b.ver, "") + renderSource("previous_version_source", prev, PrevPrefix)
	t, err := b.task(data.TaskVersionChanges, prev.Ref.Version, source, req, gc)
	if err != nil {
		return nil, nil, err
	}
	return []Task{t}, nil, nil
}

// spanAnchors lists the single-line anchors of a line span, nil for none.
func spanAnchors(prefix string, section []string, from, to int) []string {
	if from < 1 {
		return nil
	}
	out := make([]string, 0, to-from+1)
	for n := from; n <= to; n++ {
		out = append(out, prefix+data.AnchorID(section, n, n))
	}
	return out
}

// explanations plans one task per (section, resolved provision) and one
// bill-only task per section with no resolved citation.
func (b builder) explanations(ctx context.Context, p data.PlatformPorts) ([]Task, []string, error) {
	set, note, err := citationSet(ctx, p, b.ver.Ref)
	if err != nil || note != "" {
		if note != "" {
			return nil, []string{note}, nil
		}
		return nil, nil, err
	}
	tasks, cited, err := b.linked(set)
	if err != nil {
		return nil, nil, err
	}
	free, err := b.freestanding(cited)
	if err != nil {
		return nil, nil, err
	}
	tasks = append(tasks, free...)
	slices.SortFunc(tasks, func(a, c Task) int { return strings.Compare(a.Key.String(), c.Key.String()) })
	return tasks, nil, nil
}

// linked plans one explanation per resolved (section, provision) pair of the
// set, and returns the ids of the sections it covered.
func (b builder) linked(set data.CitationSet) (tasks []Task, cited map[string]bool, err error) {
	byID := map[string]data.CitedProvision{}
	for _, cp := range set.Provisions {
		byID[cp.ID] = cp
	}
	cited = map[string]bool{}
	seen := map[string]bool{}
	for _, l := range set.Links {
		sec := sectionNamed(b.ver, l.Section)
		if sec == nil || (l.State != data.CitationResolved && l.State != data.CitationStewardConfirmed) {
			continue
		}
		cited[strings.Join(l.Section, ".")] = true
		for _, id := range l.ProvisionIDs {
			target := strings.Join(l.Section, ".") + "|" + id
			cp, ok := byID[id]
			if !ok || seen[target] {
				continue
			}
			seen[target] = true
			t, err := b.explanation(data.TaskChangeExplanation, target, *sec, []data.CitedProvision{cp})
			if err != nil {
				return nil, nil, err
			}
			tasks = append(tasks, t)
		}
	}
	return tasks, cited, nil
}

// freestanding plans a bill-only explanation for each section not in cited,
// except a section that only gives the Act its short title: its text says all
// there is to say, so an explanation would only repeat it (operator feedback,
// design/tasks/m1/FEEDBACK.md item 2).
func (b builder) freestanding(cited map[string]bool) ([]Task, error) {
	var tasks []Task
	for _, s := range b.ver.Sections {
		if len(s.Lines) == 0 || cited[strings.Join(s.Path, ".")] || shortTitleOnly(s) {
			continue
		}
		t, err := b.explanation(data.TaskBillExplanation, strings.Join(s.Path, "."), s, nil)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// citationSet loads the stored citation set of a version; a non-empty note
// says why there is none (not an error).
func citationSet(ctx context.Context, p data.PlatformPorts, v data.VersionRef) (data.CitationSet, string, error) {
	if p.Citations == nil || p.Law == nil {
		return data.CitationSet{}, "explanations skipped: no citation store", nil
	}
	rel, err := p.Law.CurrentRelease(ctx)
	if errors.Is(err, data.ErrNotFound) {
		return data.CitationSet{}, "explanations skipped: no existing-law release stored", nil
	}
	if err != nil {
		return data.CitationSet{}, "", fmt.Errorf("current release: %w", err)
	}
	set, err := p.Citations.GetCitations(ctx, data.CitationRef{
		Version: v, Method: citations.Method(billxml.ParserVersion, rel.ID),
	})
	if errors.Is(err, data.ErrNotFound) {
		return data.CitationSet{}, "explanations skipped: no citation set for " + v.String(), nil
	}
	if err != nil {
		return data.CitationSet{}, "", fmt.Errorf("get citations: %w", err)
	}
	return set, "", nil
}

func (b builder) explanation(name, target string, sec data.Section, laws []data.CitedProvision) (Task, error) {
	sid := strings.Join(sec.Path, ".")
	gc := data.GenContext{Lines: map[string]string{}, Provisions: map[string]string{}, Section: sec.Path}
	for _, l := range sec.Lines {
		gc.Lines[data.AnchorID(sec.Path, l.N, l.N)] = l.Text
	}
	req := request{Task: name, Section: sid, Anchors: sectionAnchors(sec)}
	for _, cp := range laws {
		gc.Provisions[cp.ID] = cp.Text
		gc.ProvisionID = cp.ID
		req.Provisions = append(req.Provisions, requestLaw{ID: cp.ID, Citation: cp.Citation, Text: cp.Text})
	}
	return b.task(name, target, renderSource("bill_source", b.ver, ""), req, gc)
}

// sectionNamed returns the section with this path, nil if the version has none.
func sectionNamed(v data.OfficialVersion, path []string) *data.Section {
	for i := range v.Sections {
		if slices.Equal(v.Sections[i].Path, path) {
			return &v.Sections[i]
		}
	}
	return nil
}

// shortTitleOnly reports whether every line of s only names the Act ("This Act
// may be cited as ..."), as a short-title section does. A section that also
// carries a table of contents or other provisions is not.
func shortTitleOnly(s data.Section) bool {
	if len(s.Lines) == 0 {
		return false
	}
	for _, l := range s.Lines {
		t := strings.ToLower(strings.TrimSpace(l.Text))
		if !strings.Contains(t, " may be cited as ") {
			return false
		}
	}
	return true
}
