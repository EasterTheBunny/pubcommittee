package fsdocs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/data/conformance"
)

var errInjected = errors.New("injected")

func newStore(t *testing.T, root string) *Store {
	t.Helper()
	s, err := New(Config{Root: root})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func TestDocs(t *testing.T) {
	t.Parallel()
	conformance.Docs(t, func(t *testing.T) (data.PublicDocs, data.PlatformDocs, data.Moderation) {
		t.Helper()
		s := newStore(t, t.TempDir())
		return s, s, s
	})
}

func TestNewRejectsEmptyRoot(t *testing.T) {
	t.Parallel()
	if _, err := New(Config{}); !errors.Is(err, data.ErrInvalid) {
		t.Errorf("err = %v, want ErrInvalid", err)
	}
}

func bill(n int) data.BillRef {
	return data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: n}
}

func TestApplyLoggedFailureLeavesNothing(t *testing.T) {
	t.Parallel()
	s := newStore(t, t.TempDir())
	ctx := context.Background()
	boom := errInjected
	s.failChange = func() error { return boom }
	b := data.Bill{Ref: bill(1), Title: "T"}
	e := data.LogEntry{ID: "log-1", At: time.Unix(1, 0).UTC(), Kind: "k"}
	if err := s.ApplyLogged(ctx, data.Change{Bill: &b}, e); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want injected error", err)
	}
	if _, err := s.GetBill(ctx, b.Ref); !errors.Is(err, data.ErrNotFound) {
		t.Errorf("bill after failed apply: err = %v, want ErrNotFound", err)
	}
	page, err := s.ListLog(ctx, data.Page{})
	if err != nil || len(page.Items) != 0 {
		t.Errorf("log after failed apply = %v, %v; want empty", page.Items, err)
	}
	// A retry without the failure applies both.
	s.failChange = nil
	if err := s.ApplyLogged(ctx, data.Change{Bill: &b}, e); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if _, err := s.GetBill(ctx, b.Ref); err != nil {
		t.Errorf("bill after retry: %v", err)
	}
}

func TestSecondStoreSeesWrites(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	w, r := newStore(t, root), newStore(t, root)
	ctx := context.Background()
	if err := w.PutBill(ctx, data.Bill{Ref: bill(7), Title: "Seven"}); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetBill(ctx, bill(7))
	if err != nil || got.Title != "Seven" {
		t.Errorf("reader got %+v, %v", got, err)
	}
}

func TestCatalogStableCursorsAcrossPages(t *testing.T) {
	t.Parallel()
	s := newStore(t, t.TempDir())
	ctx := context.Background()
	for _, n := range []int{10, 2, 33, 4, 5} {
		if err := s.PutCatalogEntry(ctx, data.CatalogEntry{Bill: bill(n), Title: strconv.Itoa(n)}); err != nil {
			t.Fatal(err)
		}
	}
	var got []int
	cursor := ""
	for range 10 {
		p, err := s.ListCatalog(ctx, data.Page{Cursor: cursor, Limit: 2})
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range p.Items {
			got = append(got, e.Bill.Number)
		}
		if p.Next == "" {
			break
		}
		cursor = p.Next
	}
	want := []int{2, 4, 5, 10, 33}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestCatalogScale(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("scale test")
	}
	const n = 15000
	s := newStore(t, t.TempDir())
	ctx := context.Background()
	// Seed the files directly: the cost under test is listing, not 15,000 fsyncs.
	if err := os.MkdirAll(filepath.Join(s.root, colCatalog), 0o750); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= n; i++ {
		body := []byte(`{"bill":{"jurisdiction":"us","session":"119","type":"hr","number":` + strconv.Itoa(i) + `},"title":"t"}`)
		if err := os.WriteFile(s.docPath(colCatalog, bill(i).String()), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	start := time.Now()
	p, err := s.ListCatalog(ctx, data.Page{Cursor: "", Limit: 50})
	if err != nil || len(p.Items) != 50 {
		t.Fatalf("page = %d items, %v", len(p.Items), err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("first page of %d entries took %v, want under 2s", n, d)
	}
	t.Logf("first page of %d entries: %v", n, time.Since(start))
}

// TestLegacyLogEntries: entries stored before day-only times and sequences
// still list, list after nothing newer than themselves is written, and an
// identical replay (as the generate step makes) is a no-op, not a conflict.
func TestLegacyLogEntries(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	s := newStore(t, root)
	ctx := context.Background()
	at := time.Date(2026, 3, 5, 14, 30, 0, 0, time.UTC)
	legacy := data.LogEntry{ID: "gen-1", At: at, Kind: "generation", Subject: "s", Reason: "r"}
	if err := s.write(colLog, legacy.ID, legacy); err != nil {
		t.Fatal(err)
	}
	if err := s.Record(ctx, legacy); err != nil {
		t.Fatalf("replay of a legacy entry: %v", err)
	}
	fresh := data.LogEntry{ID: "adm-1", At: at, Kind: "policy_published", Subject: "s", Reason: "r"}
	if err := s.Record(ctx, fresh); err != nil {
		t.Fatal(err)
	}
	page, err := s.ListLog(ctx, data.Page{})
	if err != nil || len(page.Items) != 2 || page.Items[0].ID != "adm-1" || page.Items[1].ID != "gen-1" {
		t.Fatalf("log = %+v, %v; want the new entry first, then the legacy one", page.Items, err)
	}
	if !page.Items[0].At.Equal(data.LogDay(at)) {
		t.Errorf("new entry At = %v, want the day only", page.Items[0].At)
	}
}
