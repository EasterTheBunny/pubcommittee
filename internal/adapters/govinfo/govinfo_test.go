package govinfo_test

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/adapters/govinfo"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/data/conformance"
)

// fixturesDir is the repository fixtures directory, relative to this package.
const fixturesDir = "../../../fixtures"

// etag is the entity tag the fake server gives every file it serves.
const etag = `"fixture-1"`

// overlapWindow keeps each fake request in flight long enough for a second
// concurrent request to show up in the peak count.
const overlapWindow = 5 * time.Millisecond

// defaultBackoff is the adapter's first retry delay.
const defaultBackoff = 2 * time.Second

// fixedNow is the clock every test source uses.
func fixedNow() time.Time { return time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC) }

// hr4313Ref is the exit bill the suites check against.
func hr4313Ref() data.BillRef {
	return data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: 4313}
}

// hr4313Title is the title the status of hr4313 carries.
const hr4313Title = "Hospital Inpatient Services Modernization Act"

// hr4313Codes are the text versions the status lists, oldest first.
func hr4313Codes() []string { return []string{"ih", "rh", "eh", "rfs"} }

// statusURLPath is the govinfo BILLSTATUS path of hr4313.
const statusURLPath = "/bulkdata/BILLSTATUS/119/hr/BILLSTATUS-119hr4313.xml"

// textURLPath returns the govinfo path of one text version of hr4313.
func textURLPath(code string) string {
	id := "BILLS-119hr4313" + code
	return "/content/pkg/" + id + "/xml/" + id + ".xml"
}

// textURL returns the official URL the status lists for one text version.
func textURL(code string) string {
	return "https://www.govinfo.gov" + textURLPath(code)
}

// ihVersion is the listed "ih" version of hr4313.
func ihVersion() data.SourceVersion {
	return data.SourceVersion{Ref: data.VersionRef{Bill: hr4313Ref(), Version: "ih"}, URL: textURL("ih")}
}

// readFixture returns one file under fixtures/.
func readFixture(t *testing.T, rel string) []byte {
	t.Helper()
	b, err := fs.ReadFile(os.DirFS(fixturesDir), rel)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return b
}

// versionByCode returns the listed version with code, or fails the test.
func versionByCode(t *testing.T, vs []data.SourceVersion, code string) data.SourceVersion {
	t.Helper()
	for _, v := range vs {
		if v.Ref.Version == code {
			return v
		}
	}
	t.Fatalf("no listed version with code %q", code)
	return data.SourceVersion{}
}

// hit is one request the fake server received.
type hit struct {
	path        string
	userAgent   string
	ifNoneMatch string
}

// reply is one scripted answer that replaces the file for one request.
type reply struct {
	status     int
	retryAfter string
}

// fakeGovinfo serves the hr4313 fixture files under the real govinfo paths.
// Scripted replies for a path are served, in order, before the file.
type fakeGovinfo struct {
	srv   *httptest.Server
	files map[string][]byte

	mu     sync.Mutex
	hits   []hit
	script map[string][]reply

	inflight    atomic.Int32
	peak        atomic.Int32
	notModified atomic.Int32
}

func newFakeGovinfo(t *testing.T, script map[string][]reply) *fakeGovinfo {
	t.Helper()
	if script == nil {
		script = map[string][]reply{}
	}
	g := &fakeGovinfo{files: map[string][]byte{}, script: script}
	g.files[statusURLPath] = readFixture(t, path.Join("bills", hr4313Ref().ID(), "status.xml"))
	for _, code := range hr4313Codes() {
		g.files[textURLPath(code)] = readFixture(t, path.Join("bills", hr4313Ref().ID(), "v", code, "source.xml"))
	}
	g.srv = httptest.NewServer(http.HandlerFunc(g.serve))
	t.Cleanup(g.srv.Close)
	return g
}

func (g *fakeGovinfo) serve(w http.ResponseWriter, r *http.Request) {
	n := g.inflight.Add(1)
	defer g.inflight.Add(-1)
	for {
		p := g.peak.Load()
		if n <= p || g.peak.CompareAndSwap(p, n) {
			break
		}
	}
	time.Sleep(overlapWindow)

	g.mu.Lock()
	g.hits = append(g.hits, hit{path: r.URL.Path, userAgent: r.UserAgent(), ifNoneMatch: r.Header.Get("If-None-Match")})
	var scripted *reply
	if q := g.script[r.URL.Path]; len(q) > 0 {
		rep := q[0]
		scripted = &rep
		g.script[r.URL.Path] = q[1:]
	}
	g.mu.Unlock()

	if scripted != nil {
		if scripted.retryAfter != "" {
			w.Header().Set("Retry-After", scripted.retryAfter)
		}
		w.WriteHeader(scripted.status)
		return
	}
	body, ok := g.files[r.URL.Path]
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		g.notModified.Add(1)
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	_, _ = w.Write(body)
}

