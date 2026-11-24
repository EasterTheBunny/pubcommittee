package gcs_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/adapters/gcs"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/data/conformance"
)

// requireEmulator skips the test unless STORAGE_EMULATOR_HOST names the Cloud
// Storage stand-in. CI sets PC_REQUIRE_EMULATORS=1, which turns a missing
// stand-in into a failure, so a misconfigured job cannot pass by skipping.
func requireEmulator(t *testing.T) {
	t.Helper()
	if os.Getenv("STORAGE_EMULATOR_HOST") != "" {
		return
	}
	if os.Getenv("PC_REQUIRE_EMULATORS") == "1" {
		t.Fatal("STORAGE_EMULATOR_HOST is not set")
	}
	t.Skip("STORAGE_EMULATOR_HOST is not set; start the emulator profile (see compose.yaml)")
}

// newBucketName returns a fresh bucket name, so every test has its own namespace.
func newBucketName(t *testing.T) string {
	t.Helper()
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	return "pc-test-" + hex.EncodeToString(b[:])
}

// newStore returns a Store on a new bucket of the stand-in.
func newStore(t *testing.T) *gcs.Store {
	t.Helper()
	requireEmulator(t)
	s, err := gcs.New(t.Context(), gcs.Config{Bucket: newBucketName(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.EnsureBucket(t.Context(), "pc-test-project"); err != nil {
		t.Fatalf("EnsureBucket: %v", err)
	}
	return s
}

func TestBillTextAndPublisher(t *testing.T) {
	t.Parallel()
	conformance.BillTextAndPublisher(t, func(t *testing.T) (data.BillText, data.Publisher) {
		t.Helper()
		s := newStore(t)
		return s, s
	})
}

func TestSourceProvenance(t *testing.T) {
	t.Parallel()
	conformance.SourceProvenance(t, func(t *testing.T) (data.Publisher, data.SourceInfoReader) {
		t.Helper()
		s := newStore(t)
		return s, s
	})
}

func TestGenerated(t *testing.T) {
	t.Parallel()
	conformance.Generated(t, func(t *testing.T) (data.GeneratedReader, data.GeneratedWriter) {
		t.Helper()
		s := data.NewGeneratedStore(newStore(t))
		return s, s
	})
}

func TestComparisons(t *testing.T) {
	t.Parallel()
	conformance.Comparisons(t, func(t *testing.T) (data.ComparisonReader, data.ComparisonWriter) {
		t.Helper()
		s := newStore(t)
		return s, s
	})
}

func TestExports(t *testing.T) {
	t.Parallel()
	conformance.Exports(t, func(t *testing.T) (data.ExportReader, data.ExportWriter) {
		t.Helper()
		s := newStore(t)
		return s, s
	})
}

func TestFeed(t *testing.T) {
	t.Parallel()
	conformance.Feed(t, func(t *testing.T) (data.FeedReader, data.FeedWriter) {
		t.Helper()
		s := newStore(t)
		return s, s
	})
}

func TestLaw(t *testing.T) {
	t.Parallel()
	conformance.Law(t, func(t *testing.T) (data.LawReader, data.LawWriter) {
		t.Helper()
		s := newStore(t)
		return s, s
	})
}

func TestCitations(t *testing.T) {
	t.Parallel()
	conformance.Citations(t, func(t *testing.T) (data.CitationReader, data.CitationWriter) {
		t.Helper()
		s := newStore(t)
		return s, s
	})
}

func TestNewRejectsBadBucket(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"", "A", "ab", "has space", "-lead", "trail-", "UPPER"} {
		if _, err := gcs.New(t.Context(), gcs.Config{Bucket: name}); !errors.Is(err, data.ErrInvalid) {
			t.Errorf("New(%q): err = %v, want ErrInvalid", name, err)
		}
	}
}

func TestRejectsEscapingPaths(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	for _, p := range []string{"", "../x", "a/../../x", "/abs", "a//b", "a/", "a\\b", "a/./b", "x\x00y"} {
		if err := s.PutImmutable(t.Context(), p, []byte("x")); !errors.Is(err, data.ErrInvalid) {
			t.Errorf("PutImmutable(%q): err = %v, want ErrInvalid", p, err)
		}
		if _, err := s.GetFile(t.Context(), p); !errors.Is(err, data.ErrInvalid) {
			t.Errorf("GetFile(%q): err = %v, want ErrInvalid", p, err)
		}
	}
}

// TestSecondWriteIsConflict is the write-once acceptance check: a second write
// of different bytes to an immutable path fails with ErrConflict, and the
// first content stays.
func TestSecondWriteIsConflict(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	ctx := t.Context()
	const p = "exports/once.txt"
	if err := s.PutImmutable(ctx, p, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := s.PutImmutable(ctx, p, []byte("second")); !errors.Is(err, data.ErrConflict) {
		t.Fatalf("second write: err = %v, want ErrConflict", err)
	}
	if got, err := s.GetFile(ctx, p); err != nil || string(got) != "first" {
		t.Errorf("content = %q, %v; want first", got, err)
	}
}

func TestStatAndETag(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	ctx := t.Context()
	etag, err := s.PutReplaceable(ctx, "feeds/us.atom", []byte("v1"))
	if err != nil {
		t.Fatal(err)
	}
	info, err := s.Stat(ctx, "feeds/us.atom")
	if err != nil || info.ETag != etag || info.Immutable || info.Generation == 0 || info.Size != 2 {
		t.Errorf("Stat = %+v, %v; want etag %q, replaceable, size 2", info, err, etag)
	}
	if _, err := s.Stat(ctx, "exports/missing"); !errors.Is(err, data.ErrNotFound) {
		t.Errorf("Stat missing: err = %v, want ErrNotFound", err)
	}
}

func TestStatImmutable(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	ctx := t.Context()
	if err := s.PutImmutable(ctx, "exports/a.txt", []byte("x")); err != nil {
		t.Fatal(err)
	}
	info, err := s.Stat(ctx, "exports/a.txt")
	if err != nil || !info.Immutable || info.CacheControl == "" || info.ContentType != "text/plain; charset=utf-8" {
		t.Errorf("Stat immutable = %+v, %v", info, err)
	}
}

func TestReplaceableIfStaleWriter(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	ctx := t.Context()
	if _, err := s.PutReplaceableIf(ctx, "feeds/a", []byte("1"), ""); err != nil {
		t.Fatalf("create with empty ifMatch: %v", err)
	}
	if _, err := s.PutReplaceableIf(ctx, "feeds/a", []byte("2"), ""); !errors.Is(err, data.ErrConflict) {
		t.Errorf("stale create: err = %v, want ErrConflict", err)
	}
	etag, err := s.ETag(ctx, "feeds/a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PutReplaceableIf(ctx, "feeds/a", []byte("2"), etag); err != nil {
		t.Errorf("fresh writer: %v", err)
	}
	if _, err := s.PutReplaceableIf(ctx, "feeds/a", []byte("3"), etag); !errors.Is(err, data.ErrConflict) {
		t.Errorf("stale writer: err = %v, want ErrConflict", err)
	}
}

// TestConcurrentCreateOneWinner races writers of different bytes at one
// write-once path: exactly one wins, the rest get ErrConflict.
func TestConcurrentCreateOneWinner(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	const writers = 8
	var wins, conflicts, other atomic.Int32
	var wg sync.WaitGroup
	for i := range writers {
		wg.Go(func() {
			err := s.PutImmutable(context.Background(), "exports/race.txt", fmt.Appendf(nil, "writer-%d", i))
			switch {
			case err == nil:
				wins.Add(1)
			case errors.Is(err, data.ErrConflict):
				conflicts.Add(1)
			default:
				other.Add(1)
				t.Errorf("writer %d: %v", i, err)
			}
		})
	}
	wg.Wait()
	if wins.Load() != 1 || conflicts.Load() != writers-1 || other.Load() != 0 {
		t.Errorf("wins %d, conflicts %d, other %d; want 1, %d, 0", wins.Load(), conflicts.Load(), other.Load(), writers-1)
	}
}

// TestOriginalXMLByteForByte stores XML with a byte-order mark, CRLF, and
// non-ASCII text and reads the same bytes back, with the SHA-256 kept as the
// ETag of the original.
func TestOriginalXMLByteForByte(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	ctx := t.Context()
	src := append([]byte{0xEF, 0xBB, 0xBF}, []byte("<?xml version=\"1.0\"?>\r\n<bill>Café — § 2 \u65e5\u672c</bill>\r\n")...)
	v := conformance.SampleVersion(1, "ih")
	if err := s.PutVersion(ctx, v, src); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetSource(ctx, v.Ref)
	if err != nil || string(got) != string(src) {
		t.Fatalf("GetSource = %q, %v", got, err)
	}
	etag, err := s.ETag(ctx, "bills/"+v.Ref.Bill.String()+"/v/ih/source.xml")
	if err != nil || len(etag) != 64 {
		t.Errorf("ETag = %q, %v; want 64 hex characters", etag, err)
	}
}
