package ingest_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/adapters/fixturesrc"
	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/ingest"
)

// fixedTime is the clock of the first pull.
func fixedTime() time.Time {
	return time.Date(2026, time.November, 22, 12, 0, 0, 0, time.UTC)
}

// laterTime is the clock of a repeat pull, one day after fixedTime.
func laterTime() time.Time {
	return fixedTime().Add(24 * time.Hour)
}

// bill returns the us/119/hr bill with the given number.
func bill(number int) data.BillRef {
	return data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: number}
}

// hr4313Codes are the version codes of hr4313 in sorted order.
func hr4313Codes() []string {
	return []string{"eh", "ih", "rfs", "rh"}
}

// noTextFetcher fails the test if any original text is fetched.
type noTextFetcher struct {
	data.BillFetcher

	t     *testing.T
	calls atomic.Int32
}

// FetchOriginal records the call, reports it as a test error, and refuses it.
func (f *noTextFetcher) FetchOriginal(_ context.Context, v data.SourceVersion) (data.SourceDocument, error) {
	f.calls.Add(1)
	f.t.Errorf("unexpected FetchOriginal for %s", v.Ref)
	return data.SourceDocument{}, fmt.Errorf("test: no text expected: %w", data.ErrUnavailable)
}

// tamperFetcher returns the real original, with one extra trailing byte for one version code.
type tamperFetcher struct {
	data.BillFetcher

	code string
}

// FetchOriginal returns the fixture bytes, changed for the tampered version code.
func (f tamperFetcher) FetchOriginal(ctx context.Context, v data.SourceVersion) (data.SourceDocument, error) {
	doc, err := f.BillFetcher.FetchOriginal(ctx, v)
	if err != nil {
		return data.SourceDocument{}, fmt.Errorf("tamper: %w", err)
	}
	if v.Ref.Version == f.code {
		doc.XML = append(slices.Clone(doc.XML), '\n')
	}
	return doc, nil
}

// fixtureSource returns the fixture-backed fetcher with the given clock.
func fixtureSource(t *testing.T, now time.Time) *fixturesrc.Source {
	t.Helper()
	src, err := fixturesrc.New(fixturesrc.Config{
		FS:  os.DirFS("../../fixtures"),
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("fixturesrc.New: %v", err)
	}
	return src
}

// deps returns the pull collaborators over store, with the given fetcher and clock.
func deps(store *memory.Store, f data.BillFetcher, now time.Time) ingest.Deps {
	return ingest.Deps{
		Ports:   store.PlatformPorts(),
		Fetcher: f,
		Source:  "fixtures/119",
		Log:     slog.New(slog.DiscardHandler),
		Now:     func() time.Time { return now },
	}
}

// fixtureBytes reads one fixture file under fixtures/, failing the test on error.
func fixtureBytes(t *testing.T, parts ...string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(append([]string{"..", "..", "fixtures"}, parts...)...))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return b
}

// sourceFixture returns the fixture original of one version of hr4313.
func sourceFixture(t *testing.T, code string) []byte {
	t.Helper()
	return fixtureBytes(t, "bills", bill(4313).ID(), "v", code, "source.xml")
}

// sha256Hex returns the lower-case hex SHA-256 of b.
func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// pull runs the default pipeline for one bill and fails the test on a Pull error.
func pull(t *testing.T, d ingest.Deps, o ingest.Options, ref data.BillRef) ingest.Report {
	t.Helper()
	rep, err := ingest.Pull(t.Context(), d, o, ingest.DefaultSteps(), ref)
	if err != nil {
		t.Fatalf("Pull(%s): %v", ref, err)
	}
	return rep
}

// catalogEntry returns the catalog entry of ref, failing the test when absent.
func catalogEntry(t *testing.T, store *memory.Store, ref data.BillRef) data.CatalogEntry {
	t.Helper()
	page, err := store.ListCatalog(t.Context(), data.Page{})
	if err != nil {
		t.Fatalf("ListCatalog: %v", err)
	}
	for _, e := range page.Items {
		if e.Bill == ref {
			return e
		}
	}
	t.Fatalf("no catalog entry for %s", ref)
	return data.CatalogEntry{}
}

