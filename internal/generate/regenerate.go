package generate

import (
	"context"
	"fmt"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// Regenerate runs one task synchronously, for a steward-requested
// regeneration (admin-api), with the same validation and storage rules as the
// batch path. It returns the stored status. The task is named by task,
// version code and target as in data.GenKey. An output already stored under
// the current prompt version and model is ErrConflict: regeneration needs a
// new prompt version or model (outputs are write-once). An unknown task is
// ErrNotFound. Until a steward program exists callers should report the
// action unavailable when no Generator is wired.
func Regenerate(ctx context.Context, d Deps, bill data.BillRef, task, version, target string) (string, error) {
	if d.Gen == nil {
		return "", fmt.Errorf("regenerate: no generator wired: %w", data.ErrUnavailable)
	}
	all, _, err := PlanAll(ctx, d, bill)
	if err != nil {
		return "", err
	}
	for _, t := range all {
		if t.Key.Task == task && t.Key.Version.Version == version && t.Key.Target == target {
			return d.regenerate(ctx, t)
		}
	}
	return "", fmt.Errorf("regenerate %s %s %s: %w", task, version, target, data.ErrNotFound)
}

func (d Deps) regenerate(ctx context.Context, t Task) (string, error) {
	stored, err := isStored(ctx, d.Ports.Generated, t.Key)
	if err != nil {
		return "", err
	}
	if stored {
		return "", fmt.Errorf("regenerate %s: already stored for this prompt version and model: %w", t.Key, data.ErrConflict)
	}
	res, err := d.Gen.Generate(ctx, t.Req)
	if err != nil {
		return "", fmt.Errorf("regenerate %s: %w", t.Key, err)
	}
	status, err := d.finish(ctx, data.GenBatchItem{Key: t.Key, RequestID: t.Req.ID, Context: t.Ctx}, res)
	if err != nil {
		return "", err
	}
	if status == "" {
		return "", fmt.Errorf("regenerate %s: model call failed: %w", t.Key, data.ErrUnavailable)
	}
	return status, nil
}
