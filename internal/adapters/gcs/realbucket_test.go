package gcs_test

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/adapters/gcs"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

const (
	realRaceWriters  = 16
	realListWrites   = 30
	realPrefixBytes  = 4
	realETagHexChars = 64
	realCacheImmut   = "public, max-age=31536000, immutable"
	// realRaceBodies holds one distinct single-byte body per racing writer.
	realRaceBodies = "abcdefghijklmnop"
)

// realETag matches a strong ETag: 64 lowercase hex characters (SHA-256).
var realETag = regexp.MustCompile(`^[0-9a-f]{64}$`)

// realBucket returns the bucket named by PC_GCS_TEST_BUCKET. It skips the
// test unless that variable is set and STORAGE_EMULATOR_HOST is empty, so
// these tests only ever run against a real Cloud Storage bucket. Credentials
// are the Application Default Credentials the operator provides; this file
// never sets or reads any credential.
func realBucket(t *testing.T) string {
	t.Helper()
	bucket := os.Getenv("PC_GCS_TEST_BUCKET")
	if bucket == "" || os.Getenv("STORAGE_EMULATOR_HOST") != "" {
		t.Skip("real-bucket test skipped: set PC_GCS_TEST_BUCKET to an existing bucket " +
			"and leave STORAGE_EMULATOR_HOST empty (credentials come from Application Default Credentials)")
	}
	return bucket
}

