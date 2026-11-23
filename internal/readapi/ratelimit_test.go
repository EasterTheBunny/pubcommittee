package readapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

const (
	testAddrA      = "192.0.2.1:1234"
	testAddrB      = "192.0.2.2:1"
	testAddrOwn    = "192.0.2.1:1"
	testLivePath   = "/v1/bills"
	testBulkPath   = "/v1/feed"
	testLimit      = 60
	testBurst      = 10
	testRetryAfter = 1
	testHeader     = "X-Forwarded-For"
	testProxyChain = "203.0.113.9, 198.51.100.4"
	testProxyAddr  = "198.51.100.4"
	testRemoteHost = "192.0.2.1"
	testRateLimit  = "/problems/rate-limited"
)

// fakeClock is a clock the test advances by hand.
type fakeClock struct {
	now time.Time
}

func (c *fakeClock) Now() time.Time { return c.now }

func (c *fakeClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, time.November, 22, 12, 0, 0, 0, time.UTC)}
}

func newTestRequest(path, remoteAddr string) *http.Request {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
	r.RemoteAddr = remoteAddr
	return r
}

func newTestLimiter(t *testing.T, limits RateLimits, header string, clock *fakeClock) *limiter {
	t.Helper()
	l, err := newLimiter(limits, header, clock.Now)
	if err != nil {
		t.Fatalf("newLimiter: %v", err)
	}
	return l
}

func TestParseRateLimits(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		spec    string
		want    RateLimits
		wantErr bool
	}{
		{name: "empty gives defaults", spec: "", want: DefaultRateLimits()},
		{
			name: "overrides text and live, keeps the rest",
			spec: "text=10,live=0",
			want: RateLimits{
				classText: 10, classLive: 0,
				classBulk: defaultBulk, classDiscovery: defaultDiscovery,
			},
		},
		{name: "unknown class", spec: "bogus=1", wantErr: true},
		{name: "bad number", spec: "text=x", wantErr: true},
		{name: "missing equals", spec: "text", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseRateLimits(tc.spec)
			if tc.wantErr {
				if !errors.Is(err, data.ErrInvalid) {
					t.Fatalf("ParseRateLimits(%q) error = %v, want ErrInvalid", tc.spec, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRateLimits(%q) error = %v", tc.spec, err)
			}
			if !maps.Equal(got, tc.want) {
				t.Errorf("ParseRateLimits(%q) = %v, want %v", tc.spec, got, tc.want)
			}
		})
	}
}

