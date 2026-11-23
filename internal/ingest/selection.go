package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/selection"
)

const dateLayout = "2006-01-02"

// Catalog is a snapshot of the stored catalog by bill, used to know which
// bills are already tracked (a tracked bill stays tracked) and for companion
// following.
type Catalog map[data.BillRef]data.CatalogEntry

// LoadCatalog reads every stored catalog entry.
func LoadCatalog(ctx context.Context, docs data.PublicDocs) (Catalog, error) {
	out := Catalog{}
	cursor := ""
	for {
		page, err := docs.ListCatalog(ctx, data.Page{Cursor: cursor})
		if err != nil {
			return nil, fmt.Errorf("list catalog: %w", err)
		}
		for _, e := range page.Items {
			out[e.Bill] = e
		}
		if page.Next == "" {
			return out, nil
		}
		cursor = page.Next
	}
}

func (c Catalog) tracked(b data.BillRef) bool { return c[b].Tracked }

// entries returns the catalog's entries ordered by bill key.
func (c Catalog) entries() []data.CatalogEntry {
	out := make([]data.CatalogEntry, 0, len(c))
	for _, e := range c {
		out = append(out, e)
	}
	slices.SortFunc(out, func(a, b data.CatalogEntry) int { return compareBill(a.Bill, b.Bill) })
	return out
}

func compareBill(a, b data.BillRef) int {
	switch {
	case a.Session != b.Session:
		return compareStr(a.Session, b.Session)
	case a.Type != b.Type:
		return compareStr(a.Type, b.Type)
	}
	return a.Number - b.Number
}

