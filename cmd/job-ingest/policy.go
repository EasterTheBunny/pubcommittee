package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/ingest"
	"github.com/easterthebunny/pubcommittee/internal/selection"
)

var (
	errNoPolicy      = errors.New("no selection policy is stored; run with --publish-policy v1 first")
	errPublishPolicy = errors.New(
		"--publish-policy is not available on this target: publishing a policy version is an operator action")
)

// policyV1 names the built-in initial policy for --publish-policy.
const policyV1 = "v1"

// evaluateCatalog runs the latest policy over the stored catalog.
func evaluateCatalog(ctx context.Context, d ingest.Deps, opts ingest.Options, out io.Writer) (int, error) {
	p, err := d.Ports.PublicDocs.GetSelectionPolicy(ctx)
	if errors.Is(err, data.ErrNotFound) || (err == nil && p.Mode == "") {
		if opts.DryRun {
			// Nothing to evaluate and nothing changes: a dry run reports that
			// and succeeds, so it works as the deploy smoke test everywhere.
			return 0, encode(out, noPolicyReport{DryRun: true, Notes: []string{errNoPolicy.Error()}})
		}
		return 0, errNoPolicy
	}
	if err != nil {
		return 0, fmt.Errorf("read selection policy: %w", err)
	}
	rep, err := ingest.EvaluateAll(ctx, d, p, opts.DryRun)
	if encErr := encode(out, rep); encErr != nil {
		return len(rep.Changed), encErr
	}
	if err != nil {
		return len(rep.Changed), fmt.Errorf("evaluate: %w", err)
	}
	if len(rep.Failures) > 0 {
		return len(rep.Changed), errFailed
	}
	return len(rep.Changed), nil
}

// noPolicyReport is the report of an --evaluate dry run with no stored policy.
type noPolicyReport struct {
	DryRun bool     `json:"dry_run"`
	Notes  []string `json:"notes"`
}

// policyStore joins the two ports selection.Publish needs.
type policyStore struct {
	data.PublicDocs
	data.PlatformDocs
}

// publishPolicy stores one policy version, with its public log entry, in the
// local data directory. Where the cloud wiring is built it is refused.
func publishPolicy(ctx context.Context, d ingest.Deps, cfg config, out io.Writer) error {
	if !localTarget {
		return errPublishPolicy
	}
	now := d.Now()
	var p data.SelectionPolicy
	var err error
	if cfg.PublishPolicy == policyV1 {
		p, err = selection.V1(now)
	} else {
		var raw []byte
		raw, err = os.ReadFile(filepath.Clean(cfg.PublishPolicy))
		if err == nil {
			p, err = selection.Parse(raw, now)
		}
	}
	if err != nil {
		return fmt.Errorf("policy: %w", err)
	}
	store := policyStore{d.Ports.PublicDocs, d.Ports.PlatformDocs}
	if err := selection.Publish(ctx, store, p, cfg.PolicyReason, now); err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	return encode(out, map[string]any{"published_policy_version": p.Version, "scope": selection.Scope(p)})
}

func encode(out io.Writer, v any) error {
	enc := json.NewEncoder(out)
	// One line per report: Cloud Logging stores it as one structured entry.
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("report: %w", err)
	}
	return nil
}

// publishBreakdown writes the tracked-set breakdown after pulls made under a policy.
func publishBreakdown(ctx context.Context, d ingest.Deps, policy *data.SelectionPolicy, opts ingest.Options) error {
	if policy == nil || opts.DryRun {
		return nil
	}
	if _, err := ingest.PublishBreakdown(ctx, d, policy.Version); err != nil {
		return fmt.Errorf("breakdown: %w", err)
	}
	return nil
}
