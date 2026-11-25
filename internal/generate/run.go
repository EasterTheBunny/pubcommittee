package generate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// Deps are what the two phases need.
type Deps struct {
	Ports data.PlatformPorts
	Gen   data.Generator
	Cfg   Config
	Now   func() time.Time
	Log   *slog.Logger
	// Retry makes Submit plan again the tasks whose stored output is not ready (rejected by validation, refused,
	// truncated or invalid). Collect always stores a ready result for a key whose stored output is not ready as
	// the revised record of the key, so a paid retry is never lost. A ready output is never replaced.
	Retry bool
	// Record reports one finished generation to telemetry; may be nil.
	Record func(ctx context.Context, task, model, status string, tokensIn, tokensOut int)
}

// SubmitReport says what one submit run did for one bill.
type SubmitReport struct {
	Bill      string   `json:"bill"`
	Planned   int      `json:"planned"`
	Skipped   int      `json:"skipped"` // already stored or already in a pending batch
	Submitted int      `json:"submitted"`
	BatchID   string   `json:"batch_id,omitempty"`
	InputChar int      `json:"input_chars"` // characters of source and request of the submitted tasks
	Notes     []string `json:"notes,omitempty"`
}

// Submit plans every task of every stored version of the bill, drops the
// keys that already have a stored output or sit in a pending batch, and
// submits the rest as one batch. A re-run submits nothing new. With dryRun
// it plans and counts and submits nothing.
func Submit(ctx context.Context, d Deps, bill data.BillRef, dryRun bool) (SubmitReport, error) {
	rep := SubmitReport{Bill: bill.String()}
	todo, err := d.plan(ctx, &rep, bill)
	if err != nil {
		return rep, err
	}
	if note, missing := missingCitations(rep.Notes); missing {
		if dryRun {
			rep.Notes = append(rep.Notes, refuseNote)
			return rep, nil
		}
		return rep, fmt.Errorf("%s: %w: %s", bill, ErrNoCitations, note)
	}
	if len(todo) == 0 || dryRun {
		return rep, nil
	}
	return rep, d.submit(ctx, &rep, todo)
}

// ErrNoCitations means a version of the bill has no citation set, so its summary
// would fail the provision check and its explanations would not be planned.
var ErrNoCitations = errors.New("no citation set: run job-ingest --law, then --analyze, before submitting")

// refuseNote is the dry-run report's note for a bill Submit would refuse.
const refuseNote = "a submit would refuse this bill: no citation set (run job-ingest --law, then --analyze)"

// missingCitations returns the first note of a version planned without its citation set.
func missingCitations(notes []string) (string, bool) {
	for _, n := range notes {
		if strings.HasPrefix(n, noteNoCitations) {
			return n, true
		}
	}
	return "", false
}

// PlanAll returns every task of every stored version of the bill, stored or
// not, in version order, with the notes of steps that planned nothing.
func PlanAll(ctx context.Context, d Deps, bill data.BillRef) ([]Task, []string, error) {
	vs, err := d.Ports.BillText.ListVersions(ctx, bill)
	if err != nil {
		return nil, nil, fmt.Errorf("list versions %s: %w", bill, err)
	}
	var all []Task
	var notes []string
	var prev *data.OfficialVersion
	for _, s := range vs {
		ver, err := d.Ports.BillText.GetVersion(ctx, s.Ref)
		if err != nil {
			return nil, nil, fmt.Errorf("get version %s: %w", s.Ref, err)
		}
		tasks, n, err := PlanVersion(ctx, d.Ports, ver, prev, d.Cfg)
		if err != nil {
			return nil, nil, err
		}
		all, notes, prev = append(all, tasks...), append(notes, n...), &ver
	}
	return all, notes, nil
}

