package accuracy

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// Pending is what a number that needs a live model run shows until the run.
const Pending = "pending (live run not approved)"

// Input is everything the report shows.
type Input struct {
	At        time.Time
	Generator string // model id of the outputs, or "fakegen"
	// Live is true when Generator is a real model and the grader and CRS
	// numbers come from a live run; false means harness only.
	Live              bool
	GraderModel       string
	Bills             []string
	Prompts           map[string]string // task -> prompt version
	Stats             map[string]*TaskStats
	Planted           []PlantRow
	SampleSize        int
	Agreement         *AgreementResult
	CRS               []CRSRow
	CommandLine       string
	DeterministicOnly bool
}

func pct(n, total int) string {
	if total == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f%% (%d/%d)", 100*float64(n)/float64(total), n, total)
}

// Render writes the report. Numbers that need a live run read Pending unless
// the run was live; fakegen runs are labelled harness-only and never given
// as a measure of model quality.
func Render(in Input) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Accuracy report (M1-09)\n\nGenerated %s by `cmd/accuracy-report`.\n\n",
		in.At.UTC().Format("2006-01-02"))
	fmt.Fprintf(&b, "- Generator: `%s`%s\n- Grader: `%s` (must differ from the generator)\n- Bills: %s\n",
		in.Generator, liveNote(in.Live), orPending(in.GraderModel), strings.Join(in.Bills, ", "))
	if len(in.Prompts) > 0 {
		fmt.Fprintf(&b, "- Prompt versions: %s\n", promptList(in.Prompts))
	}
	if in.CommandLine != "" {
		fmt.Fprintf(&b, "- Reproduce: `%s`\n", in.CommandLine)
	}
	b.WriteString("\n")
	renderTasks(&b, in)
	renderPlanted(&b, in)
	renderOperator(&b, in)
	renderCRS(&b, in)
	renderTarget(&b, in)
	if !in.Live {
		renderLiveRun(&b)
	}
	return b.String()
}

func liveNote(live bool) string {
	if live {
		return " (live run)"
	}
	return " (harness only: deterministic outputs built from the bill text; no model quality is measured)"
}

func orPending(s string) string {
	if s == "" {
		return "pending"
	}
	return s
}

func promptList(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + m[k]
	}
	return strings.Join(parts, ", ")
}

func renderTasks(b *strings.Builder, in Input) {
	b.WriteString("## Per task\n\n")
	b.WriteString("| Task | Outputs | Ready | Unverified | Withheld | Refused/truncated/invalid | " +
		"Supported | Contradicted | Fidelity fail | Amendment fail | Neutral-wording fail |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |\n")
	tasks := make([]string, 0, len(in.Stats))
	for t := range in.Stats {
		tasks = append(tasks, t)
	}
	slices.Sort(tasks)
	for _, t := range tasks {
		s := in.Stats[t]
		sup, con := Pending, Pending
		if s.Graded > 0 {
			sup, con = pct(s.Labels[data.GradeSupported], s.Graded), pct(s.Labels[data.GradeContradicted], s.Graded)
		}
		bad := s.ByStatus[data.GenRefusal] + s.ByStatus[data.GenTruncated] + s.ByStatus[data.GenInvalid]
		fail := func(check string) string {
			if !in.Live {
				return pct(s.Failures[check], s.Outputs) + " (harness)"
			}
			return pct(s.Failures[check], s.Outputs)
		}
		fmt.Fprintf(b, "| %s | %d | %d | %d | %d | %d | %s | %s | %s | %s | %s |\n", t, s.Outputs,
			s.ByStatus[data.GenReady], s.ByStatus[data.GenUnverified], s.ByStatus[data.GenWithheld], bad,
			sup, con, fail(CheckFidelity), fail(CheckAmendment), fail(CheckNeutral))
	}
	b.WriteString("\nValidator failure rates are shares of outputs with at least one finding. " +
		"Supported and contradicted shares are shares of graded statements, reported separately.\n\n")
}

func renderPlanted(b *strings.Builder, in Input) {
	b.WriteString("## Planted errors: catch rate per check\n\n")
	b.WriteString("Known errors injected into good outputs. The deterministic checks run in CI on any outputs, " +
		"so these rates are measured now; the grader column needs a live run.\n\n")
	b.WriteString("| Planted error | Planted | Meant for | Caught by that check | Caught by the grader |\n" +
		"| --- | --- | --- | --- | --- |\n")
	for _, r := range in.Planted {
		check, caught := r.Check, pct(r.CaughtByCheck, r.Planted)
		if check == "" {
			check, caught = "grader only", "n/a"
		}
		grader := Pending
		if r.CaughtByGrade != NotRun && in.Live {
			grader = pct(r.CaughtByGrade, r.Planted)
		} else if r.CaughtByGrade != NotRun {
			grader = pct(r.CaughtByGrade, r.Planted) + " (fakegen grader: harness only)"
		}
		fmt.Fprintf(b, "| %s | %d | %s | %s | %s |\n", r.Kind, r.Planted, check, caught, grader)
	}
	b.WriteString("\n")
}

