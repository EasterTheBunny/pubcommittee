package readapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/contract"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/readapi"
	"github.com/easterthebunny/pubcommittee/internal/readapi/seed"
)

// serve starts the handler over a seeded memory store and returns its URL
// and the log it writes.
func serve(t *testing.T, opts readapi.Options) (string, *bytes.Buffer) {
	t.Helper()
	store := memory.New()
	if err := seed.Apply(context.Background(), store.PlatformPorts()); err != nil {
		t.Fatal(err)
	}
	logs := new(bytes.Buffer)
	opts.Logger = slog.New(slog.NewJSONHandler(logs, nil))
	h, err := readapi.Handler(readapi.New(store.PublicReadPorts()), opts)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv.URL, logs
}

// reply is a fully read response.
type reply struct {
	status int
	header http.Header
	body   []byte
}

func do(t *testing.T, method, url string, hdr map[string]string) reply {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return reply{status: resp.StatusCode, header: resp.Header, body: body}
}

func TestContractEveryRoute(t *testing.T) {
	t.Parallel()
	url, _ := serve(t, readapi.Options{RateLimits: readapi.RateLimits{}})
	h, err := contract.New(url)
	if err != nil {
		t.Fatal(err)
	}
	contract.CheckSeededRoutes(t, h)
}

func TestConditionalGet(t *testing.T) {
	t.Parallel()
	url, _ := serve(t, readapi.Options{RateLimits: readapi.RateLimits{}})
	for _, path := range []string{"/v1/bills/" + seed.TrackedBill + "/v/2/s/s4/l/1-2", "/v1/bills", "/v1/selection-policy/1"} {
		first := do(t, http.MethodGet, url+path, nil)
		etag := first.header.Get("ETag")
		if etag == "" || strings.HasPrefix(etag, "W/") {
			t.Fatalf("%s: ETag %q", path, etag)
		}
		second := do(t, http.MethodGet, url+path, map[string]string{"If-None-Match": etag})
		if second.status != http.StatusNotModified {
			t.Fatalf("%s: status %d, want 304", path, second.status)
		}
		if len(second.body) != 0 {
			t.Fatalf("%s: 304 has a body", path)
		}
		if second.header.Get("ETag") != etag {
			t.Fatalf("%s: 304 ETag changed", path)
		}
		other := do(t, http.MethodGet, url+path, map[string]string{"If-None-Match": `"other"`})
		if other.status != http.StatusOK {
			t.Fatalf("%s: stale tag status %d, want 200", path, other.status)
		}
	}
}

func TestCacheClasses(t *testing.T) {
	t.Parallel()
	url, _ := serve(t, readapi.Options{RateLimits: readapi.RateLimits{}})
	tests := map[string]string{
		"/v1/bills/" + seed.TrackedBill + "/v/1":              "immutable",
		"/v1/selection-policy/2":                              "immutable",
		"/v1/bills/" + seed.TrackedBill + "/v/latest":         "stale-while-revalidate",
		"/v1/bills/" + seed.TrackedBill + "/v/2/explanations": "stale-while-revalidate",
		"/v1/bills": "stale-while-revalidate",
		"/v1/feed":  "max-age=900",
	}
	for path, want := range tests {
		if cc := do(t, http.MethodGet, url+path, nil).header.Get("Cache-Control"); !strings.Contains(cc, want) {
			t.Errorf("%s: Cache-Control %q lacks %q", path, cc, want)
		}
	}
}

func TestCORS(t *testing.T) {
	t.Parallel()
	url, _ := serve(t, readapi.Options{RateLimits: readapi.RateLimits{}})
	resp := do(t, http.MethodGet, url+"/v1", map[string]string{"Origin": "https://example.test"})
	if resp.header.Get("Access-Control-Allow-Origin") != "*" || resp.header.Get("Access-Control-Allow-Credentials") != "" {
		t.Fatalf("CORS headers = %v", resp.header)
	}
	pre := do(t, http.MethodOptions, url+"/v1/bills", map[string]string{
		"Origin": "https://example.test", "Access-Control-Request-Method": "GET",
	})
	if pre.status != http.StatusNoContent || !strings.Contains(pre.header.Get("Access-Control-Allow-Methods"), "GET") {
		t.Fatalf("preflight = %d %v", pre.status, pre.header)
	}
}

