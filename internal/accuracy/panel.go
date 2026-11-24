package accuracy

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/generate"
)

// Panel groups: how the stored grade and the panel models agree on a statement.
const (
	PanelMisstated = "misstated" // no model finds the statement supported
	PanelSplit     = "split"     // some do, some do not
	PanelSupported = "supported" // every model finds it supported
	PanelNoVerdict = "no-verdict"
)

// panelWorkers bounds the panel requests in flight.
const panelWorkers = 4

// PanelOpinion is one panel model's view of one statement, or why it has none.
type PanelOpinion struct {
	Model   string                 `json:"model"`
	Verdict *generate.PanelVerdict `json:"verdict,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// PanelItem is one sampled statement with the stored grade, the operator's
// label so far, and the panel's opinions. It is a reading aid for the
// operator; nothing in it is stored as a grade.
type PanelItem struct {
	ID            string         `json:"id"`
	Task          string         `json:"task"`
	Statement     string         `json:"statement"`
	Cited         []string       `json:"cited"`
	OperatorLabel string         `json:"operator_label,omitempty"`
	GraderLabel   string         `json:"grader_label,omitempty"`
	Opinions      []PanelOpinion `json:"opinions"`
	Group         string         `json:"group"`
}

// RunPanel asks every model for its opinion on every sampled statement, one
// statement per request. A statement whose output is not among entries (no
// longer planned or not stored) gets an error opinion per model; a failed or
// invalid request is an error opinion too, so one bad answer never stops the
// run.
func RunPanel(
	ctx context.Context, gen data.Generator, sample []SampleItem, entries []SampleEntry, models []string,
) []PanelItem {
	byID := map[string]SampleEntry{}
	for _, e := range entries {
		for i := range e.Output.Statements {
			byID[e.Output.Key.ID()+"#"+strconv.Itoa(i)] = e
		}
	}
	items := make([]PanelItem, len(sample))
	type job struct{ item, model int }
	jobs := make(chan job)
	var wg sync.WaitGroup
	for range panelWorkers {
		wg.Go(func() {
			for j := range jobs {
				s := sample[j.item]
				items[j.item].Opinions[j.model] = panelOpinion(ctx, gen, s, byID, models[j.model])
			}
		})
	}
	for i, s := range sample {
		items[i] = PanelItem{
			ID: s.ID, Task: s.Task, Statement: s.Statement, Cited: s.Cited,
			OperatorLabel: s.OperatorLabel, GraderLabel: s.GraderLabel,
			Opinions: make([]PanelOpinion, len(models)),
		}
	}
	for i := range sample {
		for m := range models {
			jobs <- job{i, m}
		}
	}
	close(jobs)
	wg.Wait()
	for i := range items {
		items[i].Group = panelGroup(items[i])
	}
	return items
}

func panelOpinion(
	ctx context.Context, gen data.Generator, s SampleItem, byID map[string]SampleEntry, model string,
) PanelOpinion {
	op := PanelOpinion{Model: model}
	e, ok := byID[s.ID]
	if !ok {
		op.Error = "statement not found among the planned, stored outputs"
		return op
	}
	i, err := strconv.Atoi(s.ID[strings.LastIndex(s.ID, "#")+1:])
	if err != nil {
		op.Error = "statement index: " + err.Error()
		return op
	}
	req, err := generate.PanelRequest(e.Output, e.Ctx, i, model)
	if err != nil {
		op.Error = err.Error()
		return op
	}
	res, err := gen.Generate(ctx, req)
	switch {
	case err != nil:
		op.Error = err.Error()
		return op
	case res.Outcome != "ok":
		op.Error = "outcome " + res.Outcome
		return op
	}
	v, err := generate.ParsePanel(res.Text, generate.GradeCited(e.Output, e.Ctx)[i])
	if err != nil {
		op.Error = err.Error()
		return op
	}
	op.Verdict = &v
	return op
}

// panelGroup sorts an item by the labels of the stored grade and the panel.
func panelGroup(it PanelItem) string {
	var labels []string
	if it.GraderLabel != "" {
		labels = append(labels, it.GraderLabel)
	}
	for _, op := range it.Opinions {
		if op.Verdict != nil {
			labels = append(labels, op.Verdict.Label)
		}
	}
	supported := 0
	for _, l := range labels {
		if l == data.GradeSupported {
			supported++
		}
	}
	switch {
	case len(labels) == 0:
		return PanelNoVerdict
	case supported == 0:
		return PanelMisstated
	case supported == len(labels):
		return PanelSupported
	default:
		return PanelSplit
	}
}

type panelSection struct{ group, heading, intro string }

// panelSections are the groups in reading order, with their headings.
func panelSections() []panelSection {
	return []panelSection{
		{
			PanelMisstated, "Likely mis-stated",
			"No model finds these supported. Confirm quickly: check the difference named below against the cited lines.",
		},
		{PanelSplit, "Split", "The models disagree. Read these closely."},
		{PanelSupported, "All supported", "Every model finds these supported. Spot-check a few."},
		{PanelNoVerdict, "No verdict", "No model gave a usable answer; grade these from the cited lines alone."},
	}
}

// PanelMarkdown renders the panel for the operator: a count per group, then
// each group with every statement, its cited lines and each model's notes.
// generator is the model that wrote the outputs, whose supported verdicts
// weigh little.
func PanelMarkdown(items []PanelItem, graderModel, generator string) string {
	var b strings.Builder
	b.WriteString("# Operator sample 2: model panel\n\n")
	fmt.Fprintf(&b, "Each statement was checked by the stored grader (`%s`) and by each panel model, "+
		"one statement per request, against its full cited text. A reading aid only: nothing here is stored "+
		"as a grade, and your label still goes in `operator_label` in `operator-sample-2.json`. `%s` wrote "+
		"these outputs, so its `supported` verdicts weigh little; its `unsupported` or `contradicted` ones weigh more.\n\n",
		graderModel, generator)
	sections := panelSections()
	for _, sec := range sections {
		n := 0
		for _, it := range items {
			if it.Group == sec.group {
				n++
			}
		}
		fmt.Fprintf(&b, "- %s: %d\n", sec.heading, n)
	}
	for _, sec := range sections {
		first := true
		for _, it := range items {
			if it.Group != sec.group {
				continue
			}
			if first {
				fmt.Fprintf(&b, "\n## %s\n\n%s\n", sec.heading, sec.intro)
				first = false
			}
			writePanelItem(&b, it, graderModel)
		}
	}
	return b.String()
}

func writePanelItem(b *strings.Builder, it PanelItem, graderModel string) {
	label := it.OperatorLabel
	if label == "" {
		label = "not graded yet"
	}
	fmt.Fprintf(b, "\n### `%s` (%s), your label: %s\n\n> %s\n\nCited lines (shortened):\n\n",
		it.ID, it.Task, label, it.Statement)
	for _, c := range it.Cited {
		fmt.Fprintf(b, "- %s\n", c)
	}
	b.WriteString("\n")
	if it.GraderLabel != "" {
		fmt.Fprintf(b, "- **%s** (stored grade): %s\n", graderModel, it.GraderLabel)
	} else {
		fmt.Fprintf(b, "- **%s** (stored grade): none (the output was withheld before grading)\n", graderModel)
	}
	for _, op := range it.Opinions {
		if op.Verdict == nil {
			fmt.Fprintf(b, "- **%s**: no answer (%s)\n", op.Model, op.Error)
			continue
		}
		v := op.Verdict
		fmt.Fprintf(b, "- **%s**: %s\n", op.Model, v.Label)
		if v.Conflict != "" {
			fmt.Fprintf(b, "  - Conflicting words: \"%s\"\n", v.Conflict)
		}
		fmt.Fprintf(b, "  - Bill says: %s\n  - Statement says: %s\n", v.BillSays, v.StatementSays)
		if v.Difference != "" {
			fmt.Fprintf(b, "  - Difference: %s\n", v.Difference)
		}
		if v.Check != "" {
			fmt.Fprintf(b, "  - Warning: %s\n", v.Check)
		}
	}
}
