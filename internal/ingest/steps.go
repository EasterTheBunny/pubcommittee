package ingest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/billxml"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

// DefaultSteps is the M1 pull. Extension points:
//   - StepPolicy: replace with PolicyStep(<M1-06 evaluator>).
//   - StepAfterStore: an empty step that runs once per pull after the versions
//     step; insert steps after it (Pipeline.After(StepAfterStore, ...)) that
//     read Run.Stored, the versions this pull stored (citations M1-07,
//     version comparison M1-08).
func DefaultSteps() Pipeline {
	return Pipeline{
		StepFunc{StepName: StepFetchRecord, Fn: fetchRecord},
		StepFunc{StepName: StepCatalog, Fn: buildCatalog},
		PolicyStep(ManualEvaluator{}),
		StepFunc{StepName: StepVersions, Fn: storeVersions},
		StepFunc{StepName: StepAfterStore, Fn: func(context.Context, *Run) error { return nil }},
		StepFunc{StepName: StepWriteBill, Fn: writeBill},
		StepFunc{StepName: StepWriteCatalog, Fn: writeCatalog},
		StepFunc{StepName: StepIngestState, Fn: recordState},
	}
}

func fetchRecord(ctx context.Context, r *Run) error {
	rec, err := r.Fetcher.FetchRecord(ctx, r.Bill)
	if err != nil {
		return fmt.Errorf("fetch status: %w", err)
	}
	r.Record = rec
	b, err := r.Ports.PublicDocs.GetBill(ctx, r.Bill)
	switch {
	case err == nil:
		r.Existing = &b
	case errors.Is(err, data.ErrNotFound):
	default:
		return fmt.Errorf("read bill: %w", err)
	}
	return nil
}

// asOf is the record's own update time, so a repeat pull of an unchanged
// status writes identical documents.
func (r *Run) asOf() time.Time {
	if r.Record.UpdatedAt.IsZero() {
		return time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
	}
	return r.Record.UpdatedAt.UTC()
}

func (r *Run) meta() data.Meta {
	return data.Meta{Version: 1, AsOf: r.asOf(), Provenance: data.ProvenanceSource, ContentClass: data.ContentOfficial}
}

func buildCatalog(_ context.Context, r *Run) error {
	rec := r.Record
	e := data.CatalogEntry{
		Bill: r.Bill, Title: rec.Title, Status: StatusOf(rec), Chamber: rec.Chamber,
		Subjects: slices.Clone(rec.Subjects), CommitteeActivity: HasCommitteeAction(rec.CommitteeActivity),
		OfficialLinks: OfficialLinks(rec), SearchTerms: SearchTerms(rec),
		SponsorID: rec.SponsorID, Sponsor: rec.Sponsor, SponsorParty: rec.SponsorParty,
		Introduced: rec.Introduced, PolicyArea: rec.PolicyArea,
		Committees: slices.Clone(rec.CommitteeActivity), Related: slices.Clone(rec.Related),
		Reasons: []string{}, Meta: r.meta(),
	}
	if rec.LatestAction.Text != "" {
		a := rec.LatestAction
		e.LatestAction = &a
	}
	r.Entry = e
	return nil
}

// OfficialLinks lists the bill page and each text version's official URL. A
// record without its own bill page gets one derived from the bill id
// (DerivedBillPageURL), labelled as derived.
func OfficialLinks(rec data.BillRecord) []data.Link {
	var out []data.Link
	if rec.BillPageURL != "" {
		out = append(out, data.Link{Label: "Congress.gov", URL: rec.BillPageURL, Kind: data.LinkBillPage})
	}
	for _, v := range rec.Versions {
		out = append(out, data.Link{Label: v.Type, URL: v.URL, Kind: data.LinkTextVersion})
	}
	return withBillPage(rec.Bill, out)
}

// HasCommitteeAction is true when a committee did more than receive the
// referral (hearing, markup, reported, discharged ...).
func HasCommitteeAction(acts []data.CommitteeActivity) bool {
	return slices.ContainsFunc(acts, func(a data.CommitteeActivity) bool {
		return !strings.HasPrefix(strings.ToLower(a.Activity), "referred")
	})
}

// StatusOf derives a coarse status from the latest action text.
func StatusOf(rec data.BillRecord) string {
	t := strings.ToLower(rec.LatestAction.Text)
	switch {
	case strings.Contains(t, "became public law"):
		return "enacted"
	case strings.Contains(t, "presented to president"), strings.Contains(t, "signed by president"):
		return "presented"
	case strings.Contains(t, "passed/agreed to in house"), strings.Contains(t, "passed house"):
		return "passed-house"
	case strings.Contains(t, "passed/agreed to in senate"), strings.Contains(t, "passed senate"):
		return "passed-senate"
	case strings.Contains(t, "reported"), strings.Contains(t, "placed on"):
		return wordReported
	case strings.HasPrefix(t, "referred"), strings.HasPrefix(t, "read twice"),
		strings.HasPrefix(t, "received in the senate"):
		return "referred"
	}
	return wordIntroduced
}