func TestClassify(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		want string
	}{
		{path: "/v1", want: classDiscovery},
		{path: "/v1/openapi.yaml", want: classDiscovery},
		{path: "/v1/feed", want: classBulk},
		{path: "/v1/exports", want: classBulk},
		{path: "/v1/exports/x", want: classBulk},
		{path: "/v1/selection-policy/1", want: classText},
		{path: "/v1/selection-policy", want: classLive},
		{path: testLivePath, want: classLive},
		{path: "/v1/bills/us-119-hr-1", want: classLive},
		{path: "/v1/bills/us-119-hr-1/v/2", want: classText},
		{path: "/v1/bills/us-119-hr-1/v/2/s/s4", want: classText},
		{path: "/v1/bills/us-119-hr-1/s/s4/l/3", want: classText},
		{path: "/v1/bills/us-119-hr-1/v/2/explanations", want: classLive},
		{path: "/v1/moderation-log", want: classLive},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			if got := classify(tc.path); got != tc.want {
				t.Errorf("classify(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestLimiterBlocksThenRefills(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	l := newTestLimiter(t, RateLimits{classLive: testLimit}, "", clock)

	for i := range testBurst {
		req := newTestRequest(testLivePath, testAddrA)
		if ok, _ := l.allow(req); !ok {
			t.Fatalf("call %d: allow = false, want true within burst", i+1)
		}
	}
	ok, retryAfter := l.allow(newTestRequest(testLivePath, testAddrA))
	if ok {
		t.Fatal("call 11: allow = true, want false after burst")
	}
	if retryAfter != testRetryAfter {
		t.Errorf("call 11: retryAfter = %d, want %d", retryAfter, testRetryAfter)
	}

	clock.Advance(time.Second)
	if ok, _ := l.allow(newTestRequest(testLivePath, testAddrA)); !ok {
		t.Error("after 1s: allow = false, want true after refill")
	}
}

func TestLimiterSeparatesClients(t *testing.T) {
	t.Parallel()
	t.Run("clients have own buckets", func(t *testing.T) {
		t.Parallel()
		clock := newFakeClock()
		l := newTestLimiter(t, RateLimits{classLive: testLimit}, "", clock)
		for range testBurst {
			l.allow(newTestRequest(testLivePath, testAddrOwn))
		}
		if ok, _ := l.allow(newTestRequest(testLivePath, testAddrOwn)); ok {
			t.Fatal("exhausted client: allow = true, want false")
		}
		if ok, _ := l.allow(newTestRequest(testLivePath, testAddrB)); !ok {
			t.Error("other client: allow = false, want true")
		}
	})
	t.Run("classes have own buckets for one client", func(t *testing.T) {
		t.Parallel()
		clock := newFakeClock()
		l := newBulkLimiter(t, clock)
		for range testBurst {
			l.allow(newTestRequest(testLivePath, testAddrOwn))
		}
		if ok, _ := l.allow(newTestRequest(testLivePath, testAddrOwn)); ok {
			t.Fatal("live class exhausted: allow = true, want false")
		}
		if ok, _ := l.allow(newTestRequest(testBulkPath, testAddrOwn)); !ok {
			t.Error("bulk class for same client: allow = false, want true")
		}
	})
}

func newBulkLimiter(t *testing.T, clock *fakeClock) *limiter {
	t.Helper()
	return newTestLimiter(t, RateLimits{classLive: testLimit, classBulk: testLimit}, "", clock)
}

func TestLimiterUnlimited(t *testing.T) {
	t.Parallel()
	const calls = 100
	clock := newFakeClock()
	l := newTestLimiter(t, RateLimits{classLive: 0}, "", clock)
	for i := range calls {
		if ok, _ := l.allow(newTestRequest(testLivePath, testAddrA)); !ok {
			t.Fatalf("call %d: allow = false, want true for unlimited class", i+1)
		}
	}
}

func TestLimiterClientHeader(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	l := newTestLimiter(t, RateLimits{classLive: testLimit}, testHeader, clock)

	withHeader := newTestRequest(testLivePath, testAddrA)
	withHeader.Header.Set(testHeader, testProxyChain)
	if got := l.clientAddr(withHeader); got != testProxyAddr {
		t.Errorf("clientAddr with header = %q, want %q", got, testProxyAddr)
	}

	withoutHeader := newTestRequest(testLivePath, testAddrA)
	if got := l.clientAddr(withoutHeader); got != testRemoteHost {
		t.Errorf("clientAddr without header = %q, want %q", got, testRemoteHost)
	}
}

func TestLimiterKeepsNoRawAddress(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	l := newTestLimiter(t, RateLimits{classLive: testLimit}, "", clock)
	if ok, _ := l.allow(newTestRequest(testLivePath, testAddrA)); !ok {
		t.Fatal("allow = false, want true")
	}
	if len(l.buckets) == 0 {
		t.Fatal("no bucket stored after allow")
	}
	for k := range l.buckets {
		if bytes.Contains(k.client[:], []byte(testRemoteHost)) {
			t.Errorf("bucket key holds raw address bytes %q", testRemoteHost)
		}
	}

	day1 := clock.Now()
	day2 := day1.Add(24 * time.Hour)
	if l.clientKey(testRemoteHost, day1) == l.clientKey(testRemoteHost, day2) {
		t.Error("clientKey did not change across days")
	}
}

func TestMiddleware429(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	l := newTestLimiter(t, RateLimits{classLive: testLimit}, "", clock)
	handler := l.middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := range testBurst {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, newTestRequest(testLivePath, testAddrA))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want %d", i+1, rec.Code, http.StatusOK)
		}
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newTestRequest(testLivePath, testAddrA))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("request 11: status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Errorf("Retry-After = %q, want %q", got, "1")
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", got)
	}
	var body struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode problem body: %v", err)
	}
	if body.Type != testRateLimit {
		t.Errorf("problem type = %q, want %q", body.Type, testRateLimit)
	}
}
