// Package ingest pulls one bill on demand: its status into the catalog and,
// for a tracked bill, every official text version into the object store.
// A pull is a Pipeline, an ordered list of Steps over one Run. Later tasks
// extend it by inserting steps (selection policy M1-06, citations M1-07,
// version comparison M1-08) with Pipeline.After and Pipeline.Replace, never
// by editing the steps that are here.
package ingest

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// Step names of DefaultSteps, for inserting around them.
const (
	StepFetchRecord  = "fetch-record"
	StepCatalog      = "catalog"
	StepPolicy       = "policy"
	StepVersions     = "versions"
	StepAfterStore   = "after-store"
	StepWriteBill    = "write-bill"
	StepWriteCatalog = "write-catalog"
	StepIngestState  = "ingest-state"
)

// Deps are the collaborators of a pull.
type Deps struct {
	Ports   data.PlatformPorts
	Fetcher data.BillFetcher
	// Source names the ingest_state key, for example "govinfo/119".
	Source string
	Log    *slog.Logger
	Now    func() time.Time
}

// Options are the operator's choices for one pull.
type Options struct {
	// DryRun reports what would change and writes nothing; no text is fetched.
	DryRun bool
	// Verify re-fetches versions that are already stored and compares each
	// original's SHA-256 with the stored one. A mismatch is reported as a
	// failure and nothing is overwritten.
	Verify bool
	// Track marks the bill as tracked (reason "manual-pull") when no policy
	// has done so. Selection policy (M1-06) replaces the policy step.
	Track bool
}

// Report says what a pull did. It is the job's output (JSON on stdout).
type Report struct {
	Bill           string    `json:"bill"`
	DryRun         bool      `json:"dry_run"`
	Tracked        bool      `json:"tracked"`
	CatalogWritten bool      `json:"catalog_written"`
	BillWritten    bool      `json:"bill_written"`
	Stored         []string  `json:"stored"`  // version codes stored by this pull
	Present        []string  `json:"present"` // version codes already stored, untouched
	WouldStore     []string  `json:"would_store,omitempty"`
	Failures       []Failure `json:"failures,omitempty"`
}

// Failure is one version or step that did not complete.
type Failure struct {
	Item  string `json:"item"`
	Error string `json:"error"`
}

// Failed reports whether the pull must exit non-zero.
func (r Report) Failed() bool { return len(r.Failures) > 0 }

// Run is the state one pull carries through its steps.
type Run struct {
	Deps
	Options

	Bill     data.BillRef
	Record   data.BillRecord
	Existing *data.Bill        // the stored bill document, when the bill is tracked already
	Entry    data.CatalogEntry // built by the catalog step, finished by the policy step
	Decision Decision          // set by the policy step
	Stored   []data.VersionRef // versions stored by this pull; steps after the versions step read it
	Report   Report
}

// Step is one unit of a pull.
type Step interface {
	Name() string
	Do(ctx context.Context, r *Run) error
}

// StepFunc adapts a function to a Step.
type StepFunc struct {
	StepName string
	Fn       func(ctx context.Context, r *Run) error
}

// Name returns the step name.
func (s StepFunc) Name() string { return s.StepName }

// Do runs the step.
func (s StepFunc) Do(ctx context.Context, r *Run) error { return s.Fn(ctx, r) }

// Pipeline is an ordered list of steps.
type Pipeline []Step

// After returns a copy of p with step inserted after the step named name;
// name not found: appended at the end is wrong, so it panics at wiring time
// through the returned error.
func (p Pipeline) After(name string, step Step) (Pipeline, error) {
	i := slices.IndexFunc(p, func(s Step) bool { return s.Name() == name })
	if i < 0 {
		return nil, fmt.Errorf("ingest: no step %q: %w", name, data.ErrNotFound)
	}
	out := slices.Clone(p)
	return slices.Insert(out, i+1, step), nil
}

// Replace returns a copy of p with the step named name swapped for step.
func (p Pipeline) Replace(name string, step Step) (Pipeline, error) {
	i := slices.IndexFunc(p, func(s Step) bool { return s.Name() == name })
	if i < 0 {
		return nil, fmt.Errorf("ingest: no step %q: %w", name, data.ErrNotFound)
	}
	out := slices.Clone(p)
	out[i] = step
	return out, nil
}

// Pull runs the pipeline for one bill. A step error stops the pull and is
// returned with the report so far; per-version failures do not stop it and
// are listed in Report.Failures.
func Pull(ctx context.Context, d Deps, o Options, p Pipeline, bill data.BillRef) (Report, error) {
	if err := bill.Validate(); err != nil {
		return Report{}, fmt.Errorf("ingest: %w", err)
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	r := &Run{Deps: d, Options: o, Bill: bill}
	r.Report = Report{Bill: bill.ID(), DryRun: o.DryRun, Stored: []string{}, Present: []string{}}
	for _, s := range p {
		d.Log.Info("step", "bill", bill.ID(), "step", s.Name())
		if err := s.Do(ctx, r); err != nil {
			err = fmt.Errorf("ingest %s: step %s: %w", bill.ID(), s.Name(), err)
			// Keep the failed bill for the next pull and for `ingest failures`.
			r.Report.Failures = append(r.Report.Failures, Failure{Item: bill.String(), Error: shortErr(err)})
			if s.Name() != StepIngestState {
				if serr := recordState(ctx, r); serr != nil {
					d.Log.Warn("record failure", "error", serr.Error())
				}
			}
			return r.Report, err
		}
	}
	return r.Report, nil
}
