package accuracy

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// prevPrefix marks anchors of the earlier version in a version change
// summary; the keys of GenContext.Lines carry it too.
const prevPrefix = "prev:"

// sampleIntro is the instruction at the head of the operator sample.
const sampleIntro = "Read each statement against ONLY its cited lines, shown under it. " +
	"Write your label into the JSON file next to this one, in the field operator_label, " +
	"on the item with the same id.\n\n"

// SampleEntry is one output with the context needed to show its cited lines.
type SampleEntry struct {
	Output data.GeneratedOutput
	Ctx    data.GenContext
	Grade  *data.Grade // nil when not graded
}

// AgreementResult compares operator labels with grader labels.
type AgreementResult struct {
	Compared   int     // items with both labels
	Agreed     int     // items whose labels are equal
	Missed     int     // operator contradicted, grader did not
	FalseAlarm int     // grader contradicted, operator supported
	Share      float64 // Agreed / Compared; 0 when none compared
}

// BuildSample picks up to n statements from the ready entries for the
// operator to grade against their cited lines. The choice is deterministic:
// entries are sorted by key, and statements are taken round-robin across the
// tasks, sorted by name.
func BuildSample(entries []SampleEntry, n int) []SampleItem {
	if n <= 0 {
		return nil
	}
	queues, tasks := taskQueues(readyEntries(entries))
	var out []SampleItem
	for round := 0; len(out) < n && round < longest(queues); round++ {
		for _, t := range tasks {
			if round < len(queues[t]) && len(out) < n {
				out = append(out, queues[t][round])
			}
		}
	}
	return out
}

// readyEntries returns the entries that may be published, sorted by key.
func readyEntries(entries []SampleEntry) []SampleEntry {
	ready := make([]SampleEntry, 0, len(entries))
	for _, e := range entries {
		if e.Output.Status == data.GenReady {
			ready = append(ready, e)
		}
	}
	sort.SliceStable(ready, func(i, j int) bool {
		return ready[i].Output.Key.String() < ready[j].Output.Key.String()
	})
	return ready
}

// taskQueues lists the statements of each task in entry order, and the task
// names sorted.
func taskQueues(ready []SampleEntry) (map[string][]SampleItem, []string) {
	queues := map[string][]SampleItem{}
	for _, e := range ready {
		task := e.Output.Key.Task
		for i := range e.Output.Statements {
			queues[task] = append(queues[task], sampleItem(e, i))
		}
	}
	tasks := make([]string, 0, len(queues))
	for t := range queues {
		tasks = append(tasks, t)
	}
	slices.Sort(tasks)
	return queues, tasks
}

// longest is the length of the longest queue.
func longest(queues map[string][]SampleItem) int {
	n := 0
	for _, q := range queues {
		n = max(n, len(q))
	}
	return n
}

// sampleItem is statement i of the entry, with its cited rows and grader label.
func sampleItem(e SampleEntry, i int) SampleItem {
	st := e.Output.Statements[i]
	item := SampleItem{
		ID:        e.Output.Key.ID() + "#" + strconv.Itoa(i),
		Key:       e.Output.Key.String(),
		Task:      e.Output.Key.Task,
		Statement: st.Text,
		Cited:     citedRows(e.Ctx, st),
	}
	if e.Grade != nil {
		item.GraderLabel = gradeLabel(*e.Grade, i)
	}
	return item
}

// gradeLabel is the grader's label of statement index i, or "" when absent.
func gradeLabel(g data.Grade, i int) string {
	for _, s := range g.Statements {
		if s.Index == i {
			return s.Label
		}
	}
	return ""
}

// citedRows lists the line rows of each anchor, then the provision rows.
func citedRows(ctx data.GenContext, st data.Statement) []string {
	rows := make([]string, 0, len(st.Anchors)+len(st.ProvisionIDs))
	for _, a := range st.Anchors {
		rows = append(rows, anchorRows(ctx, a)...)
	}
	for _, id := range st.ProvisionIDs {
		rows = append(rows, id+": "+ctx.Provisions[id])
	}
	return rows
}

// anchorRows expands an anchor, possibly a range or a prev: anchor, into one
// "<anchor>: <line text>" row per line.
func anchorRows(ctx data.GenContext, anchor string) []string {
	prefix, core := "", anchor
	if rest, ok := strings.CutPrefix(anchor, prevPrefix); ok {
		prefix, core = prevPrefix, rest
	}
	section, from, to, ok := data.ParseAnchorID(core)
	if !ok {
		return []string{anchor + ": " + ctx.Lines[anchor]}
	}
	rows := make([]string, 0, to-from+1)
	for l := from; l <= to; l++ {
		id := prefix + data.AnchorID(section, l, l)
		rows = append(rows, id+": "+ctx.Lines[id])
	}
	return rows
}

// SampleMarkdown renders the sample for the operator. It never shows the
// grader's labels.
func SampleMarkdown(items []SampleItem) string {
	var b strings.Builder
	b.WriteString("# Operator sample\n\n")
	b.WriteString(sampleIntro)
	for i, it := range items {
		fmt.Fprintf(&b, "%d. %s\n\n", i+1, oneLine(it.Statement))
		fmt.Fprintf(&b, "   Item `%s` (%s)\n\n", it.ID, it.Task)
		if len(it.Cited) == 0 {
			b.WriteString("   > (no cited rows)\n\n")
		}
		for _, row := range it.Cited {
			fmt.Fprintf(&b, "   > %s\n", oneLine(row))
		}
		b.WriteString("\n   Your label (supported / unsupported / contradicted): ______\n\n")
	}
	return b.String()
}

// oneLine keeps a markdown list item or block quote row on one line.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// Agreement compares operator labels with grader labels over the items that
// have both.
func Agreement(items []SampleItem) AgreementResult {
	var r AgreementResult
	for _, it := range items {
		if it.OperatorLabel == "" || it.GraderLabel == "" {
			continue
		}
		r.Compared++
		if it.OperatorLabel == it.GraderLabel {
			r.Agreed++
		}
		if it.OperatorLabel == data.GradeContradicted && it.GraderLabel != data.GradeContradicted {
			r.Missed++
		}
		if it.GraderLabel == data.GradeContradicted && it.OperatorLabel == data.GradeSupported {
			r.FalseAlarm++
		}
	}
	if r.Compared > 0 {
		r.Share = float64(r.Agreed) / float64(r.Compared)
	}
	return r
}
