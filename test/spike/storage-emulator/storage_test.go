package spike

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // comparing with the server-reported MD5
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	gax "github.com/googleapis/gax-go/v2"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// Tests 1-17 of design/tasks/m1/00-storage-emulator-exploration.md.
// Configured only through STORAGE_EMULATOR_HOST (and STAND_IN=testbench for test 14).

func newClient(t *testing.T, opts ...option.ClientOption) *storage.Client {
	t.Helper()
	if os.Getenv("STORAGE_EMULATOR_HOST") == "" {
		t.Fatal("STORAGE_EMULATOR_HOST not set")
	}
	c, err := storage.NewClient(context.Background(), opts...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func newBucket(t *testing.T, c *storage.Client, tag string) *storage.BucketHandle {
	t.Helper()
	var b [4]byte
	_, _ = rand.Read(b[:])
	name := fmt.Sprintf("m1-00-%s-%s", tag, hex.EncodeToString(b[:]))
	bh := c.Bucket(name)
	if err := bh.Create(context.Background(), "spike-project", nil); err != nil {
		t.Fatalf("create bucket: %v", err)
	}
	return bh
}

func put(ctx context.Context, o *storage.ObjectHandle, data []byte, ct string) error {
	w := o.NewWriter(ctx)
	w.ContentType = ct
	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return err
	}
	return w.Close()
}

func get(ctx context.Context, o *storage.ObjectHandle) ([]byte, error) {
	r, err := o.NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func httpCode(err error) int {
	var ge *googleapi.Error
	if errors.As(err, &ge) {
		return ge.Code
	}
	return 0
}

func TestT01WriteRead(t *testing.T) {
	ctx := context.Background()
	b := newBucket(t, newClient(t), "t01")
	want := []byte("hello bill")
	if err := put(ctx, b.Object("a/b.txt"), want, "text/plain"); err != nil {
		t.Fatal(err)
	}
	got, err := get(ctx, b.Object("a/b.txt"))
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestT02ByteForByte(t *testing.T) {
	ctx := context.Background()
	b := newBucket(t, newClient(t), "t02")
	want := append([]byte{0xEF, 0xBB, 0xBF}, []byte("<?xml version=\"1.0\"?>\r\n<bill>Café — § 2 日本</bill>\r\n")...)
	if err := put(ctx, b.Object("bills/x/v/1/source.xml"), want, "application/xml"); err != nil {
		t.Fatal(err)
	}
	got, err := get(ctx, b.Object("bills/x/v/1/source.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(got) != sha256.Sum256(want) {
		t.Fatalf("sha mismatch: %x vs %x", got, want)
	}
}

func TestT03T04WriteOnce(t *testing.T) {
	ctx := context.Background()
	b := newBucket(t, newClient(t), "t03")
	o := b.Object("once.json").If(storage.Conditions{DoesNotExist: true})
	if err := put(ctx, o, []byte("v1"), "application/json"); err != nil {
		t.Fatalf("T3 first create: %v", err)
	}
	err := put(ctx, o, []byte("v2"), "application/json")
	if err == nil {
		t.Fatal("T4 second create succeeded; want 412")
	}
	if c := httpCode(err); c != http.StatusPreconditionFailed {
		t.Fatalf("T4 code %d err %v; want 412", c, err)
	}
	got, _ := get(ctx, b.Object("once.json"))
	if string(got) != "v1" {
		t.Fatalf("content changed to %q", got)
	}
}

func TestT05ConcurrentCreate(t *testing.T) {
	ctx := context.Background()
	b := newBucket(t, newClient(t), "t05")
	const n = 8
	var ok, fail412, other atomic.Int32
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			o := b.Object("race.json").If(storage.Conditions{DoesNotExist: true})
			err := put(ctx, o, fmt.Appendf(nil, "w%d", i), "application/json")
			switch {
			case err == nil:
				ok.Add(1)
			case httpCode(err) == 412:
				fail412.Add(1)
			default:
				other.Add(1)
				t.Logf("writer %d: %v", i, err)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 1 || fail412.Load() != n-1 || other.Load() != 0 {
		t.Fatalf("ok=%d 412=%d other=%d", ok.Load(), fail412.Load(), other.Load())
	}
}

func TestT06GenerationMatch(t *testing.T) {
	ctx := context.Background()
	b := newBucket(t, newClient(t), "t06")
	o := b.Object("gen.json")
	if err := put(ctx, o, []byte("v1"), "application/json"); err != nil {
		t.Fatal(err)
	}
	a1, err := o.Attrs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := put(ctx, o.If(storage.Conditions{GenerationMatch: a1.Generation}), []byte("v2"), "application/json"); err != nil {
		t.Fatalf("match current: %v", err)
	}
	err = put(ctx, o.If(storage.Conditions{GenerationMatch: a1.Generation}), []byte("v3"), "application/json")
	if err == nil || httpCode(err) != 412 {
		t.Fatalf("stale generation: code %d err %v; want 412", httpCode(err), err)
	}
	got, _ := get(ctx, o)
	if string(got) != "v2" {
		t.Fatalf("content %q", got)
	}
}

func TestT07Metadata(t *testing.T) {
	ctx := context.Background()
	b := newBucket(t, newClient(t), "t07")
	o := b.Object("meta.xml")
	w := o.NewWriter(ctx)
	w.ContentType = "application/xml"
	w.CacheControl = "public, max-age=31536000, immutable"
	w.Metadata = map[string]string{
		"source-url":   "https://example.invalid/BILLS-1.xml",
		"retrieved-at": "2026-11-23T00:00:00Z",
		"sha256":       strings.Repeat("ab", 32),
	}
	if _, err := w.Write([]byte("<x/>")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	a, err := o.Attrs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if a.ContentType != "application/xml" || a.CacheControl != "public, max-age=31536000, immutable" {
		t.Fatalf("attrs ct=%q cc=%q", a.ContentType, a.CacheControl)
	}
	for k, v := range w.Metadata {
		if a.Metadata[k] != v {
			t.Fatalf("metadata %s=%q want %q", k, a.Metadata[k], v)
		}
	}
	// The real service also returns Cache-Control and Content-Type on the media download.
	t.Run("ReaderHeaders", func(t *testing.T) {
		r, err := o.NewReader(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		if r.Attrs.ContentType != "application/xml" || r.Attrs.CacheControl != a.CacheControl {
			t.Fatalf("reader attrs content-type=%q cache-control=%q", r.Attrs.ContentType, r.Attrs.CacheControl)
		}
	})
}

func TestT08Checksums(t *testing.T) {
	ctx := context.Background()
	b := newBucket(t, newClient(t), "t08")
	o := b.Object("sum.bin")
	d1 := []byte("first content")
	d2 := []byte("second content")
	if err := put(ctx, o, d1, "application/octet-stream"); err != nil {
		t.Fatal(err)
	}
	a1, err := o.Attrs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	m := md5.Sum(d1) //nolint:gosec // see import
	if a1.Generation == 0 || a1.Etag == "" || !bytes.Equal(a1.MD5, m[:]) ||
		a1.CRC32C != crc32.Checksum(d1, crc32.MakeTable(crc32.Castagnoli)) {
		t.Fatalf("attrs %+v", a1)
	}
	if err := put(ctx, o, d2, "application/octet-stream"); err != nil {
		t.Fatal(err)
	}
	a2, _ := o.Attrs(ctx)
	if a2.Generation == a1.Generation || bytes.Equal(a2.MD5, a1.MD5) || a2.CRC32C == a1.CRC32C {
		t.Fatalf("generation, MD5, or CRC32C did not change: %d/%d", a1.Generation, a2.Generation)
	}
	// The real service changes the ETag with generation and metageneration.
	t.Run("ETagChanges", func(t *testing.T) {
		if a2.Etag == a1.Etag {
			t.Fatalf("ETag unchanged after rewrite: %q (generation %d -> %d)", a1.Etag, a1.Generation, a2.Generation)
		}
	})
}

func TestT09NotFound(t *testing.T) {
	ctx := context.Background()
	b := newBucket(t, newClient(t), "t09")
	if _, err := get(ctx, b.Object("missing")); !errors.Is(err, storage.ErrObjectNotExist) {
		t.Fatalf("read: %v", err)
	}
	if _, err := b.Object("missing").Attrs(ctx); !errors.Is(err, storage.ErrObjectNotExist) {
		t.Fatalf("attrs: %v", err)
	}
}

func TestT10RangeRead(t *testing.T) {
	ctx := context.Background()
	b := newBucket(t, newClient(t), "t10")
	data := make([]byte, 1000)
	for i := range data {
		data[i] = byte(i % 251)
	}
	o := b.Object("range.bin")
	if err := put(ctx, o, data, "application/octet-stream"); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name     string
		off, len int64
		want     []byte
	}{
		{"middle", 100, 50, data[100:150]},
		{"to end", 900, -1, data[900:]},
		{"suffix", -10, -1, data[990:]},
		{"first", 0, 1, data[:1]},
	} {
		r, err := o.NewRangeReader(ctx, c.off, c.len)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		got, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil || !bytes.Equal(got, c.want) {
			t.Fatalf("%s: got %d bytes err %v", c.name, len(got), err)
		}
	}
}

func TestT11LargeObject(t *testing.T) {
	ctx := context.Background()
	b := newBucket(t, newClient(t), "t11")
	for _, c := range []struct {
		name  string
		size  int
		chunk int
	}{{"default-chunk-17MiB", 17 << 20, 0}, {"1MiB-chunk-5MiB", 5 << 20, 1 << 20}} {
		data := make([]byte, c.size)
		_, _ = rand.Read(data)
		o := b.Object(c.name)
		w := o.NewWriter(ctx)
		if c.chunk > 0 {
			w.ChunkSize = c.chunk
		}
		if _, err := w.Write(data); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("%s close: %v", c.name, err)
		}
		got, err := get(ctx, o)
		if err != nil || sha256.Sum256(got) != sha256.Sum256(data) {
			t.Fatalf("%s: round trip mismatch (len %d) err %v", c.name, len(got), err)
		}
	}
}

func TestT12Listing(t *testing.T) {
	ctx := context.Background()
	b := newBucket(t, newClient(t), "t12")
	for _, n := range []string{"p/top1", "p/top2", "p/x/1", "p/x/2", "p/y/1", "q/other"} {
		if err := put(ctx, b.Object(n), []byte(n), "text/plain"); err != nil {
			t.Fatal(err)
		}
	}
	it := b.Objects(ctx, &storage.Query{Prefix: "p/", Delimiter: "/"})
	var objs, prefs []string
	for {
		a, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if a.Prefix != "" {
			prefs = append(prefs, a.Prefix)
		} else {
			objs = append(objs, a.Name)
		}
	}
	if strings.Join(objs, ",") != "p/top1,p/top2" || strings.Join(prefs, ",") != "p/x/,p/y/" {
		t.Fatalf("objs %v prefixes %v", objs, prefs)
	}
	t.Run("Paging", func(t *testing.T) {
		// Paging across several pages with tokens.
		var want []string
		for i := range 23 {
			n := fmt.Sprintf("page/%02d", i)
			want = append(want, n)
			if err := put(ctx, b.Object(n), []byte("x"), "text/plain"); err != nil {
				t.Fatal(err)
			}
		}
		it = b.Objects(ctx, &storage.Query{Prefix: "page/"})
		pager := iterator.NewPager(it, 5, "")
		var got []string
		pages := 0
		for {
			var batch []*storage.ObjectAttrs
			tok, err := pager.NextPage(&batch)
			if err != nil {
				t.Fatal(err)
			}
			pages++
			for _, a := range batch {
				got = append(got, a.Name)
			}
			if tok == "" {
				break
			}
		}
		if pages != 5 || !sort.StringsAreSorted(got) || strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("pages %d got %v", pages, got)
		}
	})
}

func TestT13Delete(t *testing.T) {
	ctx := context.Background()
	b := newBucket(t, newClient(t), "t13")
	o := b.Object("del")
	if err := put(ctx, o, []byte("x"), "text/plain"); err != nil {
		t.Fatal(err)
	}
	if err := o.Delete(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := get(ctx, o); !errors.Is(err, storage.ErrObjectNotExist) {
		t.Fatalf("after delete: %v", err)
	}
	if err := o.Delete(ctx); !errors.Is(err, storage.ErrObjectNotExist) {
		t.Fatalf("second delete: %v", err)
	}
}

// countingTransport adds the testbench retry-test id header and counts requests.
type countingTransport struct {
	id    string
	calls atomic.Int32
}

func (c *countingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c.calls.Add(1)
	r = r.Clone(r.Context())
	if c.id != "" {
		r.Header.Set("x-retry-test-id", c.id)
	}
	return http.DefaultTransport.RoundTrip(r)
}

func retryTest(t *testing.T, instr map[string][]string) string {
	t.Helper()
	host := os.Getenv("STORAGE_EMULATOR_HOST")
	body, _ := json.Marshal(map[string]any{"instructions": instr})
	resp, err := http.Post("http://"+host+"/retry_test", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("retry_test: %d %s", resp.StatusCode, b)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.ID == "" {
		t.Fatalf("retry_test response: %v", err)
	}
	return out.ID
}

func TestT14TransientErrors(t *testing.T) {
	if os.Getenv("STAND_IN") != "testbench" {
		t.Skip("fault injection exists only on the testbench (STAND_IN=testbench)")
	}
	ctx := context.Background()
	b := newBucket(t, newClient(t), "t14")
	if err := put(ctx, b.Object("r"), []byte("data"), "text/plain"); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name  string
		instr []string
	}{
		{"503 x2 then success", []string{"return-503", "return-503"}},
		{"connection reset x2 then success", []string{"return-reset-connection", "return-reset-connection"}},
	} {
		tr := &countingTransport{id: retryTest(t, map[string][]string{"storage.objects.get": c.instr})}
		cl := newClient(t, option.WithHTTPClient(&http.Client{Transport: tr}))
		got, err := get(ctx, cl.Bucket(b.BucketName()).Object("r"))
		if err != nil || string(got) != "data" {
			t.Fatalf("%s: got %q err %v (calls %d)", c.name, got, err, tr.calls.Load())
		}
		t.Logf("%s: %d calls", c.name, tr.calls.Load())
		if tr.calls.Load() < 2 {
			t.Fatalf("%s: only %d calls; client did not retry", c.name, tr.calls.Load())
		}
	}
	// Retries exhausted: the error is returned and carries 503 for the ErrUnavailable mapping.
	tr := &countingTransport{id: retryTest(t, map[string][]string{"storage.objects.get": {"return-503", "return-503", "return-503", "return-503"}})}
	cl := newClient(t, option.WithHTTPClient(&http.Client{Transport: tr}))
	o := cl.Bucket(b.BucketName()).Object("r").Retryer(storage.WithMaxAttempts(2), storage.WithBackoff(gax.Backoff{Initial: 10 * time.Millisecond, Max: 20 * time.Millisecond}))
	if _, err := get(ctx, o); err == nil || httpCode(err) != http.StatusServiceUnavailable {
		t.Fatalf("exhausted: code %d err %v; want 503", httpCode(err), err)
	}
	// An unconditional write is not idempotent: the client does not retry it by default.
	tr = &countingTransport{id: retryTest(t, map[string][]string{"storage.objects.insert": {"return-503"}})}
	cl = newClient(t, option.WithHTTPClient(&http.Client{Transport: tr}))
	err := put(ctx, cl.Bucket(b.BucketName()).Object("w"), []byte("x"), "text/plain")
	t.Logf("unconditional write under one 503: err=%v calls=%d", err, tr.calls.Load())
	// A DoesNotExist write is idempotent, so the client retries it.
	tr = &countingTransport{id: retryTest(t, map[string][]string{"storage.objects.insert": {"return-503"}})}
	cl = newClient(t, option.WithHTTPClient(&http.Client{Transport: tr}))
	err = put(ctx, cl.Bucket(b.BucketName()).Object("w2").If(storage.Conditions{DoesNotExist: true}), []byte("x"), "text/plain")
	if err != nil {
		t.Fatalf("conditional write under one 503: %v (calls %d)", err, tr.calls.Load())
	}
}

func TestT15RaceMarker(t *testing.T) {
	// Test 15 is the whole suite run with `go test -race`; run.sh does that. This only logs.
	t.Log("run the suite with -race; see run.sh")
}

func TestT16FirstRequest(t *testing.T) {
	// Container start to first request is measured by run.sh; here: client plus first bucket create.
	start := time.Now()
	b := newBucket(t, newClient(t), "t16")
	t.Logf("client + bucket create: %s (%s)", time.Since(start), b.BucketName())
}

// badAuth sends an invalid bearer token. The real service refuses the write with 401 or 403.
type badAuth struct{}

func (badAuth) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer not-a-real-token")
	return http.DefaultTransport.RoundTrip(r)
}

func TestT17Permissions(t *testing.T) {
	ctx := context.Background()
	b := newBucket(t, newClient(t), "t17")
	cl := newClient(t, option.WithHTTPClient(&http.Client{Transport: badAuth{}}))
	err := put(ctx, cl.Bucket(b.BucketName()).Object("denied"), []byte("x"), "text/plain")
	if err == nil {
		t.Fatal("write with invalid credential was accepted; real service returns 401/403")
	}
	if c := httpCode(err); c != http.StatusUnauthorized && c != http.StatusForbidden {
		t.Fatalf("code %d err %v", c, err)
	}
}
