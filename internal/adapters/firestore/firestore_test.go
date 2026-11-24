package firestore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/data/conformance"
)

var errInjected = errors.New("injected")

// requireEmulator skips the test unless FIRESTORE_EMULATOR_HOST names the
// emulator. CI sets PC_REQUIRE_EMULATORS=1, which turns a missing emulator into
// a failure, so a misconfigured job cannot pass by skipping.
func requireEmulator(t *testing.T) {
	t.Helper()
	if os.Getenv("FIRESTORE_EMULATOR_HOST") != "" {
		return
	}
	if os.Getenv("PC_REQUIRE_EMULATORS") == "1" {
		t.Fatal("FIRESTORE_EMULATOR_HOST is not set")
	}
	t.Skip("FIRESTORE_EMULATOR_HOST is not set; start the emulator profile (see compose.yaml)")
}

// newStore returns a Store on a project of its own: the emulator keeps the
// data of each project id apart, so every test starts empty.
func newStore(t *testing.T) *Store {
	t.Helper()
	requireEmulator(t)
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	s, err := New(t.Context(), Config{Project: "pc-test-" + hex.EncodeToString(b[:])})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestDocs(t *testing.T) {
	t.Parallel()
	conformance.Docs(t, func(t *testing.T) (data.PublicDocs, data.PlatformDocs, data.Moderation) {
		t.Helper()
		s := newStore(t)
		return s, s, s
	})
}

func TestNewRejectsBadConfig(t *testing.T) {
	t.Parallel()
	for _, cfg := range []Config{{}, {Project: "a b"}, {Project: "p", Database: "x/y"}} {
		if _, err := New(t.Context(), cfg); !errors.Is(err, data.ErrInvalid) {
			t.Errorf("New(%+v): err = %v, want ErrInvalid", cfg, err)
		}
	}
}

func bill(n int) data.BillRef {
	return data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: n}
}

// TestApplyLoggedLogFailureLeavesNothing injects a failure at the log write:
// the change was already queued in the transaction, and must not be applied.
func TestApplyLoggedLogFailureLeavesNothing(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	ctx := t.Context()
	s.failLog = func() error { return errInjected }
	b := data.Bill{Ref: bill(1), Title: "T"}
	e := data.LogEntry{ID: "log-1", At: time.Unix(1, 0).UTC(), Kind: "k"}
	if err := s.ApplyLogged(ctx, data.Change{Bill: &b}, e); !errors.Is(err, errInjected) {
		t.Fatalf("err = %v, want injected error", err)
	}
	if _, err := s.GetBill(ctx, b.Ref); !errors.Is(err, data.ErrNotFound) {
		t.Errorf("bill after failed apply: err = %v, want ErrNotFound", err)
	}
	page, err := s.ListLog(ctx, data.Page{})
	if err != nil || len(page.Items) != 0 {
		t.Errorf("log after failed apply = %v, %v; want empty", page.Items, err)
	}
	s.failLog = nil
	if err := s.ApplyLogged(ctx, data.Change{Bill: &b}, e); err != nil {
		t.Fatalf("retry after failure: %v", err)
	}
	if _, err := s.GetBill(ctx, b.Ref); err != nil {
		t.Errorf("bill after retry: %v", err)
	}
}

// TestApplyLoggedContention runs many transactions that all increment the log
// counter. Under contention Firestore may abort some after the client's
// retries: those writers get ErrConflict and their change is not applied.
// Nothing is lost or doubled: the log holds exactly the writers that
// succeeded, each with its own sequence number, and at least one succeeds.
func TestApplyLoggedContention(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	const n = 20
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := range n {
		wg.Go(func() {
			b := data.Bill{Ref: bill(i + 1), Title: "T"}
			e := data.LogEntry{ID: fmt.Sprintf("log-%02d", i), At: time.Unix(1, 0).UTC(), Kind: "k"}
			errs[i] = s.ApplyLogged(context.Background(), data.Change{Bill: &b}, e)
		})
	}
	wg.Wait()
	ok := contentionOutcomes(t, s, errs)
	if len(ok) == 0 {
		t.Fatal("no writer succeeded")
	}
	page, err := s.ListLog(t.Context(), data.Page{Limit: n})
	if err != nil || len(page.Items) != len(ok) {
		t.Fatalf("log = %d entries, %v; want %d (the writers that succeeded)", len(page.Items), err, len(ok))
	}
	seen := map[int64]bool{}
	for _, e := range page.Items {
		if !ok[e.ID] {
			t.Errorf("log entry %s belongs to a writer that failed", e.ID)
		}
		if seen[e.Seq] {
			t.Errorf("sequence %d used twice", e.Seq)
		}
		seen[e.Seq] = true
	}
}

// contentionOutcomes checks that each writer either succeeded with its bill
// stored, or got ErrConflict with nothing applied, and returns the log ids of
// the writers that succeeded.
func contentionOutcomes(t *testing.T, s *Store, errs []error) map[string]bool {
	t.Helper()
	ok := map[string]bool{}
	for i, err := range errs {
		_, getErr := s.GetBill(t.Context(), bill(i+1))
		switch {
		case err == nil:
			ok[fmt.Sprintf("log-%02d", i)] = true
			if getErr != nil {
				t.Errorf("writer %d succeeded but its bill is missing: %v", i, getErr)
			}
		case errors.Is(err, data.ErrConflict):
			if !errors.Is(getErr, data.ErrNotFound) {
				t.Errorf("writer %d got ErrConflict but its bill was applied: %v", i, getErr)
			}
		default:
			t.Errorf("writer %d: %v, want nil or ErrConflict", i, err)
		}
	}
	return ok
}

