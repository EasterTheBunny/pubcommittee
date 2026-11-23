package ingest

import (
	"context"
	"errors"
	"fmt"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// PullSteps returns the full pull pipeline used by every command that pulls
// bill text (job-ingest and admin-api reingest): the default steps with the
// analysis steps (citations and comparisons). With a stored structured
// selection policy the policy step is the evaluator of that policy and the
// policy is returned; without one, the manual evaluator stays, a warning says
// so, and the returned policy is nil.
func PullSteps(ctx context.Context, d Deps) (Pipeline, *data.SelectionPolicy, error) {
	steps, err := WithAnalysis(DefaultSteps())
	if err != nil {
		return nil, nil, fmt.Errorf("pipeline: %w", err)
	}
	p, err := d.Ports.PublicDocs.GetSelectionPolicy(ctx)
	if errors.Is(err, data.ErrNotFound) || (err == nil && p.Mode == "") {
		d.Log.Warn("no structured selection policy is stored; tracking only what --track names")
		return steps, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read selection policy: %w", err)
	}
	cat, err := LoadCatalog(ctx, d.Ports.PublicDocs)
	if err != nil {
		return nil, nil, fmt.Errorf("load catalog: %w", err)
	}
	steps, err = steps.Replace(StepPolicy, PolicyStep(PolicyEvaluator{Policy: p, Catalog: cat}))
	if err != nil {
		return nil, nil, fmt.Errorf("pipeline: %w", err)
	}
	return steps, &p, nil
}
