package httpx

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// MinGzipBytes is the smallest body with a known length that Gzip encodes:
// below it, compression does not pay.
const MinGzipBytes = 1024

// gzipLevel trades a little size for speed on every request.
const gzipLevel = 5

// gzipTagSuffix marks the strong ETag of a gzip-encoded response.
const gzipTagSuffix = "-gzip"

// newGzipPool returns a pool of gzip writers at gzipLevel.
func newGzipPool() *sync.Pool {
	return &sync.Pool{New: func() any {
		w, err := gzip.NewWriterLevel(nil, gzipLevel)
		if err != nil {
			panic(fmt.Sprintf("gzip level %d: %v", gzipLevel, err)) // a constant level never fails
		}
		return w
	}}
}

// Gzip encodes text and JSON responses for clients that accept gzip. The edge
// in front of Cloud Run passes responses through unchanged, so without it a
// long bill's JSON goes out at full size (M1-16 page weight). Responses that
// are already encoded, not text, smaller than MinGzipBytes, partial, empty,
// or marked no-transform are sent as they are. Every response varies on
// Accept-Encoding. An encoded response keeps a strong ETag, marked with
// gzipTagSuffix ("<tag>-gzip"), since its bytes differ from the identity
// form; the suffix is taken off incoming If-None-Match tags, so the handler's
// own comparison still answers 304, which then carries the marked tag.
// Standard library only, so gzip and not brotli.
func Gzip(next http.Handler) http.Handler {
	pool := newGzipPool()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept-Encoding")
		if r.Method == http.MethodHead || !acceptsGzip(r.Header.Get("Accept-Encoding")) {
			next.ServeHTTP(w, r)
			return
		}
		gw := &gzipWriter{ResponseWriter: w, pool: pool}
		if inm, ok := untagged(r.Header.Get("If-None-Match")); ok {
			r = r.Clone(r.Context())
			r.Header.Set("If-None-Match", inm)
			gw.tagged = true
		}
		defer gw.close()
		next.ServeHTTP(gw, r)
	})
}

// acceptsGzip reports whether an Accept-Encoding header allows gzip.
func acceptsGzip(header string) bool {
	star := false
	for part := range strings.SplitSeq(header, ",") {
		name, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		q := 1.0
		if v, ok := strings.CutPrefix(strings.TrimSpace(params), "q="); ok {
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				f = 0
			}
			q = f
		}
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "gzip":
			return q > 0
		case "*":
			star = q > 0
		}
	}
	return star
}

// gzipWriter decides at WriteHeader whether to encode the body.
type gzipWriter struct {
	http.ResponseWriter

	pool    *sync.Pool
	decided bool
	// tagged: the request named an encoded tag, so a 304 answers with one.
	tagged bool
	zw     *gzip.Writer
}

func (w *gzipWriter) WriteHeader(code int) {
	if !w.decided {
		w.decided = true
		h := w.Header()
		switch {
		case encodable(code, h):
			h.Del("Content-Length")
			h.Set("Content-Encoding", "gzip")
			h.Set("ETag", tagged(h.Get("ETag")))
			zw, _ := w.pool.Get().(*gzip.Writer)
			zw.Reset(w.ResponseWriter)
			w.zw = zw
		case code == http.StatusNotModified && w.tagged:
			h.Set("ETag", tagged(h.Get("ETag")))
		}
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *gzipWriter) Write(b []byte) (int, error) {
	if !w.decided {
		w.WriteHeader(http.StatusOK)
	}
	var (
		n   int
		err error
	)
	if w.zw != nil {
		n, err = w.zw.Write(b)
	} else {
		n, err = w.ResponseWriter.Write(b)
	}
	if err != nil {
		return n, fmt.Errorf("write: %w", err)
	}
	return n, nil
}

// Flush sends what is encoded so far, for handlers that stream.
func (w *gzipWriter) Flush() {
	if w.zw != nil {
		_ = w.zw.Flush()
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the real writer.
func (w *gzipWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *gzipWriter) close() {
	if w.zw == nil {
		return
	}
	_ = w.zw.Close()
	w.zw.Reset(nil)
	w.pool.Put(w.zw)
	w.zw = nil
}

// tagged marks a strong ETag as the encoded form's; weak or empty tags stay.
func tagged(etag string) string {
	if etag == "" || strings.HasPrefix(etag, "W/") || !strings.HasSuffix(etag, `"`) {
		return etag
	}
	return strings.TrimSuffix(etag, `"`) + gzipTagSuffix + `"`
}

// untagged takes the encoded-form mark off every tag of an If-None-Match
// header and reports whether any had it.
func untagged(header string) (string, bool) {
	mark := gzipTagSuffix + `"`
	if !strings.Contains(header, mark) {
		return header, false
	}
	return strings.ReplaceAll(header, mark, `"`), true
}

// encodable reports whether a response with this status and these headers is
// gzip-encoded.
func encodable(code int, h http.Header) bool {
	if code < http.StatusOK || code == http.StatusNoContent || code == http.StatusPartialContent ||
		code == http.StatusNotModified {
		return false
	}
	return headersEncodable(h)
}

// headersEncodable reports whether these response headers allow gzip: a text
// type, no encoding or range yet, not no-transform, and big enough if sized.
func headersEncodable(h http.Header) bool {
	if h.Get("Content-Encoding") != "" || h.Get("Content-Range") != "" ||
		strings.Contains(strings.ToLower(h.Get("Cache-Control")), "no-transform") {
		return false
	}
	if !compressibleType(h.Get("Content-Type")) {
		return false
	}
	if l := h.Get("Content-Length"); l != "" {
		n, err := strconv.Atoi(l)
		return err == nil && n >= MinGzipBytes
	}
	return true
}

// compressibleType reports whether a media type is text that gzip shrinks.
func compressibleType(ct string) bool {
	mt, _, _ := strings.Cut(strings.ToLower(ct), ";")
	mt = strings.TrimSpace(mt)
	return strings.HasPrefix(mt, "text/") || strings.HasSuffix(mt, "+json") || strings.HasSuffix(mt, "+xml") ||
		mt == "application/json" || mt == "application/xml" || mt == "application/javascript"
}