func renderOperator(b *strings.Builder, in Input) {
	b.WriteString("## Operator sample and grader agreement\n\n")
	fmt.Fprintf(b, "Sample prepared: %d statements (`test/accuracy/operator-sample.md` and `.json`). ", in.SampleSize)
	if in.Agreement == nil || in.Agreement.Compared == 0 {
		b.WriteString("Grader-to-operator agreement: " + Pending + ".\n\n")
		return
	}
	a := in.Agreement
	fmt.Fprintf(b, "Agreement %s; contradicted statements the grader missed: %d; false alarms: %d.\n\n",
		pct(a.Agreed, a.Compared), a.Missed, a.FalseAlarm)
}

func renderCRS(b *strings.Builder, in Input) {
	b.WriteString("## CRS summary coverage (bill summaries)\n\n")
	if len(in.CRS) == 0 {
		b.WriteString("Coverage: " + Pending +
			". CRS summaries are references only and never reach the generating model.\n\n")
		return
	}
	b.WriteString("| Version | CRS points | Covered | Contradicted |\n| --- | --- | --- | --- |\n")
	for _, r := range in.CRS {
		fmt.Fprintf(b, "| %s | %d | %s | %d |\n", r.Version, r.Points, pct(r.Covered, r.Points), r.Contradicts)
	}
	b.WriteString("\n")
}

func renderTarget(b *strings.Builder, in Input) {
	b.WriteString("## Proposed M3 target\n\n")
	if !in.Live {
		b.WriteString("Pending: a target needs the live numbers above. Proposed shape, to be filled from them: ")
		b.WriteString("zero published statements marked contradicted by the grader; " +
			"deterministic checks catching every planted number, amendment and wording error (measured above); ")
		b.WriteString("grader-to-operator agreement and CRS coverage targets set from the first live run.\n")
		return
	}
	b.WriteString("Set from the measured shares above after operator review.\n")
}

// renderLiveRun lists what the pending numbers need and what the run costs
// (estimate at list prices of 2026-11, Message Batches at half).
func renderLiveRun(b *strings.Builder) {
	b.WriteString("\n## Live run (stage L3): blocked on operator approval\n\n")
	b.WriteString("Needs: the operator's key and approval; `job-summarize --generator claude` on the " +
		"three exit bills, ")
	b.WriteString("then `accuracy-report --generator claude --grade --grade-planted --crs`, then the " +
		"operator grades ")
	b.WriteString("the sample and the report is rerun with `--agreement`.\n\n")
	b.WriteString("Estimate (3.5 characters per token, about 3,000 output tokens per task including " +
		"thinking at effort medium):\n\n")
	b.WriteString("| Step | Calls | Input tokens | Output tokens | Cost at list | Cost with batches at half |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- |\n")
	b.WriteString("| Generate (`claude-opus-5-5`, $4 / $20 per million) | 86 | about 258,000 uncached | " +
		"about 258,000 | about $6.2 | about $3.1 |\n")
	b.WriteString("| Grade the 86 outputs (`claude-sonnet-5-5`, $2 / $10) | 86 | about 172,000 | about " +
		"129,000 | about $1.6 | about $0.8 |\n")
	b.WriteString("| Grade planted errors (6 kinds, about 20 outputs each) | 120 | about 240,000 | " +
		"about 180,000 | about $2.3 | about $1.2 |\n")
	b.WriteString("| CRS coverage | 3 | about 6,000 | about 4,500 | under $0.1 | under $0.1 |\n")
	b.WriteString("| Total | 295 | about 676,000 | about 571,500 | about $10 | about $5 |\n\n")
	b.WriteString("Per exit bill generation (input tokens uncached / tasks): us-119-hr-4313 about 44,600 / 20; ")
	b.WriteString("us-119-hr-3062 about 189,000 / 53; us-119-hr-3427 about 24,000 / 13. Prompt caching " +
		"of the shared bill text ")
	b.WriteString("lowers the input share further. Output tokens dominate and are the uncertain part: " +
		"twice the assumed ")
	b.WriteString("thinking doubles the output lines.\n")
}