// TestOversizeDocumentIsInvalid: a document over Firestore's 1 MiB limit is
// refused with ErrInvalid, and nothing is stored.
func TestOversizeDocumentIsInvalid(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	b := data.Bill{Ref: bill(1), Title: strings.Repeat("x", 1<<21)}
	if err := s.PutBill(t.Context(), b); !errors.Is(err, data.ErrInvalid) {
		t.Fatalf("PutBill(2 MiB): err = %v, want ErrInvalid", err)
	}
	if _, err := s.GetBill(t.Context(), b.Ref); !errors.Is(err, data.ErrNotFound) {
		t.Errorf("GetBill after refused write: err = %v, want ErrNotFound", err)
	}
}

func TestPageLimitIsCapped(t *testing.T) {
	t.Parallel()
	if got, err := limitOf(data.Page{Limit: 1 << 30}); err != nil || got != maxLimit {
		t.Errorf("limitOf(huge) = %d, %v; want %d", got, err, maxLimit)
	}
	if _, err := limitOf(data.Page{Limit: -1}); !errors.Is(err, data.ErrInvalid) {
		t.Errorf("limitOf(-1): err = %v, want ErrInvalid", err)
	}
}

func TestDocIDs(t *testing.T) {
	t.Parallel()
	for _, id := range []string{docID("a/b"), docID("."), docID(".."), docID("__x__"), docID(""), billID(bill(7))} {
		if strings.Contains(id, "/") || id == "." || id == ".." || strings.HasPrefix(id, "__") || id == "" {
			t.Errorf("id %q is not a legal document id", id)
		}
	}
	if billID(bill(2)) >= billID(bill(10)) {
		t.Error("bill ids must sort numerically")
	}
}

func catalogEntry(n int, tracked bool, chamber string, terms ...string) data.CatalogEntry {
	return data.CatalogEntry{
		Bill: bill(n), Title: "title", Tracked: tracked, Reasons: []string{}, Chamber: chamber,
		SearchTerms: terms, Meta: conformance.SampleMeta(),
	}
}

func numbers(items []data.CatalogEntry) []int {
	out := make([]int, 0, len(items))
	for _, e := range items {
		out = append(out, e.Bill.Number)
	}
	return out
}

// TestQueryCatalogFilterAndKeyword pages through a filter plus keyword query
// and checks the pages are complete, ordered, and not repeated, even when an
// entry is added between pages.
func TestQueryCatalogFilterAndKeyword(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	ctx := t.Context()
	yes := true
	want := seedCatalog(t, s)
	q := CatalogQuery{Tracked: &yes, Chamber: data.ChamberHouse, Keyword: "TAXATION health"}
	got, pages := drainCatalog(t, s, q, func() {
		// A new matching entry arrives between pages.
		if err := s.PutCatalogEntry(ctx, catalogEntry(13, true, data.ChamberHouse, "taxation", "health")); err != nil {
			t.Fatal(err)
		}
		want = append(want, 13)
	})
	if !slices.Equal(got, want) {
		t.Errorf("matches = %v, want %v (in %d pages)", got, want, pages)
	}
	if _, err := s.QueryCatalog(ctx, q, data.Page{Cursor: "!!"}); !errors.Is(err, data.ErrInvalid) {
		t.Errorf("bad cursor: err = %v, want ErrInvalid", err)
	}
}

// seedCatalog stores bills 1 to 12 and returns the numbers a tracked, house,
// "taxation health" query must find: the even ones except 5 and the senate ones.
func seedCatalog(t *testing.T, s *Store) []int {
	t.Helper()
	var want []int
	for n := 1; n <= 12; n++ {
		chamber := data.ChamberHouse
		if n%3 == 0 {
			chamber = data.ChamberSenate
		}
		terms := []string{"hr" + strconv.Itoa(n), "Health care"}
		if n%2 == 0 {
			terms = append(terms, "Taxation")
		}
		if err := s.PutCatalogEntry(t.Context(), catalogEntry(n, n != 5, chamber, terms...)); err != nil {
			t.Fatal(err)
		}
		if n != 5 && chamber == data.ChamberHouse && n%2 == 0 {
			want = append(want, n)
		}
	}
	return want
}

// drainCatalog reads every page of q, two entries at a time; afterFirst runs
// once, after the first page.
func drainCatalog(t *testing.T, s *Store, q CatalogQuery, afterFirst func()) ([]int, int) {
	t.Helper()
	var got []int
	cursor := ""
	pages := 0
	for {
		page, err := s.QueryCatalog(t.Context(), q, data.Page{Cursor: cursor, Limit: 2})
		if err != nil {
			t.Fatal(err)
		}
		pages++
		got = append(got, numbers(page.Items)...)
		if pages == 1 {
			afterFirst()
		}
		if page.Next == "" {
			return got, pages
		}
		cursor = page.Next
	}
}

func TestSearchTerms(t *testing.T) {
	t.Parallel()
	got := searchTerms([]string{"Health Care", " HR1 ", "", "health"})
	want := []string{"care", "health", "health care", "hr1"}
	if !slices.Equal(got, want) {
		t.Errorf("searchTerms = %v, want %v", got, want)
	}
}
