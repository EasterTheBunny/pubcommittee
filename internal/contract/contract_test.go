package contract_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/contract"
)

const lineRangeOK = `{
 "version": 1, "as_of": "2026-01-02T03:04:05Z", "provenance": "source", "content_class": "official",
 "bill": "us-119-hr-4127", "code": "ih", "section_id": "s4", "section_path": ["s4"],
 "from": 5, "to": 6, "licence": "public-domain",
 "address": {"canonical": "/v1/bills/us-119-hr-4127/v/ih/s/s4/l/5-6", "pinned_version": "ih",
   "latest_version": "rh", "is_latest": false, "resolved_from_latest": false,
   "compare": "/v1/bills/us-119-hr-4127/v/ih/compare/rh"},
 "lines": [
  {"n": 5, "depth": 0, "text": "Congress finds the following:", "anchor": "s4-l5", "block_to": 6},
  {"n": 6, "depth": 1, "text": "(1) one", "anchor": "s4-l6", "block_to": 6}
 ]}`

// stub answers each path with a canned status and body.
func stub(t *testing.T, routes map[string]struct {
	status int
	ctype  string
	body   string
},
) *contract.Harness {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rt, ok := routes[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", rt.ctype)
		if rt.status == http.StatusOK {
			w.Header().Set("ETag", `"abc"`)
		}
		if rt.status == http.StatusTooManyRequests {
			w.Header().Set("Retry-After", "3")
		}
		w.WriteHeader(rt.status)
		_, _ = w.Write([]byte(rt.body))
	}))
	t.Cleanup(srv.Close)
	h, err := contract.New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

type route = struct {
	status int
	ctype  string
	body   string
}

func TestHarnessAcceptsValidResponses(t *testing.T) {
	t.Parallel()
	h := stub(t, map[string]route{
		"/v1/bills/us-119-hr-4127/v/ih/s/s4/l/5-6": {200, "application/json", lineRangeOK},
		"/v1/bills/us-119-hr-4127/v/zz": {
			404, "application/problem+json",
			`{"type":"/problems/not-found","title":"Not Found","status":404}`,
		},
	})
	res := h.MustGet(t, "/v1/bills/us-119-hr-4127/v/ih/s/s4/l/5-6", nil)
	if res.Status != http.StatusOK {
		t.Fatalf("status %d", res.Status)
	}
	if res := h.MustGet(t, "/v1/bills/us-119-hr-4127/v/zz", nil); res.Status != http.StatusNotFound {
		t.Fatalf("status %d", res.Status)
	}
}

func TestHarnessRejectsDepartures(t *testing.T) {
	t.Parallel()
	noClass := strings.Replace(lineRangeOK, `"content_class": "official",`, "", 1)
	userClass := strings.Replace(lineRangeOK, `"content_class": "official"`, `"content_class": "user"`, 1)
	badRange := strings.Replace(lineRangeOK, `"block_to": 6},
  {"n": 6`, `"block_to": 6},
  {"n": "six"`, 1)
	cases := map[string]route{
		"/missing-class":  {200, "application/json", noClass},
		"/user-class":     {200, "application/json", userClass},
		"/bad-line":       {200, "application/json", badRange},
		"/wrong-problem":  {404, "application/json", `{"type":"/problems/not-found","title":"x","status":404}`},
		"/unknown-status": {418, "application/json", `{}`},
	}
	for name, rt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := stub(t, map[string]route{"/v1/bills/us-119-hr-4127/v/ih/s/s4/l/5-6": rt})
			_, err := h.Get(t.Context(), "/v1/bills/us-119-hr-4127/v/ih/s/s4/l/5-6", nil, nil)
			t.Logf("rejected: %v", err)
			if err == nil {
				t.Fatal("departure from the contract was accepted")
			}
		})
	}
}

func TestHarnessRejectsUnknownRoute(t *testing.T) {
	t.Parallel()
	h := stub(t, map[string]route{"/v1/authors": {200, "application/json", `{}`}})
	if _, err := h.Get(t.Context(), "/v1/authors", nil, nil); err == nil {
		t.Fatal("a route outside the spec was accepted")
	}
}
