package generate

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// RevalidateReport says what one revalidation run did for one bill.
type RevalidateReport struct {
	Bill      string `json:"bill"`
	Checked   int    `json:"checked"`   // stored outputs that have statements to check (ready or unverified)
	Unchanged int    `json:"unchanged"` // same status and findings as stored
	// Moves counts the outputs whose status changed, keyed "from->to" (for example "unverified->ready").
	Moves map[string]int `json:"moves,omitempty"`
	// Refound counts the outputs that stayed unverified with different findings.
	Refound int `json:"refound"`
	// Changed lists the stored key of every output whose status or findings changed.
	Changed []string `json:"changed,omitempty"`
}

// Revalidate repeats the deterministic checks on the stored outputs of the bill with the current validators and no
// model call. The context of each output is planned again from the stored bill text and citation set, so no
// regeneration is involved. An output whose status or findings differ is stored as a revised record beside the
// write-once one (data.GeneratedWriter.PutRevised), and a newly ready output is published like a collected one.
// Outputs that are not ready or unverified (refused, truncated, invalid) have no statements and are left alone.
func Revalidate(ctx context.Context, d Deps, bill data.BillRef) (RevalidateReport, error) {
	rep := RevalidateReport{Bill: bill.String(), Moves: map[string]int{}}
	tasks, _, err := PlanAll(ctx, d, bill)
	if err != nil {
		return rep, err
	}
	for _, t := range tasks {
		old, err := d.Ports.Generated.GetGenerated(ctx, t.Key)
		if errors.Is(err, data.ErrNotFound) {
			continue
		}
		if err != nil {
			return rep, fmt.Errorf("get %s: %w", t.Key, err)
		}
		if old.Status != data.GenReady && old.Status != data.GenUnverified {
			continue
		}
		rep.Checked++
		if err := d.revalidate(ctx, &rep, t, old); err != nil {
			return rep, err
		}
	}
	return rep, nil
}

func (d Deps) revalidate(ctx context.Context, rep *RevalidateReport, t Task, old data.GeneratedOutput) error {
	findings := Validate(t.Key.Task, ModelOutput{Headline: old.Headline, Statements: old.Statements}, t.Ctx)
	status := data.GenReady
	if len(findings) > 0 {
		status = data.GenUnverified
	}
	if status == old.Status && slices.Equal(findings, old.Findings) {
		rep.Unchanged++
		return nil
	}
	if status == old.Status {
		rep.Refound++
	} else {
		rep.Moves[old.Status+"->"+status]++
	}
	rep.Changed = append(rep.Changed, t.Key.String())
	rec := old
	rec.Status, rec.Findings = status, findings
	if err := d.Ports.GeneratedStore.PutRevised(ctx, rec); err != nil {
		return fmt.Errorf("revise %s: %w", t.Key, err)
	}
	if status == data.GenReady {
		return d.publish(ctx, rec)
	}
	return nil
}
