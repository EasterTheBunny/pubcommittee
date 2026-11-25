package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/adminapi"
)

const testBill = "us-119-hr-4313"

// recorded is one request seen by the fake admin-api.
type recorded struct {
	method string
	path   string
	query  string
	body   []byte
	header http.Header
}

// fake is a fake admin-api that records requests and answers with a fixed status and body.
type fake struct {
	mu   sync.Mutex
	reqs []recorded
}

func (f *fake) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.reqs)
}

func (f *fake) last(t *testing.T) recorded {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.reqs) == 0 {
		t.Fatal("no request reached the fake admin-api")
	}
	return f.reqs[len(f.reqs)-1]
}

// newFake starts a fake admin-api that answers every request with status and answer.
func newFake(t *testing.T, status int, answer string) (*httptest.Server, *fake) {
	t.Helper()
	f := &fake{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.reqs = append(f.reqs, recorded{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, body: body, header: r.Header.Clone()})
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, answer)
	}))
	t.Cleanup(srv.Close)
	return srv, f
}

// invoke runs pubctl against url with no environment token and returns the outputs.
func invoke(t *testing.T, url string, args ...string) (string, string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	noEnv := func(string) string { return "" }
	all := append([]string{"--url", url}, args...)
	err := run(context.Background(), all, noEnv, &out, &errOut)
	return out.String(), errOut.String(), err
}

// bodyFields decodes a recorded JSON body into a map.
func bodyFields(t *testing.T, raw []byte) map[string]json.RawMessage {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("body is not a JSON object: %v: %s", err, raw)
	}
	return m
}

func TestMissingReasonSendsNothing(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args []string
	}{
		{"track", []string{"bills", "track", testBill}},
		{"untrack", []string{"bills", "untrack", testBill}},
		{"reingest", []string{"bills", "reingest", testBill}},
		{"reparse", []string{"bills", "reparse", testBill, "--version", "1"}},
		{"patch", []string{"bills", "catalog", "patch", testBill, "--title", "T"}},
		{"generations", []string{"generations", "create", "--task", "t", "--bill", testBill, "--version", "1"}},
		{"policy", []string{"selection-policy", "publish", "--builtin", "v1"}},
		{"blank reason", []string{"bills", "track", testBill, "--reason", "   "}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv, f := newFake(t, http.StatusOK, "{}")
			_, _, err := invoke(t, srv.URL, tc.args...)
			if err == nil || !strings.Contains(err.Error(), "--reason is required") {
				t.Fatalf("err = %v, want --reason is required", err)
			}
			if n := f.count(); n != 0 {
				t.Fatalf("requests = %d, want 0", n)
			}
		})
	}
}

func TestBillsTrackPostsReason(t *testing.T) {
	t.Parallel()
	srv, f := newFake(t, http.StatusOK, `{"bill":"us-119-hr-4313","changed":true}`)
	out, _, err := invoke(t, srv.URL, "bills", "track", testBill, "--reason", "public note")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	req := f.last(t)
	if req.method != http.MethodPost {
		t.Errorf("method = %s, want POST", req.method)
	}
	if want := "/admin/v1/bills/" + testBill + "/track"; req.path != want {
		t.Errorf("path = %s, want %s", req.path, want)
	}
	var body adminapi.Reasoned
	if err := json.Unmarshal(req.body, &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Reason != "public note" {
		t.Errorf("reason = %q, want %q", body.Reason, "public note")
	}
	if !strings.Contains(out, `"changed": true`) {
		t.Errorf("output lacks the answer: %s", out)
	}
}

func TestBillsTrackPullFailureExitsNonZero(t *testing.T) {
	t.Parallel()
	srv, _ := newFake(t, http.StatusOK, `{"bill":"us-119-hr-4313","changed":true,"pull_error":"source down"}`)
	out, _, err := invoke(t, srv.URL, "bills", "track", testBill, "--reason", "r")
	if err == nil || err.Error() != "the pull reported failures" {
		t.Fatalf("err = %v, want the pull reported failures", err)
	}
	if !strings.Contains(out, "source down") {
		t.Errorf("output lacks pull_error before the error: %s", out)
	}
}

func TestCatalogPatchSendsOnlySetFlags(t *testing.T) {
	t.Parallel()
	srv, f := newFake(t, http.StatusOK, "{}")
	_, _, err := invoke(t, srv.URL, "bills", "catalog", "patch", testBill,
		"--reason", "fix title", "--title", "New title", "--subjects", "a, b,")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	req := f.last(t)
	if req.method != http.MethodPatch {
		t.Errorf("method = %s, want PATCH", req.method)
	}
	if want := "/admin/v1/bills/" + testBill + "/catalog"; req.path != want {
		t.Errorf("path = %s, want %s", req.path, want)
	}
	fields := bodyFields(t, req.body)
	want := []string{"reason", "subjects", "title"}
	if len(fields) != len(want) {
		t.Errorf("body keys = %v, want exactly %v", keys(fields), want)
	}
	for _, k := range want {
		if _, ok := fields[k]; !ok {
			t.Errorf("body lacks %q: %s", k, req.body)
		}
	}
	checkSubjects(t, fields["subjects"], []string{"a", "b"})
}

// checkSubjects decodes a subjects field and compares it with want.
func checkSubjects(t *testing.T, raw json.RawMessage, want []string) {
	t.Helper()
	var got []string
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode subjects: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("subjects = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("subjects = %v, want %v", got, want)
		}
	}
}

