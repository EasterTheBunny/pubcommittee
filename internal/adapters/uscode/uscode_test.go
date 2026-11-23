package uscode_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/adapters/uscode"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

const testUA = "pubcommittee-test/1"

const downloadPage = `<html><body>
<p>The code is current through Public Law 119-142 (November 10, 2026).</p>
<a href="releasepoints/us/pl/119/142/xml_usc01@119-142.zip">usc01</a>
<a href="releasepoints/us/pl/119/142/xml_usc03@119-142.zip">usc03</a>
<a href="releasepoints/us/pl/119/140/htm_usc02@119-140.zip">older</a>
<a href="releasepoints/us/pl/119/142/xml_usc42@119-142.zip">usc42</a>
</body></html>`

const priorPage = `<html><body>
<a href="releasepoints/us/pl/99/10/xml_usc01@99-10.zip">a</a>
<a href="releasepoints/us/pl/100/1/xml_usc01@100-1.zip">b</a>
<a href="releasepoints/us/pl/119/140/xml_usc05@119-140.zip">dup</a>
</body></html>`

// serve starts a site with the given routes; unknown paths answer 404.
func serve(t *testing.T, routes map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for p, h := range routes {
		mux.HandleFunc(p, h)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func page(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}
}

func newSource(t *testing.T, baseURL string) *uscode.Source {
	t.Helper()
	src, err := uscode.New(uscode.Config{BaseURL: baseURL, UserAgent: testUA, MinInterval: -1})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return src
}

// buildZip returns a zip holding one entry called name with content.
func buildZip(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	if _, err := w.Write(content); err != nil {
		t.Fatalf("zip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func zipRoute(body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}
}

func titleXML() []byte {
	return []byte("<?xml version=\"1.0\"?>\r\n<uscDoc>café \x00 bytes</uscDoc>\r\n")
}

func TestListReleasePoints(t *testing.T) {
	t.Parallel()
	date := time.Date(2026, time.November, 10, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		routes map[string]http.HandlerFunc
		want   []data.ReleasePoint
	}{
		{
			name: "current page with date; prior page 404 tolerated",
			routes: map[string]http.HandlerFunc{
				"/download.html": page(downloadPage),
			},
			want: []data.ReleasePoint{
				{ID: "119-142", Date: date},
				{ID: "119-140"},
			},
		},
		{
			name: "prior page adds ids, numeric order, no guessed dates",
			routes: map[string]http.HandlerFunc{
				"/download.html":                   page(downloadPage),
				"/download/priorreleasepoints.htm": page(priorPage),
			},
			want: []data.ReleasePoint{
				{ID: "119-142", Date: date},
				{ID: "119-140"},
				{ID: "100-1"},
				{ID: "99-10"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := serve(t, tt.routes)
			got, err := newSource(t, srv.URL).ListReleasePoints(context.Background())
			if err != nil {
				t.Fatalf("ListReleasePoints: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d release points, want %d: %+v", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if got[i].ID != tt.want[i].ID || !got[i].Date.Equal(tt.want[i].Date) {
					t.Errorf("point %d = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestListReleasePointsErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		page   http.HandlerFunc
		wantIs error
	}{
		{"current page 404", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }, data.ErrNotFound},
		{"current page 500", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) }, data.ErrUnavailable},
		{"layout changed: no ids", page("<html>nothing here</html>"), data.ErrUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := serve(t, map[string]http.HandlerFunc{"/download.html": tt.page})
			_, err := newSource(t, srv.URL).ListReleasePoints(context.Background())
			if !errors.Is(err, tt.wantIs) {
				t.Fatalf("err = %v, want %v", err, tt.wantIs)
			}
		})
	}
}

func TestFetchTitleExactBytes(t *testing.T) {
	t.Parallel()
	rp := data.ReleasePoint{ID: "119-142"}
	tests := []struct {
		title int
		entry string
		path  string
	}{
		{3, "usc03.xml", "/download/releasepoints/us/pl/119/142/xml_usc03@119-142.zip"},
		{42, "USC42.xml", "/download/releasepoints/us/pl/119/142/xml_usc42@119-142.zip"},
	}
	for _, tt := range tests {
		t.Run(tt.entry, func(t *testing.T) {
			t.Parallel()
			srv := serve(t, map[string]http.HandlerFunc{
				tt.path: zipRoute(buildZip(t, "xml/"+tt.entry, titleXML())),
			})
			got, err := newSource(t, srv.URL).FetchTitle(context.Background(), rp, tt.title)
			if err != nil {
				t.Fatalf("FetchTitle: %v", err)
			}
			if !bytes.Equal(got, titleXML()) {
				t.Fatalf("bytes differ: got %d bytes, want %d", len(got), len(titleXML()))
			}
		})
	}
}

func TestFetchTitleErrors(t *testing.T) {
	t.Parallel()
	const okPath = "/download/releasepoints/us/pl/119/142/xml_usc03@119-142.zip"
	tests := []struct {
		name   string
		routes map[string]http.HandlerFunc
		rp     data.ReleasePoint
		title  int
		wantIs error
	}{
		{name: "404", routes: map[string]http.HandlerFunc{}, rp: data.ReleasePoint{ID: "119-142"}, title: 3, wantIs: data.ErrNotFound},
		{name: "title 0", routes: map[string]http.HandlerFunc{}, rp: data.ReleasePoint{ID: "119-142"}, title: 0, wantIs: data.ErrInvalid},
		{name: "bad release id", routes: map[string]http.HandlerFunc{}, rp: data.ReleasePoint{ID: "119/142"}, title: 3, wantIs: data.ErrInvalid},
		{name: "bad zip", routes: map[string]http.HandlerFunc{okPath: zipRoute([]byte("not a zip"))}, rp: data.ReleasePoint{ID: "119-142"}, title: 3, wantIs: data.ErrUnavailable},
		{name: "no usc03.xml in zip", routes: map[string]http.HandlerFunc{okPath: zipRoute(buildZip(t, "usc04.xml", titleXML()))}, rp: data.ReleasePoint{ID: "119-142"}, title: 3, wantIs: data.ErrUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := serve(t, tt.routes)
			_, err := newSource(t, srv.URL).FetchTitle(context.Background(), tt.rp, tt.title)
			if !errors.Is(err, tt.wantIs) {
				t.Fatalf("err = %v, want %v", err, tt.wantIs)
			}
		})
	}
}

func TestOversizeTitle(t *testing.T) {
	t.Parallel()
	const p = "/download/releasepoints/us/pl/119/142/xml_usc03@119-142.zip"
	body := bytes.Repeat([]byte("x"), 200)
	srv := serve(t, map[string]http.HandlerFunc{p: zipRoute(buildZip(t, "usc03.xml", body))})
	src, err := uscode.New(uscode.Config{BaseURL: srv.URL, UserAgent: testUA, MinInterval: -1, MaxBytes: 100})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = src.FetchTitle(context.Background(), data.ReleasePoint{ID: "119-142"}, 3)
	if !errors.Is(err, data.ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
}

func TestRetryThenSucceed(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := serve(t, map[string]http.HandlerFunc{
		"/download.html": func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) <= 2 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			page(downloadPage)(w, r)
		},
	})
	src := newSource(t, srv.URL)
	got, err := src.ListReleasePoints(context.Background())
	if err != nil {
		t.Fatalf("ListReleasePoints: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("no release points after retry")
	}
	if n := calls.Load(); n != 3 {
		t.Fatalf("download page requested %d times, want 3", n)
	}
}

func TestPersistentRateLimited(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := serve(t, map[string]http.HandlerFunc{
		"/download.html": func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusTooManyRequests)
		},
	})
	_, err := newSource(t, srv.URL).ListReleasePoints(context.Background())
	if !errors.Is(err, data.ErrRateLimited) {
		t.Fatalf("err = %v, want ErrRateLimited", err)
	}
	if n := calls.Load(); n != 4 {
		t.Fatalf("requested %d times, want 4 (1 try + 3 retries)", n)
	}
}

func TestBadConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		cfg  uscode.Config
	}{
		{"ftp base URL", uscode.Config{BaseURL: "ftp://example.test", UserAgent: testUA}},
		{"unparseable base URL", uscode.Config{BaseURL: "://bad", UserAgent: testUA}},
		{"base URL without host", uscode.Config{BaseURL: "https://", UserAgent: testUA}},
		{"empty user agent", uscode.Config{UserAgent: "   "}},
		{"negative max bytes", uscode.Config{UserAgent: testUA, MaxBytes: -1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := uscode.New(tt.cfg)
			if !errors.Is(err, data.ErrInvalid) {
				t.Fatalf("err = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestNewMakesNoRequest(t *testing.T) {
	t.Parallel()
	var hits atomic.Int32
	srv := serve(t, map[string]http.HandlerFunc{
		"/": func(http.ResponseWriter, *http.Request) { hits.Add(1) },
	})
	if _, err := uscode.New(uscode.Config{BaseURL: srv.URL, UserAgent: testUA}); err != nil {
		t.Fatalf("New: %v", err)
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("New made %d requests, want 0", n)
	}
}

func TestUserAgentOnEveryRequest(t *testing.T) {
	t.Parallel()
	var bad atomic.Int32
	check := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("User-Agent") != testUA {
				bad.Add(1)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			h(w, r)
		}
	}
	const p = "/download/releasepoints/us/pl/119/142/xml_usc03@119-142.zip"
	srv := serve(t, map[string]http.HandlerFunc{
		"/download.html":                   check(page(downloadPage)),
		"/download/priorreleasepoints.htm": check(page(priorPage)),
		p:                                  check(zipRoute(buildZip(t, "usc03.xml", titleXML()))),
	})
	src := newSource(t, srv.URL)
	if _, err := src.ListReleasePoints(context.Background()); err != nil {
		t.Fatalf("ListReleasePoints: %v", err)
	}
	if _, err := src.FetchTitle(context.Background(), data.ReleasePoint{ID: "119-142"}, 3); err != nil {
		t.Fatalf("FetchTitle: %v", err)
	}
	if n := bad.Load(); n != 0 {
		t.Fatalf("%d requests lacked the User-Agent", n)
	}
}

func TestRequestsSerialized(t *testing.T) {
	t.Parallel()
	var inFlight, maxSeen atomic.Int32
	slow := func(w http.ResponseWriter, r *http.Request) {
		n := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			m := maxSeen.Load()
			if n <= m || maxSeen.CompareAndSwap(m, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		page(downloadPage)(w, r)
	}
	srv := serve(t, map[string]http.HandlerFunc{"/download.html": slow})
	src := newSource(t, srv.URL)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Go(func() {
			_, errs[i] = src.ListReleasePoints(context.Background())
		})
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if m := maxSeen.Load(); m != 1 {
		t.Fatalf("max concurrent requests = %d, want 1", m)
	}
}