func compareStr(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// PolicyEvaluator applies a selection policy version to a pull. Catalog is
// the stored catalog before the pull; nil means nothing is stored.
type PolicyEvaluator struct {
	Policy  data.SelectionPolicy
	Catalog Catalog
}

// Evaluate implements Evaluator. A bill that is already tracked (a stored bill
// document or catalog entry) keeps its reasons, policy version and since date.
// With Run.Track, a bill the policy does not select is tracked with the
// manual reason, unless its measure kind is excluded.
func (e PolicyEvaluator) Evaluate(_ context.Context, r *Run) (Decision, error) {
	stored := e.Catalog[r.Bill]
	var prior *selection.Prior
	switch {
	case r.Existing != nil:
		prior = &selection.Prior{Reasons: slices.Clone(r.Existing.TrackedReasons), PolicyVersion: r.Existing.PolicyVersion}
	case stored.Tracked:
		prior = &selection.Prior{Reasons: slices.Clone(stored.Reasons), PolicyVersion: stored.PolicyVersion}
	}
	res := selection.Evaluate(e.Policy, selection.FromCatalog(r.Entry), prior, e.Catalog.tracked)
	if r.Track && prior == nil {
		switch res.MarkupStatus {
		case data.MarkupNotACandidate:
			r.Log.Warn("--track ignored: excluded measure", "bill", r.Bill.ID(), "reason", res.ExclusionReason)
		default:
			res.Tracked = true
			res.Reasons = append(slices.Clone(res.Reasons), ReasonManual)
			res.PolicyVersion = e.Policy.Version
			res.MarkupStatus = data.MarkupInReview
		}
	}
	d := Decision{
		Tracked: res.Tracked, Reasons: res.Reasons, PolicyVersion: res.PolicyVersion,
		MarkupStatus: res.MarkupStatus, ExclusionReason: res.ExclusionReason,
	}
	if res.Tracked {
		d.TrackedSince = stored.TrackedSince
		if d.TrackedSince == "" {
			d.TrackedSince = r.Now().UTC().Format(dateLayout)
		}
	}
	return d, nil
}

// EvalReport is the output of EvaluateAll.
type EvalReport struct {
	PolicyVersion int                 `json:"policy_version"`
	DryRun        bool                `json:"dry_run"`
	Evaluated     int                 `json:"evaluated"`
	Changed       []string            `json:"changed"`
	NewlyTracked  []string            `json:"newly_tracked"` // pull these for their text: job-ingest --bill <id>
	Breakdown     selection.Breakdown `json:"breakdown"`
	Failures      []Failure           `json:"failures,omitempty"`
}

// EvaluateAll re-evaluates every stored catalog entry against policy p and
// writes the entries (and the bill documents of tracked bills) that change,
// then writes the tracked-set breakdown. Tracked bills are never untracked.
// A bill that becomes tracked is not pulled: pull its text with job-ingest --bill.
func EvaluateAll(ctx context.Context, d Deps, p data.SelectionPolicy, dryRun bool) (EvalReport, error) {
	if d.Now == nil {
		d.Now = time.Now
	}
	cat, err := LoadCatalog(ctx, d.Ports.PublicDocs)
	if err != nil {
		return EvalReport{}, err
	}
	rep := EvalReport{PolicyVersion: p.Version, DryRun: dryRun, Changed: []string{}, NewlyTracked: []string{}}
	now := d.Now().UTC()
	wasTracked := map[data.BillRef]bool{}
	for b, e := range cat {
		wasTracked[b] = e.Tracked
	}
	changed := sweep(p, cat, now)
	rep.Evaluated = len(cat)
	rep.applyChanges(ctx, d, cat, changed, wasTracked, dryRun)
	rep.Breakdown = selection.Compute(cat.entries(), p.Version, now)
	if !dryRun {
		if err := putBreakdown(ctx, d.Ports.Publisher, rep.Breakdown); err != nil {
			return rep, err
		}
	}
	return rep, nil
}

// sweep evaluates every entry of cat in place and returns the bills that
// changed. Companions depend on other bills' state, so it repeats until a
// pass changes nothing.
func sweep(p data.SelectionPolicy, cat Catalog, now time.Time) map[data.BillRef]bool {
	changed := map[data.BillRef]bool{}
	for pass := 0; pass <= len(cat); pass++ {
		moved := false
		for _, e := range cat.entries() {
			next := evaluateEntry(p, e, cat, now)
			if !sameDecision(e, next) {
				cat[e.Bill] = next
				changed[e.Bill] = true
				moved = true
			}
		}
		if !moved {
			break
		}
	}
	return changed
}

// writeChanged stores a changed catalog entry and refreshes the bill document.
func writeChanged(ctx context.Context, d Deps, e data.CatalogEntry) error {
	if err := d.Ports.PlatformDocs.PutCatalogEntry(ctx, e); err != nil {
		return fmt.Errorf("put catalog entry %s: %w", e.Bill.ID(), err)
	}
	return refreshBillDoc(ctx, d, e)
}

// evaluateEntry returns e with the evaluation of policy p applied.
func evaluateEntry(p data.SelectionPolicy, e data.CatalogEntry, cat Catalog, now time.Time) data.CatalogEntry {
	var prior *selection.Prior
	if e.Tracked {
		prior = &selection.Prior{Reasons: e.Reasons, PolicyVersion: e.PolicyVersion}
	}
	res := selection.Evaluate(p, selection.FromCatalog(e), prior, cat.tracked)
	next := e
	next.Tracked = res.Tracked
	next.Reasons = nonNil(res.Reasons)
	next.PolicyVersion = res.PolicyVersion
	next.MarkupStatus = res.MarkupStatus
	next.ExclusionReason = res.ExclusionReason
	if res.Tracked && next.TrackedSince == "" {
		next.TrackedSince = now.Format(dateLayout)
	}
	if !sameDecision(e, next) {
		next.Meta.Version++
		next.Meta.AsOf = now
	}
	return next
}

func sameDecision(a, b data.CatalogEntry) bool {
	return a.Tracked == b.Tracked && a.PolicyVersion == b.PolicyVersion && a.MarkupStatus == b.MarkupStatus &&
		a.ExclusionReason == b.ExclusionReason && slices.Equal(a.Reasons, b.Reasons)
}

// refreshBillDoc brings the reasons on a stored bill document in line with its
// catalog entry (a title-only match confirmed by subjects, for example).
func refreshBillDoc(ctx context.Context, d Deps, e data.CatalogEntry) error {
	if !e.Tracked {
		return nil
	}
	b, err := d.Ports.PublicDocs.GetBill(ctx, e.Bill)
	if err != nil {
		return nil //nolint:nilerr // no bill document yet: the text pull writes it
	}
	if slices.Equal(b.TrackedReasons, e.Reasons) && b.PolicyVersion == e.PolicyVersion {
		return nil
	}
	b.TrackedReasons, b.PolicyVersion = slices.Clone(e.Reasons), e.PolicyVersion
	b.Meta.Version++
	if err := d.Ports.PlatformDocs.PutBill(ctx, b); err != nil {
		return fmt.Errorf("put bill: %w", err)
	}
	return nil
}

// PublishBreakdown reads the stored catalog and writes the tracked-set
// breakdown to selection.BreakdownPath.
func PublishBreakdown(ctx context.Context, d Deps, policyVersion int) (selection.Breakdown, error) {
	if d.Now == nil {
		d.Now = time.Now
	}
	cat, err := LoadCatalog(ctx, d.Ports.PublicDocs)
	if err != nil {
		return selection.Breakdown{}, err
	}
	b := selection.Compute(cat.entries(), policyVersion, d.Now())
	return b, putBreakdown(ctx, d.Ports.Publisher, b)
}

func putBreakdown(ctx context.Context, pub data.Publisher, b selection.Breakdown) error {
	raw, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("encode breakdown: %w", err)
	}
	if _, err := pub.PutReplaceable(ctx, selection.BreakdownPath, append(raw, '\n')); err != nil {
		return fmt.Errorf("publish breakdown: %w", err)
	}
	return nil
}

// applyChanges lists the changed bills in the report and, unless dryRun,
// writes them.
func (rep *EvalReport) applyChanges(
	ctx context.Context, d Deps, cat Catalog, changed, wasTracked map[data.BillRef]bool, dryRun bool,
) {
	for _, e := range cat.entries() {
		if !changed[e.Bill] {
			continue
		}
		rep.Changed = append(rep.Changed, e.Bill.ID())
		if e.Tracked && !wasTracked[e.Bill] {
			rep.NewlyTracked = append(rep.NewlyTracked, e.Bill.ID())
		}
		if dryRun {
			continue
		}
		if err := writeChanged(ctx, d, e); err != nil {
			rep.Failures = append(rep.Failures, Failure{Item: e.Bill.ID(), Error: shortErr(err)})
		}
	}
}
