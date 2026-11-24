package gcs_test

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // comparing with the server-reported MD5
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"cloud.google.com/go/storage"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"
)

// The tests in this file check the Cloud Storage stand-in's behaviour that the
// gcs adapter relies on. They use the raw SDK client, not the adapter.

const (
	emuProject   = "pc-test-project"
	emuMIMEText  = "text/plain"
	emuMIMEJSON  = "application/json"
	emuMIMEXML   = "application/xml"
	emuMIMEBytes = "application/octet-stream"
	emuMiB       = 1 << 20
	emuPageSize  = 5
	emuPageCount = 23
)

// emuClient returns a raw SDK client for the stand-in, closed at cleanup.
func emuClient(t *testing.T) *storage.Client {
	t.Helper()
	requireEmulator(t)
	c, err := storage.NewClient(t.Context())
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// emuBucket creates a fresh bucket on the stand-in and returns its handle.
func emuBucket(t *testing.T) *storage.BucketHandle {
	t.Helper()
	bh := emuClient(t).Bucket(newBucketName(t))
	if err := bh.Create(t.Context(), emuProject, nil); err != nil {
		t.Fatalf("create bucket: %v", err)
	}
	return bh
}

// emuPut writes data to o with the given content type and returns the write error.
func emuPut(ctx context.Context, o *storage.ObjectHandle, data []byte, ct string) error {
	w := o.NewWriter(ctx)
	w.ContentType = ct
	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return fmt.Errorf("write object: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close object: %w", err)
	}
	return nil
}

// emuGet reads the whole object o.
func emuGet(ctx context.Context, o *storage.ObjectHandle) ([]byte, error) {
	r, err := o.NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("open object: %w", err)
	}
	defer func() { _ = r.Close() }()
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read object: %w", err)
	}
	return data, nil
}

// emuHTTPCode returns the HTTP status of a Google API error, or 0.
func emuHTTPCode(err error) int {
	if ge, ok := errors.AsType[*googleapi.Error](err); ok {
		return ge.Code
	}
	return 0
}

