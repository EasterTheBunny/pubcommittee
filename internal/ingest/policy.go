package ingest

import (
	"context"
	"slices"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/selection"
)

// ReasonManual is the tracked reason of a bill an operator pulled with --track.
// Whether the string is shown publicly is an open question for the operator.
const ReasonManual = selection.ReasonManual

// Decision is the outcome of selection policy for one bill.
type Decision struct {
	Tracked         bool
	Reasons         []string
	PolicyVersion   int
	TrackedSince    string // YYYY-MM-DD the bill was first tracked; empty when untracked or unknown
	MarkupStatus    string
	ExclusionReason string
}

// Evaluator decides whether a bill is tracked. PolicyEvaluator (selection.go)
// applies the published selection policy; ManualEvaluator is the stand-in used
// when no policy is stored.
type Evaluator interface {
	Evaluate(ctx context.Context, r *Run) (Decision, error)
}

// ManualEvaluator tracks a bill that is already tracked (it keeps its reasons
// and policy version; policy never untracks) or that the operator asked to
// track. Everything else is a catalog entry only. Measure kinds follow the
// published policy shape: bills and joint resolutions are eligible, other
// kinds are not candidates.
type ManualEvaluator struct{}

// Evaluate implements Evaluator.
func (ManualEvaluator) Evaluate(_ context.Context, r *Run) (Decision, error) {
	var d Decision
	switch {
	case r.Existing != nil:
		d.Tracked = true
		d.Reasons = slices.Clone(r.Existing.TrackedReasons)
		d.PolicyVersion = r.Existing.PolicyVersion
	case r.Track:
		d.Tracked = true
		d.Reasons = []string{ReasonManual}
	}
	switch {
	case d.Tracked:
		d.MarkupStatus = data.MarkupInReview
	case eligibleKind(r.Bill.Type):
		d.MarkupStatus = data.MarkupEligible
	default:
		d.MarkupStatus = data.MarkupNotACandidate
		d.ExclusionReason = "measure kind " + r.Bill.Type + " is not a bill or joint resolution"
	}
	return d, nil
}

func eligibleKind(t string) bool { return t == "hr" || t == "s" || t == "hjres" || t == "sjres" }

// PolicyStep runs an Evaluator and records its decision on the run.
func PolicyStep(e Evaluator) Step {
	return StepFunc{StepName: StepPolicy, Fn: func(ctx context.Context, r *Run) error {
		d, err := e.Evaluate(ctx, r)
		if err != nil {
			return err //nolint:wrapcheck // Pull names the step
		}
		r.Decision = d
		r.Entry.Tracked = d.Tracked
		r.Entry.Reasons = nonNil(d.Reasons)
		r.Entry.PolicyVersion = d.PolicyVersion
		r.Entry.TrackedSince = d.TrackedSince
		r.Entry.MarkupStatus = d.MarkupStatus
		r.Entry.ExclusionReason = d.ExclusionReason
		r.Report.Tracked = d.Tracked
		return nil
	}}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
