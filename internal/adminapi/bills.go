package adminapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/billxml"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/ingest"
	"github.com/easterthebunny/pubcommittee/internal/selection"
)

// Log entry kinds of administrator actions. Roster kinds are in roster.go;
// policy publication keeps the kind selection.Publish writes.
const (
	KindTracked   = "bill-tracked"
	KindUntracked = "bill-untracked"
	KindPatched   = "catalog-patch"
	KindReingest  = "bill-reingest"
)

var (
	errNoPolicy   = errors.New("no selection policy is stored; publish one first")
	errNoFetcher  = errors.New("this target has no bill source")
	errSignoff    = errors.New("needs sign-off: the steward sign-off phase decides a selection with two stewards (M3)")
	errNotInCat   = errors.New("the bill is not in the catalog")
	errNoChange   = errors.New("the patch changes nothing")
	errEmptyPatch = errors.New("the patch names no field")
	errEmptyTitle = errors.New("a bill needs a title")
)

func (s *Server) billRef(w http.ResponseWriter, r *http.Request) (data.BillRef, bool) {
	ref, err := data.ParseBillID(r.PathValue("bill"))
	if err != nil {
		s.fail(w, err)
		return data.BillRef{}, false
	}
	return ref, true
}

// bodyReasoned decodes a body that holds only a reason.
func bodyReasoned(w http.ResponseWriter, r *http.Request) (Reasoned, bool) {
	var b Reasoned
	return b, decode(w, r, &b, func() string { return b.Reason })
}

// pull runs one pull of a bill.
func (s *Server) pull(ctx context.Context, ref data.BillRef, o ingest.Options) (ingest.Report, error) {
	if s.Fetcher == nil {
		return ingest.Report{}, fmt.Errorf("%w: %w", errNoFetcher, data.ErrUnavailable)
	}
	d := ingest.Deps{
		Ports: s.Ports, Fetcher: s.Fetcher, Source: s.Source + "/" + ref.Session, Log: s.Log, Now: s.Now,
	}
	steps, _, err := ingest.PullSteps(ctx, d)
	if err != nil {
		return ingest.Report{}, fmt.Errorf("pull steps: %w", err)
	}
	rep, err := ingest.Pull(ctx, d, o, steps, ref)
	if err != nil {
		return rep, fmt.Errorf("pull %s: %w", ref.ID(), err)
	}
	return rep, nil
}

// catalogEntry returns the bill's catalog entry, pulling its status first
// when the catalog has none.
func (s *Server) catalogEntry(ctx context.Context, ref data.BillRef) (data.CatalogEntry, error) {
	cat, err := ingest.LoadCatalog(ctx, s.Ports.PublicDocs)
	if err != nil {
		return data.CatalogEntry{}, fmt.Errorf("catalog: %w", err)
	}
	if e, ok := cat[ref]; ok {
		return e, nil
	}
	if s.Fetcher == nil {
		return data.CatalogEntry{}, fmt.Errorf("%s: %w: %w", ref.ID(), errNotInCat, data.ErrNotFound)
	}
	if _, err := s.pull(ctx, ref, ingest.Options{}); err != nil {
		return data.CatalogEntry{}, err
	}
	if cat, err = ingest.LoadCatalog(ctx, s.Ports.PublicDocs); err != nil {
		return data.CatalogEntry{}, fmt.Errorf("catalog: %w", err)
	}
	e, ok := cat[ref]
	if !ok {
		return data.CatalogEntry{}, fmt.Errorf("%s: %w: %w", ref.ID(), errNotInCat, data.ErrNotFound)
	}
	return e, nil
}

// currentPolicy returns the latest structured selection policy; none stored
// is a conflict, since selection needs a policy to refuse excluded kinds.
func (s *Server) currentPolicy(ctx context.Context) (data.SelectionPolicy, error) {
	p, err := s.Ports.PublicDocs.GetSelectionPolicy(ctx)
	if errors.Is(err, data.ErrNotFound) || (err == nil && p.Mode == "") {
		return data.SelectionPolicy{}, fmt.Errorf("%w: %w", errNoPolicy, data.ErrConflict)
	}
	if err != nil {
		return data.SelectionPolicy{}, fmt.Errorf("read selection policy: %w", err)
	}
	return p, nil
}