// requests returns the requests received so far.
func (g *fakeGovinfo) requests() []hit {
	g.mu.Lock()
	defer g.mu.Unlock()
	return slices.Clone(g.hits)
}

// sleepLog is a Sleep that records each duration and returns at once.
type sleepLog struct {
	mu        sync.Mutex
	durations []time.Duration
}

func (l *sleepLog) sleep(_ context.Context, d time.Duration) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.durations = append(l.durations, d)
	return nil
}

func (l *sleepLog) got() []time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.durations)
}

// newSource returns a govinfo Source that talks to g with no pacing delay and
// the recording sleep.
func newSource(t *testing.T, g *fakeGovinfo, sleeps *sleepLog) *govinfo.Source {
	t.Helper()
	src, err := govinfo.New(govinfo.Config{
		BaseURL:     g.srv.URL,
		MinInterval: -1,
		Sleep:       sleeps.sleep,
		Now:         fixedNow,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return src
}

func TestBillFetcherConformance(t *testing.T) {
	t.Parallel()
	conformance.BillFetcher(t, func(t *testing.T) conformance.FetcherFixture {
		t.Helper()
		g := newFakeGovinfo(t, nil)
		src := newSource(t, g, &sleepLog{})
		rec, err := src.FetchRecord(context.Background(), hr4313Ref())
		if err != nil {
			t.Fatalf("FetchRecord: %v", err)
		}
		return conformance.FetcherFixture{
			Fetcher:      src,
			Bill:         hr4313Ref(),
			Title:        hr4313Title,
			VersionCodes: hr4313Codes(),
			Original:     versionByCode(t, rec.Versions, "ih"),
			OriginalXML:  readFixture(t, path.Join("bills", hr4313Ref().ID(), "v", "ih", "source.xml")),
			Missing:      data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: 999999},
		}
	})
}

func TestUserAgentNamesProjectWithoutAt(t *testing.T) {
	t.Parallel()
	g := newFakeGovinfo(t, nil)
	src := newSource(t, g, &sleepLog{})
	ctx := context.Background()
	if _, err := src.FetchRecord(ctx, hr4313Ref()); err != nil {
		t.Fatalf("FetchRecord: %v", err)
	}
	if _, err := src.FetchOriginal(ctx, ihVersion()); err != nil {
		t.Fatalf("FetchOriginal: %v", err)
	}
	reqs := g.requests()
	if len(reqs) == 0 {
		t.Fatal("no requests reached the server")
	}
	for _, r := range reqs {
		if !strings.Contains(r.userAgent, "pubcommittee") {
			t.Errorf("request %s User-Agent %q does not name the project", r.path, r.userAgent)
		}
		if strings.Contains(r.userAgent, "@") {
			t.Errorf("request %s User-Agent %q contains @", r.path, r.userAgent)
		}
	}
}

func TestOneRequestInFlight(t *testing.T) {
	t.Parallel()
	g := newFakeGovinfo(t, nil)
	src := newSource(t, g, &sleepLog{})
	ctx := context.Background()

	const callers = 5
	var wg sync.WaitGroup
	errs := make(chan error, callers)
	for range callers {
		wg.Go(func() {
			_, err := src.FetchOriginal(ctx, ihVersion())
			errs <- err
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("FetchOriginal: %v", err)
		}
	}
	if peak := g.peak.Load(); peak != 1 {
		t.Fatalf("peak requests in flight = %d, want 1", peak)
	}
}

func TestBacksOffOnServerErrors(t *testing.T) {
	t.Parallel()
	script := map[string][]reply{textURLPath("ih"): {{status: http.StatusServiceUnavailable}, {status: http.StatusServiceUnavailable}}}
	g := newFakeGovinfo(t, script)
	sleeps := &sleepLog{}
	src := newSource(t, g, sleeps)
	if _, err := src.FetchOriginal(context.Background(), ihVersion()); err != nil {
		t.Fatalf("FetchOriginal after two 503: %v", err)
	}
	if n := len(g.requests()); n != 3 {
		t.Fatalf("requests = %d, want 3", n)
	}
	want := []time.Duration{defaultBackoff, 2 * defaultBackoff}
	if got := sleeps.got(); !slices.Equal(got, want) {
		t.Fatalf("backoff sleeps = %v, want %v", got, want)
	}
}

func TestHonorsRetryAfter(t *testing.T) {
	t.Parallel()
	script := map[string][]reply{textURLPath("ih"): {{status: http.StatusTooManyRequests, retryAfter: "7"}}}
	g := newFakeGovinfo(t, script)
	sleeps := &sleepLog{}
	src := newSource(t, g, sleeps)
	if _, err := src.FetchOriginal(context.Background(), ihVersion()); err != nil {
		t.Fatalf("FetchOriginal after 429 with Retry-After: %v", err)
	}
	got := sleeps.got()
	if len(got) != 1 {
		t.Fatalf("sleeps = %v, want one sleep", got)
	}
	if got[0] < 7*time.Second {
		t.Fatalf("sleep after Retry-After: 7 = %v, want at least 7s", got[0])
	}
}

func TestRateLimitedGivesUpAfterRetries(t *testing.T) {
	t.Parallel()
	const attempts = 4 // the first attempt and three retries
	script := map[string][]reply{textURLPath("ih"): {}}
	for range attempts {
		script[textURLPath("ih")] = append(script[textURLPath("ih")], reply{status: http.StatusTooManyRequests})
	}
	g := newFakeGovinfo(t, script)
	src := newSource(t, g, &sleepLog{})
	_, err := src.FetchOriginal(context.Background(), ihVersion())
	if !errors.Is(err, data.ErrRateLimited) {
		t.Fatalf("FetchOriginal on constant 429: err = %v, want ErrRateLimited", err)
	}
	if n := len(g.requests()); n != attempts {
		t.Fatalf("requests = %d, want %d", n, attempts)
	}
}

func TestNotFoundIsNotRetried(t *testing.T) {
	t.Parallel()
	g := newFakeGovinfo(t, nil)
	src := newSource(t, g, &sleepLog{})
	missing := data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: 999999}
	_, err := src.FetchRecord(context.Background(), missing)
	if !errors.Is(err, data.ErrNotFound) {
		t.Fatalf("FetchRecord of a missing bill: err = %v, want ErrNotFound", err)
	}
	if n := len(g.requests()); n != 1 {
		t.Fatalf("requests = %d, want 1", n)
	}
}

func TestConditionalRequestReturnsSameRecord(t *testing.T) {
	t.Parallel()
	g := newFakeGovinfo(t, nil)
	src := newSource(t, g, &sleepLog{})
	ctx := context.Background()
	first, err := src.FetchRecord(ctx, hr4313Ref())
	if err != nil {
		t.Fatalf("first FetchRecord: %v", err)
	}
	second, err := src.FetchRecord(ctx, hr4313Ref())
	if err != nil {
		t.Fatalf("second FetchRecord: %v", err)
	}
	reqs := g.requests()
	if len(reqs) != 2 {
		t.Fatalf("requests = %d, want 2", len(reqs))
	}
	if reqs[1].ifNoneMatch != etag {
		t.Fatalf("second request If-None-Match = %q, want %q", reqs[1].ifNoneMatch, etag)
	}
	if n := g.notModified.Load(); n != 1 {
		t.Fatalf("304 responses = %d, want 1", n)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("record after 304 differs from the first record")
	}
}

func TestRefusesOtherHosts(t *testing.T) {
	t.Parallel()
	g := newFakeGovinfo(t, nil)
	src := newSource(t, g, &sleepLog{})
	v := data.SourceVersion{Ref: data.VersionRef{Bill: hr4313Ref(), Version: "ih"}, URL: "https://evil.example/x.xml"}
	_, err := src.FetchOriginal(context.Background(), v)
	if !errors.Is(err, data.ErrInvalid) {
		t.Fatalf("FetchOriginal of a foreign host: err = %v, want ErrInvalid", err)
	}
	if n := len(g.requests()); n != 0 {
		t.Fatalf("requests = %d, want 0", n)
	}
}

func TestRefusesNonXMLURL(t *testing.T) {
	t.Parallel()
	g := newFakeGovinfo(t, nil)
	src := newSource(t, g, &sleepLog{})
	v := data.SourceVersion{Ref: data.VersionRef{Bill: hr4313Ref(), Version: "ih"}, URL: "https://www.govinfo.gov/content/pkg/BILLS-119hr4313ih/xml/BILLS-119hr4313ih.htm"}
	_, err := src.FetchOriginal(context.Background(), v)
	if !errors.Is(err, data.ErrInvalid) {
		t.Fatalf("FetchOriginal of a non-.xml URL: err = %v, want ErrInvalid", err)
	}
	if n := len(g.requests()); n != 0 {
		t.Fatalf("requests = %d, want 0", n)
	}
}