// plan returns the tasks of the bill that have no stored output and are not
// in a pending batch, and counts the rest in rep.
func (d Deps) plan(ctx context.Context, rep *SubmitReport, bill data.BillRef) ([]Task, error) {
	all, notes, err := PlanAll(ctx, d, bill)
	if err != nil {
		return nil, err
	}
	rep.Notes = append(rep.Notes, notes...)
	busy, err := pendingKeys(ctx, d.Ports.GeneratedStore)
	if err != nil {
		return nil, err
	}
	var todo []Task
	for _, t := range all {
		rep.Planned++
		stored, err := d.isDone(ctx, t.Key)
		if err != nil {
			return nil, err
		}
		if stored || busy[t.Key.String()] {
			rep.Skipped++
			continue
		}
		todo = append(todo, t)
		rep.InputChar += len(t.Req.Source) + len(t.Req.Input) + len(t.Req.Instructions)
	}
	return todo, nil
}

func (d Deps) submit(ctx context.Context, rep *SubmitReport, todo []Task) error {
	reqs := make([]data.GenRequest, len(todo))
	batch := data.GenBatch{SubmittedAt: d.Now().UTC()}
	for i, t := range todo {
		reqs[i] = t.Req
		batch.Items = append(batch.Items, data.GenBatchItem{Key: t.Key, RequestID: t.Req.ID, Context: t.Ctx})
	}
	id, err := d.Gen.Submit(ctx, reqs)
	if err != nil {
		return fmt.Errorf("submit batch: %w", err)
	}
	batch.ID = id
	if err := d.Ports.GeneratedStore.SaveBatch(ctx, batch); err != nil {
		return fmt.Errorf("save batch %s: %w", id, err)
	}
	rep.BatchID, rep.Submitted = id, len(todo)
	rec := data.GenerationRecord{
		ID: "submit-" + id, Task: "batch", Model: d.Cfg.Model, Outcome: "submitted", At: d.Now().UTC(),
		Meta: autoMeta(d.Now()),
	}
	if err := d.Ports.PlatformDocs.PutGeneration(ctx, rec); err != nil && !errors.Is(err, data.ErrConflict) {
		return fmt.Errorf("record submission %s: %w", id, err)
	}
	return nil
}

func pendingKeys(ctx context.Context, w data.GeneratedWriter) (map[string]bool, error) {
	out := map[string]bool{}
	if w == nil {
		return out, nil
	}
	bs, err := w.PendingBatches(ctx)
	if err != nil {
		return nil, fmt.Errorf("pending batches: %w", err)
	}
	for _, b := range bs {
		for _, it := range b.Items {
			out[it.Key.String()] = true
		}
	}
	return out, nil
}

// isDone reports whether the key needs no new generation: it has a stored output, except under Retry a stored
// output that is not ready.
func (d Deps) isDone(ctx context.Context, k data.GenKey) (bool, error) {
	stored, err := isStored(ctx, d.Ports.Generated, k)
	if err != nil || !stored || !d.Retry {
		return stored, err
	}
	old, err := d.Ports.Generated.GetGenerated(ctx, k)
	if err != nil {
		return false, fmt.Errorf("get generated %s: %w", k, err)
	}
	return old.Status == data.GenReady, nil
}

func isStored(ctx context.Context, r data.GeneratedReader, k data.GenKey) (bool, error) {
	if r == nil {
		return false, nil
	}
	_, err := r.GetGenerated(ctx, k)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, data.ErrNotFound):
		return false, nil
	}
	return false, fmt.Errorf("get generated %s: %w", k, err)
}

func autoMeta(at time.Time) data.Meta {
	return data.Meta{Version: 1, AsOf: at.UTC(), Provenance: data.ProvenanceAuto, ContentClass: data.ContentPlatform}
}

// CollectReport counts what one collect run did.
type CollectReport struct {
	Batches  int            `json:"batches"`   // batches finished this run
	Waiting  int            `json:"waiting"`   // batches still processing
	ByStatus map[string]int `json:"by_status"` // outputs stored, by status
	Errors   int            `json:"errors"`    // results with outcome error: not stored, planned again by the next submit
}