// track selects a bill: the catalog entry becomes tracked with the
// administrator reason, atomically with its public log entry. It does not pull
// text: that is the explicit, logged reingest. A refused kind or the sign-off
// phase changes nothing.
func (s *Server) track(w http.ResponseWriter, r *http.Request) {
	ref, ok := s.billRef(w, r)
	if !ok {
		return
	}
	body, ok := bodyReasoned(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	policy, err := s.currentPolicy(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	e, err := s.catalogEntry(ctx, ref)
	if err != nil {
		s.fail(w, err)
		return
	}
	res := ActionResult{Bill: ref.ID()}
	final := e
	if !e.Tracked || !slices.Contains(e.Reasons, selection.ReasonAdministrator) {
		if final, err = s.selectBill(ctx, e, policy, body.Reason, &res); err != nil {
			s.fail(w, err)
			return
		}
	}
	if err := s.syncBillDoc(ctx, final); err != nil {
		s.fail(w, err)
		return
	}
	res.Note = "the bill is tracked; its text is not pulled yet: run `pubctl bills reingest " + ref.ID() +
		" --reason R` or `job-ingest --bill " + ref.ID() + "`"
	writeJSON(w, res)
}

// selectBill applies the refusal rules, then writes the tracked entry with
// its log entry in one step and records the log id in res.
func (s *Server) selectBill(
	ctx context.Context, e data.CatalogEntry, p data.SelectionPolicy, why string, res *ActionResult,
) (data.CatalogEntry, error) {
	if reason := selection.ExclusionReason(p, selection.FromCatalog(e)); reason != "" && !e.Tracked {
		return e, fmt.Errorf("refused by the selection policy: %s: %w", reason, data.ErrInvalid)
	}
	if p.Phase == data.PolicyPhaseStewardSignoff {
		return e, fmt.Errorf("%w: %w", errSignoff, data.ErrConflict)
	}
	next := withAdministrator(e, p, s.Now())
	le := s.entry(KindTracked, e.Bill.String(), why, operatorOf(ctx))
	if err := s.Ports.PlatformDocs.ApplyLogged(ctx, data.Change{Catalog: &next}, le); err != nil {
		return e, fmt.Errorf("track %s: %w", e.Bill.ID(), err)
	}
	res.Changed, res.LogID = true, le.ID
	return next, nil
}

// withAdministrator returns e tracked with the administrator reason added. A
// bill already tracked keeps its reasons, policy version and since date.
func withAdministrator(e data.CatalogEntry, p data.SelectionPolicy, now time.Time) data.CatalogEntry {
	e.Reasons = append(slices.Clone(e.Reasons), selection.ReasonAdministrator)
	if !e.Tracked {
		e.Tracked, e.PolicyVersion = true, p.Version
		e.Reasons = []string{selection.ReasonAdministrator}
		e.MarkupStatus = data.MarkupInReview
		e.TrackedSince = now.UTC().Format("2006-01-02")
	}
	e.Meta.Version++
	e.Meta.AsOf = now.UTC()
	return e
}

// syncBillDoc brings the reasons on the stored bill document, if there is one,
// in line with the catalog entry e, which is the record. It is idempotent, so
// repeating an action repairs a document an earlier call left behind.
func (s *Server) syncBillDoc(ctx context.Context, e data.CatalogEntry) error {
	b, err := s.Ports.PublicDocs.GetBill(ctx, e.Bill)
	if errors.Is(err, data.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read bill %s: %w", e.Bill.ID(), err)
	}
	if slices.Equal(b.TrackedReasons, e.Reasons) && b.PolicyVersion == e.PolicyVersion {
		return nil
	}
	b.TrackedReasons, b.PolicyVersion = slices.Clone(e.Reasons), e.PolicyVersion
	b.Meta.Version++
	b.Meta.AsOf = s.Now().UTC()
	if err := s.Ports.PlatformDocs.PutBill(ctx, b); err != nil {
		return fmt.Errorf("logged, but bill %s still shows its old reasons; repeat the action: %w", e.Bill.ID(), err)
	}
	return nil
}

// untrack reverses a selection made in error. The bill's stored text stays.
func (s *Server) untrack(w http.ResponseWriter, r *http.Request) {
	ref, ok := s.billRef(w, r)
	if !ok {
		return
	}
	body, ok := bodyReasoned(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	e, err := s.catalogEntry(ctx, ref)
	if err != nil {
		s.fail(w, err)
		return
	}
	res := ActionResult{Bill: ref.ID()}
	if e.Tracked {
		e.Tracked, e.Reasons, e.PolicyVersion, e.TrackedSince = false, []string{}, 0, ""
		e.MarkupStatus = data.MarkupEligible
		e.Meta.Version++
		e.Meta.AsOf = s.Now().UTC()
		le := s.entry(KindUntracked, ref.String(), body.Reason, operatorOf(ctx))
		if err := s.Ports.PlatformDocs.ApplyLogged(ctx, data.Change{Catalog: &e}, le); err != nil {
			s.fail(w, fmt.Errorf("untrack %s: %w", ref.ID(), err))
			return
		}
		res.Changed, res.LogID = true, le.ID
		res.Note = "the stored bill text is not removed"
	}
	if err := s.syncBillDoc(ctx, e); err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, res)
}

// patchCatalog corrects fields ingestion got wrong. It cannot touch what a
// bill says, who sponsors it, or whether it is tracked.
func (s *Server) patchCatalog(w http.ResponseWriter, r *http.Request) {
	ref, ok := s.billRef(w, r)
	if !ok {
		return
	}
	var p CatalogPatch
	if !decode(w, r, &p, func() string { return p.Reason }) {
		return
	}
	ctx := r.Context()
	e, err := s.catalogEntry(ctx, ref)
	if err != nil {
		s.fail(w, err)
		return
	}
	next, err := applyPatch(e, p)
	if err != nil {
		s.fail(w, fmt.Errorf("%w: %w", err, data.ErrInvalid))
		return
	}
	if reflect.DeepEqual(next, e) {
		writeJSON(w, ActionResult{Bill: ref.ID(), Note: errNoChange.Error()})
		return
	}
	next.Meta.Version++
	next.Meta.AsOf = s.Now().UTC()
	le := s.entry(KindPatched, ref.String(), p.Reason, operatorOf(ctx))
	if err := s.Ports.PlatformDocs.ApplyLogged(ctx, data.Change{Catalog: &next}, le); err != nil {
		s.fail(w, fmt.Errorf("patch %s: %w", ref.ID(), err))
		return
	}
	writeJSON(w, ActionResult{Bill: ref.ID(), Changed: true, LogID: le.ID})
}

func applyPatch(e data.CatalogEntry, p CatalogPatch) (data.CatalogEntry, error) {
	set := 0
	for _, f := range []struct {
		given bool
		apply func()
	}{
		{p.Title != nil, func() { e.Title = *p.Title }},
		{p.Status != nil, func() { e.Status = *p.Status }},
		{p.Chamber != nil, func() { e.Chamber = *p.Chamber }},
		{p.Introduced != nil, func() { e.Introduced = *p.Introduced }},
		{p.PolicyArea != nil, func() { e.PolicyArea = *p.PolicyArea }},
		{p.Subjects != nil, func() { e.Subjects = slices.Clone(*p.Subjects) }},
	} {
		if f.given {
			f.apply()
			set++
		}
	}
	switch {
	case set == 0:
		return e, errEmptyPatch
	case e.Title == "":
		return e, errEmptyTitle
	}
	return e, nil
}

// reingest fetches the bill again and compares each stored original by
// SHA-256; a mismatch is reported and nothing is overwritten. Missing
// versions are stored. The public log entry is written first.
func (s *Server) reingest(w http.ResponseWriter, r *http.Request) {
	ref, ok := s.billRef(w, r)
	if !ok {
		return
	}
	body, ok := bodyReasoned(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	if s.Fetcher == nil {
		s.fail(w, fmt.Errorf("%w: %w", errNoFetcher, data.ErrUnavailable))
		return
	}
	le := s.entry(KindReingest, ref.String(), body.Reason, operatorOf(ctx))
	if err := s.Ports.Moderation.Record(ctx, le); err != nil {
		s.fail(w, fmt.Errorf("reingest %s: log: %w", ref.ID(), err))
		return
	}
	res := ActionResult{Bill: ref.ID(), Changed: true, LogID: le.ID}
	rep, err := s.pull(ctx, ref, ingest.Options{Verify: true})
	res.Pull = &rep
	if err != nil {
		res.PullError = err.Error()
	}
	writeJSON(w, res)
}

// reparse parses a stored version's original again with the current parser.
// Storing a second parser version beside the stored one has no storage path
// yet (the parsed text is write-once per version), so a different parser
// version is reported, not written, and nothing is logged.
func (s *Server) reparse(w http.ResponseWriter, r *http.Request) {
	ref, ok := s.billRef(w, r)
	if !ok {
		return
	}
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil || n < 1 {
		s.fail(w, fmt.Errorf("version %q: %w", r.PathValue("n"), data.ErrInvalid))
		return
	}
	if _, ok := bodyReasoned(w, r); !ok {
		return
	}
	res, err := s.reparseVersion(r.Context(), ref, n)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, res)
}

// reparseVersion parses the n-th stored version again and compares.
func (s *Server) reparseVersion(ctx context.Context, ref data.BillRef, n int) (ReparseResult, error) {
	vs, err := s.Ports.BillText.ListVersions(ctx, ref)
	if err != nil {
		return ReparseResult{}, fmt.Errorf("list versions: %w", err)
	}
	if n > len(vs) {
		return ReparseResult{}, fmt.Errorf("%s has %d stored versions: %w", ref.ID(), len(vs), data.ErrNotFound)
	}
	stored, err := s.Ports.BillText.GetVersion(ctx, vs[n-1].Ref)
	if err != nil {
		return ReparseResult{}, fmt.Errorf("read version: %w", err)
	}
	src, err := s.Ports.BillText.GetSource(ctx, vs[n-1].Ref)
	if err != nil {
		return ReparseResult{}, fmt.Errorf("read original: %w", err)
	}
	parsed, err := billxml.Parse(src, s.Log)
	if err != nil {
		return ReparseResult{}, fmt.Errorf("parse: %w: %w", err, data.ErrInvalid)
	}
	res := ReparseResult{
		Bill: ref.ID(), Version: n, StoredParser: stored.ParserVersion, CurrentParser: billxml.ParserVersion,
	}
	switch {
	case stored.ParserVersion != billxml.ParserVersion:
		return res, fmt.Errorf("a new parser version cannot be stored beside the old one yet: %w", data.ErrUnavailable)
	case !reflect.DeepEqual(stored.Sections, parsed.DataSections()):
		return res, fmt.Errorf("the current parser gives different text under the same parser version: %w",
			data.ErrConflict)
	}
	res.Note = "the stored text is what the current parser produces; nothing changed"
	return res, nil
}
