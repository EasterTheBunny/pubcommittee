package httpx_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/httpx"
)

// bigJSON is a JSON body well above httpx.MinGzipBytes.
func bigJSON() string {
	return `{"lines":["` + strings.Repeat("Line of bill text. ", 200) + `"]}`
}

// serve sends one GET through httpx.Gzip to handler and returns the
// response headers, status, and raw body.
func serve(t *testing.T, reqHeaders map[string]string, handler http.HandlerFunc) (http.Header, int, []byte) {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/bills/x", nil)
	for k, v := range reqHeaders {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	httpx.Gzip(handler).ServeHTTP(rec, req)
	return rec.Header(), rec.Code, rec.Body.Bytes()
}

// answer returns a handler that sets headers, then writes status and body.
func answer(status int, headers map[string]string, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

// conditional answers 304 when If-None-Match names "abc", as read-api does.
func conditional(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("ETag", `"abc"`)
	if r.Header.Get("If-None-Match") == `"abc"` {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, bigJSON())
}

func TestGzipEncodesJSON(t *testing.T) {
	t.Parallel()
	h, _, body := serve(t, map[string]string{"Accept-Encoding": "gzip, br"}, answer(http.StatusOK, map[string]string{
		"Content-Type": "application/json", "Content-Length": strconv.Itoa(len(bigJSON())), "ETag": `"abc"`,
	}, bigJSON()))
	if h.Get("Content-Encoding") != "gzip" || h.Get("Content-Length") != "" {
		t.Fatalf("headers = %v", h)
	}
	if h.Get("ETag") != `"abc-gzip"` || !strings.Contains(h.Get("Vary"), "Accept-Encoding") {
		t.Errorf("etag %q vary %q", h.Get("ETag"), h.Get("Vary"))
	}
	zr, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(zr)
	if err != nil || string(got) != bigJSON() {
		t.Fatalf("decoded %d bytes, %v; want the body", len(got), err)
	}
}

func TestGzipConditionalRequest(t *testing.T) {
	t.Parallel()
	h, code, _ := serve(t, map[string]string{"Accept-Encoding": "gzip", "If-None-Match": `"abc-gzip"`}, conditional)
	if code != http.StatusNotModified || h.Get("ETag") != `"abc-gzip"` {
		t.Errorf("encoded tag: status %d etag %q, want 304 \"abc-gzip\"", code, h.Get("ETag"))
	}
	h, code, _ = serve(t, map[string]string{"Accept-Encoding": "gzip", "If-None-Match": `"abc"`}, conditional)
	if code != http.StatusNotModified || h.Get("ETag") != `"abc"` {
		t.Errorf("identity tag: status %d etag %q, want 304 \"abc\"", code, h.Get("ETag"))
	}
	_, code, _ = serve(t, map[string]string{"Accept-Encoding": "gzip", "If-None-Match": `"old-gzip"`}, conditional)
	if code != http.StatusOK {
		t.Errorf("stale tag: status %d, want 200", code)
	}
}

func TestGzipLeavesOthersAlone(t *testing.T) {
	t.Parallel()
	small := `{"a":1}`
	cases := []struct {
		name    string
		accept  string
		status  int
		headers map[string]string
		body    string
	}{
		{"no gzip accepted", "br", http.StatusOK, map[string]string{"Content-Type": "application/json"}, bigJSON()},
		{"gzip refused", "gzip;q=0", http.StatusOK, map[string]string{"Content-Type": "application/json"}, bigJSON()},
		{"small", "gzip", http.StatusOK, map[string]string{
			"Content-Type": "application/json", "Content-Length": strconv.Itoa(len(small)),
		}, small},
		{"binary", "gzip", http.StatusOK, map[string]string{"Content-Type": "image/png"}, bigJSON()},
		{"encoded", "gzip", http.StatusOK, map[string]string{
			"Content-Type": "application/json", "Content-Encoding": "br",
		}, bigJSON()},
		{"no-transform", "gzip", http.StatusOK, map[string]string{
			"Content-Type": "application/json", "Cache-Control": "public, no-transform",
		}, bigJSON()},
		{"not modified", "gzip", http.StatusNotModified, map[string]string{"ETag": `"abc"`}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			h, _, body := serve(t, map[string]string{"Accept-Encoding": c.accept}, answer(c.status, c.headers, c.body))
			if h.Get("Content-Encoding") == "gzip" {
				t.Fatalf("Content-Encoding gzip, want unchanged")
			}
			if string(body) != c.body {
				t.Errorf("body changed: %d bytes, want %d", len(body), len(c.body))
			}
			if etag := c.headers["ETag"]; etag != "" && h.Get("ETag") != etag {
				t.Errorf("etag %q, want %q", h.Get("ETag"), etag)
			}
		})
	}
}

func TestGzipAcceptsStar(t *testing.T) {
	t.Parallel()
	h, _, _ := serve(t, map[string]string{"Accept-Encoding": "*"},
		answer(http.StatusOK, map[string]string{"Content-Type": "application/problem+json"}, bigJSON()))
	if h.Get("Content-Encoding") != "gzip" {
		t.Errorf("Content-Encoding %q, want gzip for *", h.Get("Content-Encoding"))
	}
}