// countLinks returns how many links of one kind a catalog entry holds.
func countLinks(links []data.Link, kind string) int {
	n := 0
	for _, l := range links {
		if l.Kind == kind {
			n++
		}
	}
	return n
}

// failureItems returns the Item of each ingest_state failure.
func failureItems(st data.IngestState) []string {
	out := make([]string, 0, len(st.Failed))
	for _, f := range st.Failed {
		out = append(out, f.Item)
	}
	return out
}

// checkStoredVersion compares one stored version with its fixture original.
func checkStoredVersion(t *testing.T, store *memory.Store, code string) {
	t.Helper()
	ctx := t.Context()
	ref := data.VersionRef{Bill: bill(4313), Version: code}
	want := sourceFixture(t, code)

	raw, err := store.GetSource(ctx, ref)
	if err != nil {
		t.Fatalf("GetSource %s: %v", ref, err)
	}
	if !slices.Equal(raw, want) {
		t.Errorf("GetSource %s: bytes differ from fixture", ref)
	}

	ver, err := store.GetVersion(ctx, ref)
	if err != nil {
		t.Fatalf("GetVersion %s: %v", ref, err)
	}
	if ver.ParserVersion != "billxml/1" {
		t.Errorf("GetVersion %s: ParserVersion = %q, want billxml/1", ref, ver.ParserVersion)
	}
	if ver.SourceSHA256 != sha256Hex(want) {
		t.Errorf("GetVersion %s: SourceSHA256 = %s, want %s", ref, ver.SourceSHA256, sha256Hex(want))
	}
	if len(ver.Sections) == 0 {
		t.Errorf("GetVersion %s: no sections", ref)
	}

	info, err := store.GetSourceInfo(ctx, ref)
	if err != nil {
		t.Fatalf("GetSourceInfo %s: %v", ref, err)
	}
	if !strings.HasPrefix(info.URL, "https://www.govinfo.gov/content/pkg/BILLS-119hr4313") {
		t.Errorf("GetSourceInfo %s: URL = %q", ref, info.URL)
	}
	if !info.RetrievedAt.Equal(fixedTime()) {
		t.Errorf("GetSourceInfo %s: RetrievedAt = %v, want %v", ref, info.RetrievedAt, fixedTime())
	}
}

// checkTrackedBill compares the bill document and catalog entry of a tracked hr4313.
func checkTrackedBill(t *testing.T, store *memory.Store) {
	t.Helper()
	doc, err := store.GetBill(t.Context(), bill(4313))
	if err != nil {
		t.Fatalf("GetBill: %v", err)
	}
	if len(doc.Versions) != 3 {
		t.Errorf("GetBill: %d versions, want 3 (rfs collapsed into eh)", len(doc.Versions))
	}
	if !slices.Equal(doc.TrackedReasons, []string{"manual-pull"}) {
		t.Errorf("GetBill: TrackedReasons = %v, want [manual-pull]", doc.TrackedReasons)
	}
	if doc.SponsorID != "B001260" {
		t.Errorf("GetBill: SponsorID = %q, want B001260", doc.SponsorID)
	}

	checkTrackedCatalog(t, store, doc.Ref)
}

// checkTrackedCatalog compares the catalog entry of a tracked bill.
func checkTrackedCatalog(t *testing.T, store *memory.Store, ref data.BillRef) {
	t.Helper()
	entry := catalogEntry(t, store, ref)
	if !entry.Tracked {
		t.Error("catalog entry: Tracked = false, want true")
	}
	if entry.MarkupStatus != data.MarkupInReview {
		t.Errorf("catalog entry: MarkupStatus = %q, want in-review", entry.MarkupStatus)
	}
	if !slices.Contains(entry.SearchTerms, "hr4313") || !slices.Contains(entry.SearchTerms, "hospital") {
		t.Errorf("catalog entry: SearchTerms = %v, want hr4313 and hospital", entry.SearchTerms)
	}
	if n := countLinks(entry.OfficialLinks, data.LinkBillPage); n != 1 {
		t.Errorf("catalog entry: %d bill-page links, want 1", n)
	}
	if n := countLinks(entry.OfficialLinks, data.LinkTextVersion); n != 4 {
		t.Errorf("catalog entry: %d text-version links, want 4", n)
	}
}