// SearchTerms normalises the bill number, title words and subject terms:
// lower case, letters and digits only, sorted, without duplicates.
func SearchTerms(rec data.BillRecord) []string {
	set := map[string]bool{}
	add := func(s string) {
		for _, w := range strings.FieldsFunc(strings.ToLower(s), func(c rune) bool {
			return (c < 'a' || c > 'z') && (c < '0' || c > '9')
		}) {
			if len(w) > 2 || strings.ContainsAny(w, "0123456789") {
				set[w] = true
			}
		}
	}
	number := strconv.Itoa(rec.Bill.Number)
	set[rec.Bill.Type+number] = true
	set[number] = true
	add(rec.Title)
	add(rec.PolicyArea)
	for _, s := range rec.Subjects {
		add(s)
	}
	out := make([]string, 0, len(set))
	for w := range set {
		out = append(out, w)
	}
	slices.Sort(out)
	return out
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Status and version type words.
const (
	wordIntroduced = "introduced"
	wordReported   = "reported"
)

// versionType maps a version code to the OfficialVersion.Type vocabulary.
func versionType(v data.SourceVersion) string {
	switch v.Ref.Version {
	case "ih", "is":
		return wordIntroduced
	case "rh", "rs":
		return wordReported
	case "eh", "es":
		return "engrossed"
	case "rfs":
		return "referred-in-senate"
	case "rfh":
		return "referred-in-house"
	case "enr":
		return "enrolled"
	case "pcs", "pch":
		return "placed-on-calendar"
	case "cph", "cps":
		return "considered-and-passed"
	}
	return strings.ReplaceAll(strings.ToLower(v.Type), " ", "-")
}

func (r *Run) fail(item string, err error) {
	r.Log.Warn("pull failure", "item", item, "error", err.Error())
	r.Report.Failures = append(r.Report.Failures, Failure{Item: item, Error: shortErr(err)})
}

func shortErr(err error) string {
	s := err.Error()
	const limit = 300
	if len(s) > limit {
		s = s[:limit]
	}
	return s
}

// storeVersions stores every listed version not stored yet, for a tracked
// bill only. An untracked bill's text is never fetched.
func storeVersions(ctx context.Context, r *Run) error {
	if !r.Decision.Tracked {
		return nil
	}
	stored, err := r.Ports.BillText.ListVersions(ctx, r.Bill)
	if err != nil {
		return fmt.Errorf("list stored versions: %w", err)
	}
	have := map[string]bool{}
	for _, s := range stored {
		have[s.Ref.Version] = true
	}
	for _, v := range r.Record.Versions {
		switch {
		case have[v.Ref.Version]:
			r.present(ctx, v)
		case r.DryRun:
			r.Report.WouldStore = append(r.Report.WouldStore, v.Ref.Version)
		default:
			r.storeNew(ctx, v)
		}
	}
	return nil
}

// present notes a stored version and, with Verify, compares a re-fetch.
func (r *Run) present(ctx context.Context, v data.SourceVersion) {
	r.Report.Present = append(r.Report.Present, v.Ref.Version)
	if !r.Verify || r.DryRun {
		return
	}
	if err := verifyStored(ctx, r, v); err != nil {
		r.fail(v.Ref.String(), err)
	}
}

func (r *Run) storeNew(ctx context.Context, v data.SourceVersion) {
	if err := storeOne(ctx, r, v); err != nil {
		r.fail(v.Ref.String(), err)
		return
	}
	r.Stored = append(r.Stored, v.Ref)
	r.Report.Stored = append(r.Report.Stored, v.Ref.Version)
}

// errHashMismatch marks a re-fetched original that differs from the stored one.
var errHashMismatch = errors.New("original differs from the stored one (SHA-256 mismatch); nothing was changed")

func verifyStored(ctx context.Context, r *Run, v data.SourceVersion) error {
	doc, err := r.Fetcher.FetchOriginal(ctx, v)
	if err != nil {
		return fmt.Errorf("re-fetch: %w", err)
	}
	old, err := r.Ports.BillText.GetSource(ctx, v.Ref)
	if err != nil {
		return fmt.Errorf("read stored original: %w", err)
	}
	if sum(old) != sum(doc.XML) {
		return fmt.Errorf("%w: stored %s, fetched %s", errHashMismatch, sum(old), sum(doc.XML))
	}
	return nil
}

func storeOne(ctx context.Context, r *Run, v data.SourceVersion) error {
	doc, err := r.Fetcher.FetchOriginal(ctx, v)
	if err != nil {
		return fmt.Errorf("fetch original: %w", err)
	}
	// A source left by an interrupted pull must equal what we fetched now.
	if old, err := r.Ports.BillText.GetSource(ctx, v.Ref); err == nil && !bytes.Equal(old, doc.XML) {
		return fmt.Errorf("%w: stored %s, fetched %s", errHashMismatch, sum(old), sum(doc.XML))
	}
	parsed, err := billxml.Parse(doc.XML, slog.New(slog.DiscardHandler))
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	ver := data.OfficialVersion{
		Ref: v.Ref, Type: versionType(v), PublishedAt: v.PublishedAt.UTC(), SourceSHA256: sum(doc.XML),
		ParserVersion: billxml.ParserVersion, Sections: parsed.DataSections(),
		Meta: data.Meta{
			Version: 1, AsOf: v.PublishedAt.UTC(), Provenance: data.ProvenanceSource, ContentClass: data.ContentOfficial,
		},
		Source: doc.Info,
	}
	if err := r.Ports.Publisher.PutVersion(ctx, ver, doc.XML); err != nil {
		return fmt.Errorf("store: %w", err)
	}
	return nil
}

func writeBill(ctx context.Context, r *Run) error {
	if !r.Decision.Tracked || r.DryRun {
		return nil
	}
	vs, err := r.Ports.BillText.ListVersions(ctx, r.Bill)
	if err != nil {
		return fmt.Errorf("list versions: %w", err)
	}
	b := data.Bill{
		Ref: r.Bill, Title: r.Record.Title, Sponsor: r.Record.Sponsor, SponsorID: r.Record.SponsorID,
		Status: r.Entry.Status, Chamber: r.Record.Chamber, ReviewState: data.ReviewOpen,
		OfficialLinks: r.Entry.OfficialLinks, TrackedReasons: r.Decision.Reasons, PolicyVersion: r.Decision.PolicyVersion,
		Versions: vs, Meta: r.meta(),
	}
	if r.Existing != nil {
		b.Meta.Version = r.Existing.Meta.Version
		if !reflect.DeepEqual(*r.Existing, b) {
			b.Meta.Version++
		} else {
			return nil
		}
	}
	if err := r.Ports.PlatformDocs.PutBill(ctx, b); err != nil {
		return fmt.Errorf("put bill: %w", err)
	}
	r.Report.BillWritten = true
	return nil
}

func writeCatalog(ctx context.Context, r *Run) error {
	if r.DryRun {
		return nil
	}
	if err := r.Ports.PlatformDocs.PutCatalogEntry(ctx, r.Entry); err != nil {
		return fmt.Errorf("put catalog entry: %w", err)
	}
	r.Report.CatalogWritten = true
	return nil
}

// recordState keeps the bill's failed items in ingest_state for the next pull
// to retry and for operators to inspect. There is no watermark in M1.
// The read-modify-write is not atomic: two overlapping pulls of different
// bills can lose each other's failure entry; the next pull of that bill
// records it again.
func recordState(ctx context.Context, r *Run) error {
	if r.DryRun {
		return nil
	}
	st, err := r.Ports.PlatformDocs.GetIngestState(ctx, r.Source)
	if errors.Is(err, data.ErrNotFound) {
		st = data.IngestState{Source: r.Source}
	} else if err != nil {
		return fmt.Errorf("read ingest state: %w", err)
	}
	failed := r.stateFailures(st.Failed)
	if len(failed) == 0 && len(st.Failed) == 0 {
		return nil // nothing to record; do not create an empty state
	}
	now := r.Now().UTC()
	st.Failed = failed
	st.Meta = data.Meta{
		Version: st.Meta.Version + 1, AsOf: now, Provenance: data.ProvenanceSource, ContentClass: data.ContentPlatform,
	}
	if err := r.Ports.PlatformDocs.PutIngestState(ctx, st); err != nil {
		return fmt.Errorf("put ingest state: %w", err)
	}
	return nil
}

// stateFailures replaces this bill's entries in old with the run's failures.
func (r *Run) stateFailures(old []data.IngestFailure) []data.IngestFailure {
	prefix := r.Bill.String()
	var failed []data.IngestFailure
	for _, f := range old {
		if f.Item != prefix && !strings.HasPrefix(f.Item, prefix+"/") {
			failed = append(failed, f)
		}
	}
	now := r.Now().UTC()
	for _, f := range r.Report.Failures {
		failed = append(failed, data.IngestFailure{Item: f.Item, Error: f.Error, At: now})
	}
	return failed
}