func TestEmulatorWriteRead(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	b := emuBucket(t)
	want := []byte("hello bill")
	if err := emuPut(ctx, b.Object("a/b.txt"), want, emuMIMEText); err != nil {
		t.Fatal(err)
	}
	got, err := emuGet(ctx, b.Object("a/b.txt"))
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestEmulatorByteForByte(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	b := emuBucket(t)
	want := append([]byte{0xEF, 0xBB, 0xBF}, []byte("<?xml version=\"1.0\"?>\r\n<bill>Café — § 2 \u65e5\u672c</bill>\r\n")...)
	if err := emuPut(ctx, b.Object("bills/x/v/1/source.xml"), want, emuMIMEXML); err != nil {
		t.Fatal(err)
	}
	got, err := emuGet(ctx, b.Object("bills/x/v/1/source.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(got) != sha256.Sum256(want) {
		t.Fatalf("sha mismatch: %x vs %x", got, want)
	}
}

func TestEmulatorWriteOnce(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	b := emuBucket(t)
	o := b.Object("once.json").If(storage.Conditions{DoesNotExist: true})
	if err := emuPut(ctx, o, []byte("v1"), emuMIMEJSON); err != nil {
		t.Fatalf("first create: %v", err)
	}
	err := emuPut(ctx, o, []byte("v2"), emuMIMEJSON)
	if err == nil {
		t.Fatal("second create succeeded; want 412")
	}
	if c := emuHTTPCode(err); c != http.StatusPreconditionFailed {
		t.Fatalf("code %d err %v; want 412", c, err)
	}
	got, err := emuGet(ctx, b.Object("once.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "v1" {
		t.Fatalf("content changed to %q", got)
	}
}

func TestEmulatorConcurrentCreate(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	b := emuBucket(t)
	const writers = 8
	var ok, fail412, other atomic.Int32
	var wg sync.WaitGroup
	for i := range writers {
		wg.Go(func() {
			o := b.Object("race.json").If(storage.Conditions{DoesNotExist: true})
			err := emuPut(ctx, o, fmt.Appendf(nil, "w%d", i), emuMIMEJSON)
			switch {
			case err == nil:
				ok.Add(1)
			case emuHTTPCode(err) == http.StatusPreconditionFailed:
				fail412.Add(1)
			default:
				other.Add(1)
				t.Logf("writer %d: %v", i, err)
			}
		})
	}
	wg.Wait()
	if ok.Load() != 1 || fail412.Load() != writers-1 || other.Load() != 0 {
		t.Fatalf("ok=%d 412=%d other=%d", ok.Load(), fail412.Load(), other.Load())
	}
}

func TestEmulatorGenerationMatch(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	b := emuBucket(t)
	o := b.Object("gen.json")
	if err := emuPut(ctx, o, []byte("v1"), emuMIMEJSON); err != nil {
		t.Fatal(err)
	}
	a1, err := o.Attrs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := emuPut(ctx, o.If(storage.Conditions{GenerationMatch: a1.Generation}), []byte("v2"), emuMIMEJSON); err != nil {
		t.Fatalf("match current: %v", err)
	}
	err = emuPut(ctx, o.If(storage.Conditions{GenerationMatch: a1.Generation}), []byte("v3"), emuMIMEJSON)
	if err == nil || emuHTTPCode(err) != http.StatusPreconditionFailed {
		t.Fatalf("stale generation: code %d err %v; want 412", emuHTTPCode(err), err)
	}
	got, err := emuGet(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "v2" {
		t.Fatalf("content %q", got)
	}
}

func TestEmulatorMetadata(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	b := emuBucket(t)
	o := b.Object("meta.xml")
	w := o.NewWriter(ctx)
	w.ContentType = emuMIMEXML
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
	emuCheckMetadata(t, a, w.Metadata)
	// The real service also returns Cache-Control and Content-Type on the media download.
	t.Run("ReaderHeaders", func(t *testing.T) {
		t.Parallel()
		r, err := o.NewReader(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = r.Close() }()
		if r.Attrs.ContentType != emuMIMEXML || r.Attrs.CacheControl != a.CacheControl {
			t.Fatalf("reader attrs content-type=%q cache-control=%q", r.Attrs.ContentType, r.Attrs.CacheControl)
		}
	})
}

// emuCheckMetadata compares the stored content type, cache control, and custom metadata with the written values.
func emuCheckMetadata(t *testing.T, a *storage.ObjectAttrs, meta map[string]string) {
	t.Helper()
	if a.ContentType != emuMIMEXML || a.CacheControl != "public, max-age=31536000, immutable" {
		t.Fatalf("attrs ct=%q cc=%q", a.ContentType, a.CacheControl)
	}
	for k, v := range meta {
		if a.Metadata[k] != v {
			t.Fatalf("metadata %s=%q want %q", k, a.Metadata[k], v)
		}
	}
}

// emuCheckSums compares the stored generation, MD5, and CRC32C with the written content.
func emuCheckSums(t *testing.T, a *storage.ObjectAttrs, data []byte) {
	t.Helper()
	m := md5.Sum(data) //nolint:gosec // see import
	if a.Generation == 0 || !bytes.Equal(a.MD5, m[:]) ||
		a.CRC32C != crc32.Checksum(data, crc32.MakeTable(crc32.Castagnoli)) {
		t.Fatalf("attrs %+v", a)
	}
}

func TestEmulatorChecksums(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	b := emuBucket(t)
	o := b.Object("sum.bin")
	d1 := []byte("first content")
	d2 := []byte("second content")
	if err := emuPut(ctx, o, d1, emuMIMEBytes); err != nil {
		t.Fatal(err)
	}
	a1, err := o.Attrs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	emuCheckSums(t, a1, d1)
	if err := emuPut(ctx, o, d2, emuMIMEBytes); err != nil {
		t.Fatal(err)
	}
	a2, err := o.Attrs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if a2.Generation == a1.Generation || bytes.Equal(a2.MD5, a1.MD5) || a2.CRC32C == a1.CRC32C {
		t.Fatalf("generation, MD5, or CRC32C did not change: %d/%d", a1.Generation, a2.Generation)
	}
}

func TestEmulatorNotFound(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	b := emuBucket(t)
	if _, err := emuGet(ctx, b.Object("missing")); !errors.Is(err, storage.ErrObjectNotExist) {
		t.Fatalf("read: %v", err)
	}
	if _, err := b.Object("missing").Attrs(ctx); !errors.Is(err, storage.ErrObjectNotExist) {
		t.Fatalf("attrs: %v", err)
	}
}

func TestEmulatorRangeRead(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	b := emuBucket(t)
	const size = 1000
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 251)
	}
	o := b.Object("range.bin")
	if err := emuPut(ctx, o, data, emuMIMEBytes); err != nil {
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

func TestEmulatorLargeObject(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	b := emuBucket(t)
	for _, c := range []struct {
		name  string
		size  int
		chunk int
	}{{"default-chunk-17MiB", 17 * emuMiB, 0}, {"1MiB-chunk-5MiB", 5 * emuMiB, emuMiB}} {
		data := make([]byte, c.size)
		if _, err := rand.Read(data); err != nil {
			t.Fatal(err)
		}
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
		got, err := emuGet(ctx, o)
		if err != nil || sha256.Sum256(got) != sha256.Sum256(data) {
			t.Fatalf("%s: round trip mismatch (len %d) err %v", c.name, len(got), err)
		}
	}
}

// emuListNames drains it and splits the results into object names and prefixes.
func emuListNames(t *testing.T, it *storage.ObjectIterator) (objs, prefs []string) {
	t.Helper()
	for {
		a, err := it.Next()
		if errors.Is(err, iterator.Done) {
			return objs, prefs
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
}

func TestEmulatorListing(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	b := emuBucket(t)
	for _, n := range []string{"p/top1", "p/top2", "p/x/1", "p/x/2", "p/y/1", "q/other"} {
		if err := emuPut(ctx, b.Object(n), []byte(n), emuMIMEText); err != nil {
			t.Fatal(err)
		}
	}
	objs, prefs := emuListNames(t, b.Objects(ctx, &storage.Query{Prefix: "p/", Delimiter: "/"}))
	if strings.Join(objs, ",") != "p/top1,p/top2" || strings.Join(prefs, ",") != "p/x/,p/y/" {
		t.Fatalf("objs %v prefixes %v", objs, prefs)
	}
	t.Run("Paging", func(t *testing.T) {
		t.Parallel()
		emuCheckPaging(ctx, t, b)
	})
}

// emuCheckPaging writes emuPageCount objects and pages through them emuPageSize at a time.
func emuCheckPaging(ctx context.Context, t *testing.T, b *storage.BucketHandle) {
	t.Helper()
	want := make([]string, 0, emuPageCount)
	for i := range emuPageCount {
		n := fmt.Sprintf("page/%02d", i)
		want = append(want, n)
		if err := emuPut(ctx, b.Object(n), []byte("x"), emuMIMEText); err != nil {
			t.Fatal(err)
		}
	}
	pages, got := emuDrainPages(t, iterator.NewPager(b.Objects(ctx, &storage.Query{Prefix: "page/"}), emuPageSize, ""))
	wantPages := (len(want) + emuPageSize - 1) / emuPageSize
	if pages != wantPages || !sort.StringsAreSorted(got) || strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("pages %d got %v", pages, got)
	}
}

// emuDrainPages reads every page and returns the page count and the object names in order.
func emuDrainPages(t *testing.T, pager *iterator.Pager) (int, []string) {
	t.Helper()
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
			return pages, got
		}
	}
}

func TestEmulatorDelete(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	b := emuBucket(t)
	o := b.Object("del")
	if err := emuPut(ctx, o, []byte("x"), emuMIMEText); err != nil {
		t.Fatal(err)
	}
	if err := o.Delete(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := emuGet(ctx, o); !errors.Is(err, storage.ErrObjectNotExist) {
		t.Fatalf("after delete: %v", err)
	}
	if err := o.Delete(ctx); !errors.Is(err, storage.ErrObjectNotExist) {
		t.Fatalf("second delete: %v", err)
	}
}