func TestCatalogPatchNeedsOneField(t *testing.T) {
	t.Parallel()
	srv, f := newFake(t, http.StatusOK, "{}")
	_, _, err := invoke(t, srv.URL, "bills", "catalog", "patch", testBill, "--reason", "r")
	if err == nil || !strings.Contains(err.Error(), "at least one catalog flag") {
		t.Fatalf("err = %v, want at least one catalog flag", err)
	}
	if n := f.count(); n != 0 {
		t.Fatalf("requests = %d, want 0", n)
	}
}

func TestSelectionPolicyPublishBuiltinV1(t *testing.T) {
	t.Parallel()
	srv, f := newFake(t, http.StatusOK, `{"version":1,"changed":true}`)
	_, _, err := invoke(t, srv.URL, "selection-policy", "publish", "--builtin", "v1", "--reason", "pilot")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	req := f.last(t)
	if req.method != http.MethodPost || req.path != "/admin/v1/selection-policy" {
		t.Fatalf("request = %s %s, want POST /admin/v1/selection-policy", req.method, req.path)
	}
	var body struct {
		Reason string `json:"reason"`
		Policy struct {
			Version int `json:"version"`
		} `json:"policy"`
	}
	if err := json.Unmarshal(req.body, &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Policy.Version != 1 {
		t.Errorf("policy version = %d, want 1", body.Policy.Version)
	}
	if body.Reason != "pilot" {
		t.Errorf("reason = %q, want pilot", body.Reason)
	}
}

func TestSelectionPolicyNeedsExactlyOneSource(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args []string
	}{
		{"neither", []string{"selection-policy", "publish", "--reason", "r"}},
		{"both", []string{"selection-policy", "publish", "--builtin", "v1", "--file", "x.json", "--reason", "r"}},
		{"unknown builtin", []string{"selection-policy", "publish", "--builtin", "v9", "--reason", "r"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv, f := newFake(t, http.StatusOK, "{}")
			if _, _, err := invoke(t, srv.URL, tc.args...); err == nil {
				t.Fatal("err = nil, want an error")
			}
			if n := f.count(); n != 0 {
				t.Fatalf("requests = %d, want 0", n)
			}
		})
	}
}

func TestUnknownCommandErrors(t *testing.T) {
	t.Parallel()
	srv, f := newFake(t, http.StatusOK, "{}")
	_, errOut, err := invoke(t, srv.URL, "nope")
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("err = %v, want unknown command", err)
	}
	for _, want := range []string{"bills track", "selection-policy publish", "jobs"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("usage lacks %q: %s", want, errOut)
		}
	}
	if n := f.count(); n != 0 {
		t.Fatalf("requests = %d, want 0", n)
	}
}

func TestForbiddenSurfacesStatus(t *testing.T) {
	t.Parallel()
	srv, _ := newFake(t, http.StatusForbidden, `{"title":"Forbidden","detail":"not on the roster"}`)
	_, _, err := invoke(t, srv.URL, "bills", "track", testBill, "--reason", "r")
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("err = %v, want an error containing 403", err)
	}
}

func TestJobsIsGET(t *testing.T) {
	t.Parallel()
	srv, f := newFake(t, http.StatusOK, `{"sources":[]}`)
	if _, _, err := invoke(t, srv.URL, "jobs"); err != nil {
		t.Fatalf("run: %v", err)
	}
	req := f.last(t)
	if req.method != http.MethodGet || req.path != "/admin/v1/jobs" {
		t.Fatalf("request = %s %s, want GET /admin/v1/jobs", req.method, req.path)
	}
}

func TestIngestFailuresSendsSources(t *testing.T) {
	t.Parallel()
	srv, f := newFake(t, http.StatusOK, `{"sources":[]}`)
	_, _, err := invoke(t, srv.URL, "ingest", "failures", "--source", "govinfo/119", "--source", "congress/119")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	req := f.last(t)
	if req.path != "/admin/v1/ingest/failures" {
		t.Errorf("path = %s", req.path)
	}
	if want := "source=govinfo%2F119&source=congress%2F119"; req.query != want {
		t.Errorf("query = %s, want %s", req.query, want)
	}
}

func TestMissingURLErrors(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	err := run(context.Background(), []string{"jobs"}, func(string) string { return "" }, &out, &errOut)
	if err == nil || err.Error() != "set --url or PUBCTL_URL" {
		t.Fatalf("err = %v, want set --url or PUBCTL_URL", err)
	}
}

func TestHelpReturnsNil(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	if err := run(context.Background(), []string{"-h"}, func(string) string { return "" }, &out, &errOut); err != nil {
		t.Fatalf("-h returned %v, want nil", err)
	}
	if !strings.Contains(errOut.String(), "bills track") {
		t.Errorf("help lacks command list: %s", errOut.String())
	}
}

// keys lists the keys of a decoded object, for error messages.
func keys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestTokenSentInBothHeaders: Cloud Run checks X-Serverless-Authorization and
// passes Authorization on with its signature, for admin-api to verify.
func TestTokenSentInBothHeaders(t *testing.T) {
	t.Parallel()
	srv, f := newFake(t, http.StatusOK, `{"sources":[]}`)
	env := func(k string) string {
		if k == envToken {
			return "tok"
		}
		return ""
	}
	var out, errOut bytes.Buffer
	if err := run(context.Background(), []string{"--url", srv.URL, "jobs"}, env, &out, &errOut); err != nil {
		t.Fatalf("run: %v", err)
	}
	h := f.last(t).header
	for _, k := range []string{"Authorization", "X-Serverless-Authorization"} {
		if got := h.Get(k); got != "Bearer tok" {
			t.Errorf("%s = %q, want %q", k, got, "Bearer tok")
		}
	}
}