// snapshot is the public state of one bill after a pull.
type snapshot struct {
	catalog data.Paged[data.CatalogEntry]
	bill    data.Bill
	info    data.SourceInfo
}

// takeSnapshot reads the public state of hr4313 and the source info of its ih version.
func takeSnapshot(t *testing.T, store *memory.Store) snapshot {
	t.Helper()
	ctx := t.Context()
	cat, err := store.ListCatalog(ctx, data.Page{})
	if err != nil {
		t.Fatalf("ListCatalog: %v", err)
	}
	b, err := store.GetBill(ctx, bill(4313))
	if err != nil {
		t.Fatalf("GetBill: %v", err)
	}
	info, err := store.GetSourceInfo(ctx, data.VersionRef{Bill: bill(4313), Version: "ih"})
	if err != nil {
		t.Fatalf("GetSourceInfo: %v", err)
	}
	return snapshot{catalog: cat, bill: b, info: info}
}

// checkRepeatReport checks that a pull of an unchanged bill stored and wrote nothing.
func checkRepeatReport(t *testing.T, rep ingest.Report) {
	t.Helper()
	if len(rep.Stored) != 0 {
		t.Errorf("second pull Stored = %v, want none", rep.Stored)
	}
	present := slices.Clone(rep.Present)
	slices.Sort(present)
	if want := hr4313Codes(); !slices.Equal(present, want) {
		t.Errorf("second pull Present = %v, want %v", present, want)
	}
	if rep.BillWritten {
		t.Error("second pull BillWritten = true, want false")
	}
	if !rep.Tracked {
		t.Error("second pull Tracked = false, want true (the bill document exists)")
	}
}

// checkMismatchReport checks that a verify pull reported exactly the ih SHA-256 mismatch.
func checkMismatchReport(t *testing.T, rep ingest.Report) {
	t.Helper()
	if !rep.Failed() {
		t.Fatal("report.Failed() = false, want true")
	}
	for _, f := range rep.Failures {
		if strings.HasSuffix(f.Item, "/ih") && strings.Contains(f.Error, "SHA-256 mismatch") {
			return
		}
	}
	t.Errorf("no failure for .../ih with SHA-256 mismatch: %+v", rep.Failures)
}

func TestPullTrackedStoresEverything(t *testing.T) {
	t.Parallel()
	store := memory.New()

	rep, err := ingest.Pull(t.Context(), deps(store, fixtureSource(t, fixedTime()), fixedTime()),
		ingest.Options{Track: true}, ingest.DefaultSteps(), bill(4313))
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if rep.Failed() {
		t.Fatalf("report failed: %+v", rep.Failures)
	}
	if !rep.Tracked {
		t.Error("report.Tracked = false, want true")
	}
	stored := slices.Clone(rep.Stored)
	slices.Sort(stored)
	if want := hr4313Codes(); !slices.Equal(stored, want) {
		t.Errorf("Stored = %v, want %v", stored, want)
	}

	versions, err := store.ListVersions(t.Context(), bill(4313))
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 3 {
		t.Fatalf("ListVersions returned %d versions, want 3 (rfs collapsed into eh)", len(versions))
	}
	for _, code := range hr4313Codes() {
		checkStoredVersion(t, store, code)
	}
	checkTrackedBill(t, store)
}