// Collect gathers every pending batch that has finished: it validates each
// result (schema, anchors, stop reason, fact fidelity, amendment consistency,
// neutral wording), stores the output write-once whatever its status, and
// publishes the ready ones. Refused, truncated, invalid and unverified
// outputs are stored with their status and never published.
func Collect(ctx context.Context, d Deps) (CollectReport, error) {
	rep := CollectReport{ByStatus: map[string]int{}}
	pending, err := d.Ports.GeneratedStore.PendingBatches(ctx)
	if err != nil {
		return rep, fmt.Errorf("pending batches: %w", err)
	}
	for _, b := range pending {
		done, err := d.collectBatch(ctx, b, &rep)
		if err != nil {
			return rep, err
		}
		if !done {
			rep.Waiting++
			continue
		}
		rep.Batches++
	}
	return rep, nil
}

// collectBatch finishes one batch if the generator says it is done.
func (d Deps) collectBatch(ctx context.Context, b data.GenBatch, rep *CollectReport) (bool, error) {
	results, done, err := d.Gen.Collect(ctx, b.ID)
	if err != nil {
		return false, fmt.Errorf("collect batch %s: %w", b.ID, err)
	}
	if !done {
		return false, nil
	}
	byID := make(map[string]data.GenResult, len(results))
	for _, r := range results {
		byID[r.ID] = r
	}
	for _, it := range b.Items {
		r, ok := byID[it.RequestID]
		if !ok {
			r = data.GenResult{ID: it.RequestID, Outcome: "error"}
		}
		status, err := d.finish(ctx, it, r)
		if err != nil {
			return false, err
		}
		if status == "" {
			rep.Errors++
			continue
		}
		rep.ByStatus[status]++
	}
	b.Done = true
	if err := d.Ports.GeneratedStore.SaveBatch(ctx, b); err != nil {
		return false, fmt.Errorf("save batch %s: %w", b.ID, err)
	}
	return true, nil
}

// judge maps a result and its validation to the status it is stored with;
// "" means a transient error result.
func (d Deps) judge(it data.GenBatchItem, r data.GenResult) (status string, out ModelOutput, findings []string) {
	switch r.Outcome {
	case "refusal":
		return data.GenRefusal, ModelOutput{}, []string{"stop reason " + r.Stop}
	case "truncated":
		return data.GenTruncated, ModelOutput{}, []string{"stop reason " + r.Stop}
	case "invalid":
		return data.GenInvalid, ModelOutput{}, []string{"the adapter could not read the output"}
	case "ok":
	default:
		return "", ModelOutput{}, nil
	}
	out, err := ParseOutput(r.Text)
	if err != nil {
		return data.GenInvalid, ModelOutput{}, []string{err.Error()}
	}
	findings = Validate(it.Key.Task, out, it.Context)
	if len(findings) > 0 {
		return data.GenUnverified, out, findings
	}
	return data.GenReady, out, nil
}

// finish stores and publishes one result; it returns the stored status, or
// "" for a transient error result that is not stored.
func (d Deps) finish(ctx context.Context, it data.GenBatchItem, r data.GenResult) (string, error) {
	status, out, findings := d.judge(it, r)
	if status == "" {
		d.Log.Warn("generation error result", "key", it.Key.String())
		return "", nil
	}
	rec := data.GeneratedOutput{
		Key: it.Key, Status: status, Findings: findings, Headline: out.Headline, Statements: out.Statements,
		Section: it.Context.Section, ProvisionID: it.Context.ProvisionID,
		CurrentText: it.Context.Provisions[it.Context.ProvisionID], ReviewState: data.ReviewUnreviewed,
		TokensIn: r.TokensIn, TokensOut: r.TokensOut, Meta: autoMeta(d.Now()),
	}
	if rec.Statements == nil {
		rec.Statements = []data.Statement{}
	}
	if r.FallbackFrom != "" {
		rec.FallbackModel = r.Model
	}
	rec, err := d.keep(ctx, rec)
	if err != nil {
		return "", err
	}
	if rec.Status == data.GenReady {
		if err := d.publish(ctx, rec); err != nil {
			return "", err
		}
	}
	g := data.GenerationRecord{
		ID: "gen-" + it.Key.ID(), Task: it.Key.Task, Model: it.Key.Model, PromptHash: it.Key.PromptVersion,
		Outcome: rec.Status, At: rec.Meta.AsOf, TokensIn: r.TokensIn, TokensOut: r.TokensOut, Meta: rec.Meta,
	}
	if err := d.Ports.PlatformDocs.PutGeneration(ctx, g); err != nil && !errors.Is(err, data.ErrConflict) {
		return "", fmt.Errorf("record generation %s: %w", it.Key, err)
	}
	if d.Record != nil {
		d.Record(ctx, it.Key.Task, it.Key.Model, rec.Status, r.TokensIn, r.TokensOut)
	}
	return rec.Status, nil
}