// realPrefix returns a fresh object prefix "realtest/<random hex>/", so runs
// never collide. The tests write under it and never delete anything.
func realPrefix(t *testing.T) string {
	t.Helper()
	var b [realPrefixBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	return "realtest/" + hex.EncodeToString(b[:]) + "/"
}

// realStore returns a Store on the real bucket and a fresh prefix for the test.
func realStore(t *testing.T) (*gcs.Store, string) {
	t.Helper()
	bucket := realBucket(t)
	s, err := gcs.New(t.Context(), gcs.Config{Bucket: bucket})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, realPrefix(t)
}

// badAuth replaces the request's credentials with a token that is not valid,
// so the service answers 401 and the adapter must map it to ErrUnavailable.
type badAuth struct {
	next http.RoundTripper
}

func (b badAuth) RoundTrip(r *http.Request) (*http.Response, error) {
	clone := r.Clone(r.Context())
	clone.Header.Set("Authorization", "Bearer not-a-real-token")
	resp, err := b.next.RoundTrip(clone)
	if err != nil {
		return nil, fmt.Errorf("bad-token round trip: %w", err)
	}
	return resp, nil
}

func TestRealBucketETagChangesWithContent(t *testing.T) {
	t.Parallel()
	s, prefix := realStore(t)
	ctx := t.Context()
	p := prefix + "etag.json"

	if _, err := s.PutReplaceable(ctx, p, []byte("one")); err != nil {
		t.Fatalf("first PutReplaceable: %v", err)
	}
	first, err := s.Stat(ctx, p)
	if err != nil {
		t.Fatalf("first Stat: %v", err)
	}
	if _, err := s.PutReplaceable(ctx, p, []byte("two")); err != nil {
		t.Fatalf("second PutReplaceable: %v", err)
	}
	second, err := s.Stat(ctx, p)
	if err != nil {
		t.Fatalf("second Stat: %v", err)
	}

	if first.ETag == second.ETag {
		t.Errorf("ETag did not change with content: both %q", first.ETag)
	}
	if !realETag.MatchString(first.ETag) || !realETag.MatchString(second.ETag) {
		t.Errorf("ETags %q and %q; want %d lowercase hex characters",
			first.ETag, second.ETag, realETagHexChars)
	}
	if second.Generation <= first.Generation {
		t.Errorf("Generation %d then %d; want an increase", first.Generation, second.Generation)
	}
}

func TestRealBucketConcurrentCreateOneWinner(t *testing.T) {
	t.Parallel()
	s, prefix := realStore(t)
	ctx := t.Context()
	p := prefix + "race.txt"

	var wins, conflicts atomic.Int32
	var wg sync.WaitGroup
	for i := range realRaceWriters {
		wg.Go(func() {
			err := s.PutImmutable(ctx, p, []byte(realRaceBodies[i:i+1]))
			switch {
			case err == nil:
				wins.Add(1)
			case errors.Is(err, data.ErrConflict):
				conflicts.Add(1)
			default:
				t.Errorf("writer %d: unexpected error: %v", i, err)
			}
		})
	}
	wg.Wait()

	if wins.Load() != 1 || conflicts.Load() != realRaceWriters-1 {
		t.Errorf("wins %d, conflicts %d; want 1 and %d",
			wins.Load(), conflicts.Load(), realRaceWriters-1)
	}
}

func TestRealBucketListAfterWrite(t *testing.T) {
	t.Parallel()
	s, prefix := realStore(t)
	ctx := t.Context()

	// Store has no generic list method, so this checks read-after-write:
	// each object is readable at once after its PutImmutable returns.
	for n := 1; n <= realListWrites; n++ {
		p := fmt.Sprintf("%slist/%d.json", prefix, n)
		want := []byte(fmt.Sprintf(`{"n":%d}`, n))
		if err := s.PutImmutable(ctx, p, want); err != nil {
			t.Fatalf("PutImmutable %d: %v", n, err)
		}
		got, err := s.GetFile(ctx, p)
		if err != nil {
			t.Fatalf("GetFile %d right after write: %v", n, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("GetFile %d = %q, want %q", n, got, want)
		}
	}
}

func TestRealBucketPermissionDenied(t *testing.T) {
	t.Parallel()
	bucket := realBucket(t)
	s, err := gcs.New(t.Context(), gcs.Config{
		Bucket:    bucket,
		Transport: badAuth{next: http.DefaultTransport},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	// Read-only: no write access is needed, the request fails before any read.
	_, err = s.GetFile(t.Context(), realPrefix(t)+"x")
	if !errors.Is(err, data.ErrUnavailable) {
		t.Errorf("GetFile with bad token: err = %v, want ErrUnavailable", err)
	}
	if errors.Is(err, data.ErrNotFound) {
		t.Errorf("GetFile with bad token: err = %v, must not be ErrNotFound", err)
	}
}

func TestRealBucketWriteOnceSurvivesReplaceable(t *testing.T) {
	t.Parallel()
	s, prefix := realStore(t)
	ctx := t.Context()

	immutablePath := prefix + "once-then-replace.txt"
	if err := s.PutImmutable(ctx, immutablePath, []byte("first")); err != nil {
		t.Fatalf("PutImmutable: %v", err)
	}
	if _, err := s.PutReplaceable(ctx, immutablePath, []byte("second")); !errors.Is(err, data.ErrConflict) {
		t.Errorf("PutReplaceable over immutable: err = %v, want ErrConflict", err)
	}

	replaceablePath := prefix + "replace-then-once.txt"
	if _, err := s.PutReplaceable(ctx, replaceablePath, []byte("first")); err != nil {
		t.Fatalf("PutReplaceable: %v", err)
	}
	if err := s.PutImmutable(ctx, replaceablePath, []byte("second")); !errors.Is(err, data.ErrConflict) {
		t.Errorf("PutImmutable over replaceable: err = %v, want ErrConflict", err)
	}
}

func TestRealBucketCacheControlOnImmutable(t *testing.T) {
	t.Parallel()
	s, prefix := realStore(t)
	ctx := t.Context()
	p := prefix + "cache.json"

	if err := s.PutImmutable(ctx, p, []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("PutImmutable: %v", err)
	}
	info, err := s.Stat(ctx, p)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.Immutable {
		t.Errorf("Immutable = false, want true")
	}
	if info.CacheControl != realCacheImmut {
		t.Errorf("CacheControl = %q, want %q", info.CacheControl, realCacheImmut)
	}
	if info.ContentType != "application/json" {
		t.Errorf("ContentType = %q, want application/json", info.ContentType)
	}
}