func TestSecondPullChangesNothing(t *testing.T) {
	t.Parallel()
	store := memory.New()

	pull(t, deps(store, fixtureSource(t, fixedTime()), fixedTime()), ingest.Options{Track: true}, bill(4313))
	before := takeSnapshot(t, store)

	rep := pull(t, deps(store, fixtureSource(t, laterTime()), laterTime()), ingest.Options{Track: false}, bill(4313))
	checkRepeatReport(t, rep)

	after := takeSnapshot(t, store)
	if !reflect.DeepEqual(before.catalog, after.catalog) {
		t.Error("catalog changed by the second pull")
	}
	if !reflect.DeepEqual(before.bill, after.bill) {
		t.Error("bill changed by the second pull")
	}
	if !after.info.RetrievedAt.Equal(fixedTime()) {
		t.Errorf("RetrievedAt = %v after second pull, want first pull time %v", after.info.RetrievedAt, fixedTime())
	}
	if !reflect.DeepEqual(before.info, after.info) {
		t.Error("source info changed by the second pull")
	}
}

func TestUntrackedStoresNoText(t *testing.T) {
	t.Parallel()
	store := memory.New()
	f := &noTextFetcher{BillFetcher: fixtureSource(t, fixedTime()), t: t}

	rep := pull(t, deps(store, f, fixedTime()), ingest.Options{}, bill(4313))

	if rep.Tracked {
		t.Error("report.Tracked = true, want false")
	}
	if rep.Failed() {
		t.Errorf("report failed: %+v", rep.Failures)
	}
	entry := catalogEntry(t, store, bill(4313))
	if entry.Tracked {
		t.Error("catalog entry: Tracked = true, want false")
	}
	if entry.MarkupStatus != data.MarkupEligible {
		t.Errorf("catalog entry: MarkupStatus = %q, want eligible", entry.MarkupStatus)
	}
	if n := countLinks(entry.OfficialLinks, data.LinkTextVersion); n != 4 {
		t.Errorf("catalog entry: %d text-version links, want 4", n)
	}
	versions, err := store.ListVersions(t.Context(), bill(4313))
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("ListVersions = %d versions, want none", len(versions))
	}
	if _, err := store.GetBill(t.Context(), bill(4313)); !errors.Is(err, data.ErrNotFound) {
		t.Errorf("GetBill error = %v, want ErrNotFound", err)
	}
	if n := f.calls.Load(); n != 0 {
		t.Errorf("FetchOriginal calls = %d, want 0", n)
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	t.Parallel()
	store := memory.New()
	f := &noTextFetcher{BillFetcher: fixtureSource(t, fixedTime()), t: t}

	rep := pull(t, deps(store, f, fixedTime()), ingest.Options{Track: true, DryRun: true}, bill(4313))

	if rep.CatalogWritten || rep.BillWritten {
		t.Errorf("dry run wrote: catalog=%v bill=%v", rep.CatalogWritten, rep.BillWritten)
	}
	if !rep.DryRun {
		t.Error("report.DryRun = false, want true")
	}
	if len(rep.WouldStore) != 4 {
		t.Errorf("WouldStore = %v, want 4 entries", rep.WouldStore)
	}
	page, err := store.ListCatalog(t.Context(), data.Page{})
	if err != nil {
		t.Fatalf("ListCatalog: %v", err)
	}
	if len(page.Items) != 0 {
		t.Errorf("ListCatalog = %d entries, want none", len(page.Items))
	}
	versions, err := store.ListVersions(t.Context(), bill(4313))
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("ListVersions = %d versions, want none", len(versions))
	}
	if n := f.calls.Load(); n != 0 {
		t.Errorf("FetchOriginal calls = %d, want 0", n)
	}
}