// keep stores the output write-once; when a crashed earlier collect already
// stored one, the stored one wins, unless it is not ready and the new one is (a retry).
func (d Deps) keep(ctx context.Context, rec data.GeneratedOutput) (data.GeneratedOutput, error) {
	err := d.Ports.GeneratedStore.PutGenerated(ctx, rec)
	if err == nil {
		return rec, nil
	}
	if !errors.Is(err, data.ErrConflict) {
		return rec, fmt.Errorf("store %s: %w", rec.Key, err)
	}
	old, gerr := d.Ports.Generated.GetGenerated(ctx, rec.Key)
	if gerr != nil {
		return rec, fmt.Errorf("stored %s: %w", rec.Key, gerr)
	}
	if old.Status != data.GenReady && rec.Status == data.GenReady {
		if err := d.Ports.GeneratedStore.PutRevised(ctx, rec); err != nil {
			return rec, fmt.Errorf("revise %s: %w", rec.Key, err)
		}
		return rec, nil
	}
	return old, nil
}

// publish makes a ready output visible: a summary or change summary through
// the current pointer, an explanation as a change explanation document.
func (d Deps) publish(ctx context.Context, o data.GeneratedOutput) error {
	switch o.Key.Task {
	case data.TaskBillSummary, data.TaskVersionChanges:
		if err := d.Ports.GeneratedStore.SetCurrent(ctx, o); err != nil {
			return fmt.Errorf("set current %s: %w", o.Key, err)
		}
		return nil
	}
	flags := []data.Provenance{data.ProvenanceAuto}
	if o.Key.Task == data.TaskBillExplanation {
		flags = append(flags, data.ProvenanceBillOnly)
	}
	ex := data.ChangeExplanation{
		Version: o.Key.Version, Section: o.Section, ProvisionID: o.ProvisionID, CurrentText: o.CurrentText,
		Anchors:     changeAnchors(o.Statements),
		WhatChanges: joinRole(o.Statements, data.RoleChanges), WhyMatters: joinRole(o.Statements, data.RoleWhy),
		Flags: flags, Meta: o.Meta,
	}
	log := data.LogEntry{
		ID: "gen-" + o.Key.ID(), At: o.Meta.AsOf, Kind: "generation", Subject: o.Key.Version.String() + "#" + o.Key.Target,
		Reason: "auto-generated explanation stored (unreviewed)", Meta: o.Meta,
	}
	if err := d.Ports.PlatformDocs.ApplyLogged(ctx, data.Change{Explanation: &ex}, log); err != nil {
		return fmt.Errorf("apply explanation %s: %w", o.Key, err)
	}
	return nil
}

func joinRole(ss []data.Statement, role string) string {
	var parts []string
	for _, s := range ss {
		if s.Role == role {
			parts = append(parts, s.Text)
		}
	}
	return strings.Join(parts, " ")
}

// changeAnchors lists the anchors of the statements that say what the bill changes, each once, in order; the
// anchors of every statement when none has that role.
func changeAnchors(ss []data.Statement) []string {
	var out []string
	for _, role := range []string{data.RoleChanges, ""} {
		for _, s := range ss {
			if role != "" && s.Role != role {
				continue
			}
			for _, a := range s.Anchors {
				if !slices.Contains(out, a) {
					out = append(out, a)
				}
			}
		}
		if len(out) > 0 {
			break
		}
	}
	return out
}