func TestWritesAndUnknownRoutes(t *testing.T) {
	t.Parallel()
	url, _ := serve(t, readapi.Options{RateLimits: readapi.RateLimits{}})
	if resp := do(t, http.MethodPost, url+"/v1/bills", nil); resp.status != http.StatusMethodNotAllowed {
		t.Fatalf("POST status %d, want 405", resp.status)
	}
	resp := do(t, http.MethodGet, url+"/v1/nothing", nil)
	if resp.status != http.StatusNotFound || !strings.Contains(string(resp.body), "/problems/not-found") {
		t.Fatalf("unknown route = %d %s", resp.status, resp.body)
	}
}

func TestRateLimitSlowsClient(t *testing.T) {
	t.Parallel()
	url, _ := serve(t, readapi.Options{RateLimits: readapi.RateLimits{"discovery": 6}})
	h, err := contract.New(url)
	if err != nil {
		t.Fatal(err)
	}
	limited := false
	for range 30 {
		res, err := h.Get(t.Context(), "/v1", nil, nil)
		if err != nil {
			t.Fatalf("contract: %v", err)
		}
		if res.Status == http.StatusTooManyRequests {
			limited = true
			if res.Header.Get("Retry-After") == "" {
				t.Fatal("429 without Retry-After")
			}
			break
		}
	}
	if !limited {
		t.Fatal("no 429 after 30 requests at 6 per minute")
	}
}

func TestLogsHoldNoClientData(t *testing.T) {
	t.Parallel()
	url, logs := serve(t, readapi.Options{RateLimits: readapi.RateLimits{}})
	do(t, http.MethodGet, url+"/v1/bills?q=secretquery", map[string]string{
		"Cookie": "session=secretcookie", "X-Forwarded-For": "203.0.113.77", "Authorization": "Bearer secrettoken",
	})
	out := logs.String()
	if !strings.Contains(out, `"route":"GET /v1/bills"`) {
		t.Fatalf("log lacks the route: %s", out)
	}
	for _, banned := range []string{"127.0.0.1", "[::1]", "203.0.113.77", "secretcookie", "secretquery", "secrettoken", "remote"} {
		if strings.Contains(out, banned) {
			t.Fatalf("log contains %q: %s", banned, out)
		}
	}
}

// TestModerationLogPagesNewestWrittenFirst: entries of different kinds on one
// day come newest first across pages, whatever time of day they carried.
func TestModerationLogPagesNewestWrittenFirst(t *testing.T) {
	t.Parallel()
	store := memory.New()
	day := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	for i, e := range []data.LogEntry{
		{ID: "g1", At: day.Add(20 * time.Hour), Kind: "generation"},
		{ID: "a1", At: day, Kind: "policy_published"},
		{ID: "g2", At: day.Add(2 * time.Hour), Kind: "generation"},
		{ID: "a2", At: day, Kind: "operator_added"},
	} {
		e.Meta = data.Meta{Version: 1, AsOf: day, Provenance: data.ProvenanceSource, ContentClass: data.ContentOfficial}
		if err := store.Record(t.Context(), e); err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
	}
	h, err := readapi.Handler(readapi.New(store.PublicReadPorts()), readapi.Options{Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	var got []string
	url := srv.URL + "/v1/moderation-log?limit=3"
	for url != "" {
		var page struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
			Next string `json:"next"`
		}
		if err := json.Unmarshal(do(t, http.MethodGet, url, nil).body, &page); err != nil {
			t.Fatal(err)
		}
		for _, it := range page.Items {
			got = append(got, it.ID)
		}
		url = ""
		if page.Next != "" {
			url = srv.URL + "/v1/moderation-log?limit=3&cursor=" + neturl.QueryEscape(page.Next)
		}
	}
	if want := []string{"a2", "g2", "a1", "g1"}; !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}