func TestHashMismatchReportedNotApplied(t *testing.T) {
	t.Parallel()
	store := memory.New()

	pull(t, deps(store, fixtureSource(t, fixedTime()), fixedTime()), ingest.Options{Track: true}, bill(4313))

	tampered := tamperFetcher{BillFetcher: fixtureSource(t, fixedTime()), code: "ih"}
	rep, err := ingest.Pull(t.Context(), deps(store, tampered, fixedTime()),
		ingest.Options{Verify: true}, ingest.DefaultSteps(), bill(4313))
	if err != nil {
		t.Fatalf("Pull returned error %v, want nil (mismatch is a reported failure)", err)
	}
	checkMismatchReport(t, rep)

	got, err := store.GetSource(t.Context(), data.VersionRef{Bill: bill(4313), Version: "ih"})
	if err != nil {
		t.Fatalf("GetSource ih: %v", err)
	}
	if !slices.Equal(got, sourceFixture(t, "ih")) {
		t.Error("stored ih source changed by the verify pull; want fixture bytes")
	}

	st, err := store.GetIngestState(t.Context(), "fixtures/119")
	if err != nil {
		t.Fatalf("GetIngestState: %v", err)
	}
	if !slices.ContainsFunc(failureItems(st), func(s string) bool { return strings.HasSuffix(s, "/ih") }) {
		t.Errorf("ingest_state failures = %v, want an item ending /ih", failureItems(st))
	}
}

func TestMissingBill(t *testing.T) {
	t.Parallel()
	store := memory.New()
	missing := bill(999999)

	_, err := ingest.Pull(t.Context(), deps(store, fixtureSource(t, fixedTime()), fixedTime()),
		ingest.Options{}, ingest.DefaultSteps(), missing)
	if !errors.Is(err, data.ErrNotFound) {
		t.Fatalf("Pull error = %v, want ErrNotFound", err)
	}

	st, err := store.GetIngestState(t.Context(), "fixtures/119")
	if err != nil {
		t.Fatalf("GetIngestState: %v", err)
	}
	if !slices.Contains(failureItems(st), missing.String()) {
		t.Errorf("ingest_state failures = %v, want %s", failureItems(st), missing)
	}
}

func TestPipelineExtension(t *testing.T) {
	t.Parallel()
	store := memory.New()

	var seen []string
	probe := ingest.StepFunc{
		StepName: "probe",
		Fn: func(_ context.Context, r *ingest.Run) error {
			seen = append(seen, strconv.Itoa(len(r.Stored)))
			return nil
		},
	}
	p, err := ingest.DefaultSteps().After(ingest.StepAfterStore, probe)
	if err != nil {
		t.Fatalf("After(%s): %v", ingest.StepAfterStore, err)
	}
	rep, err := ingest.Pull(t.Context(), deps(store, fixtureSource(t, fixedTime()), fixedTime()),
		ingest.Options{Track: true}, p, bill(3427))
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if rep.Failed() {
		t.Fatalf("report failed: %+v", rep.Failures)
	}
	if !slices.Equal(seen, []string{"4"}) {
		t.Errorf("probe saw stored counts %v, want [4]", seen)
	}

	if _, err := ingest.DefaultSteps().After("no-such-step", probe); !errors.Is(err, data.ErrNotFound) {
		t.Errorf("After unknown name: error = %v, want ErrNotFound", err)
	}
}

func TestStatusMapping(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		action string
		want   string
	}{
		{name: "enacted", action: "Became Public Law No: 119-75.", want: "enacted"},
		{name: "referred", action: "Received in the Senate and Read twice and referred to the Committee on Finance.", want: "referred"},
		{name: "passed house", action: "Passed/agreed to in House: On passage Passed by recorded vote", want: "passed-house"},
		{name: "empty action", action: "", want: "introduced"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := data.BillRecord{LatestAction: data.Action{Text: tt.action}}
			if got := ingest.StatusOf(rec); got != tt.want {
				t.Errorf("StatusOf(%q) = %q, want %q", tt.action, got, tt.want)
			}
		})
	}

	t.Run("search terms", func(t *testing.T) {
		t.Parallel()
		rec := data.BillRecord{
			Bill:     bill(4313),
			Title:    "Hospital Inpatient Services Act",
			Subjects: []string{"Medicare"},
		}
		want := []string{"4313", "act", "hospital", "hr4313", "inpatient", "medicare", "services"}
		if got := ingest.SearchTerms(rec); !slices.Equal(got, want) {
			t.Errorf("SearchTerms = %v, want %v", got, want)
		}
	})
}
