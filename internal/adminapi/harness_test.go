package adminapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/adapters/fixturesrc"
	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/adminapi"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/selection"
)

// Fake principals only; the real operator list is private.
const (
	principalOp1    = "user:fake-op-1"
	principalOp2    = "user:fake-op-2"
	principalDeploy = "serviceAccount:fake-deploy"
	principalOther  = "user:fake-stranger"
)

func testNowFn() time.Time { return time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC) }

// headerAuth takes the principal from a test header; empty fails.
type headerAuth struct{}

var errNoPrincipal = errors.New("no principal")

func (headerAuth) Authenticate(r *http.Request) (string, error) {
	if p := r.Header.Get("X-Test-Principal"); p != "" {
		return p, nil
	}
	return "", errNoPrincipal
}

// failingDocs makes ApplyLogged and Record fail, to prove nothing is applied.
type failingDocs struct {
	data.PlatformDocs

	fail bool
}

var errInjected = errors.New("injected log failure")

func (f *failingDocs) ApplyLogged(ctx context.Context, c data.Change, e data.LogEntry) error {
	if f.fail {
		return errInjected
	}
	return f.PlatformDocs.ApplyLogged(ctx, c, e) //nolint:wrapcheck // test double
}

type failingModeration struct {
	data.Moderation

	fail *bool
}

func (f failingModeration) Record(ctx context.Context, e data.LogEntry) error {
	if *f.fail {
		return errInjected
	}
	return f.Moderation.Record(ctx, e) //nolint:wrapcheck // test double
}

type env struct {
	t     *testing.T
	store *memory.Store
	docs  *failingDocs
	modF  *bool
	srv   http.Handler
}

func newEnv(t *testing.T) *env {
	t.Helper()
	store := memory.New()
	ports := store.PlatformPorts()
	docs := &failingDocs{PlatformDocs: ports.PlatformDocs}
	modFail := false
	ports.PlatformDocs = docs
	ports.Moderation = failingModeration{Moderation: ports.Moderation, fail: &modFail}
	src, err := fixturesrc.New(fixturesrc.Config{FS: os.DirFS("../../fixtures"), Now: testNowFn})
	if err != nil {
		t.Fatalf("fixturesrc: %v", err)
	}
	s, err := adminapi.New(adminapi.Deps{
		Ports: ports, Fetcher: src, Source: "fixtures", Auth: headerAuth{}, Now: testNowFn,
		Config: adminapi.Config{
			Operators: []adminapi.Operator{
				{ID: "op-1", Name: "Operator One", Principal: principalOp1},
				{ID: "op-2", Name: "Operator Two", Principal: principalOp2},
			},
			DeployPrincipal: principalDeploy,
		},
	})
	if err != nil {
		t.Fatalf("adminapi.New: %v", err)
	}
	return &env{t: t, store: store, docs: docs, modF: &modFail, srv: s.Handler()}
}

// publishV1 stores the built-in policy directly.
func (e *env) publishV1() {
	e.t.Helper()
	p, err := selection.V1(testNowFn())
	if err != nil {
		e.t.Fatal(err)
	}
	ports := e.store.PlatformPorts()
	store := struct {
		data.PublicDocs
		data.PlatformDocs
	}{ports.PublicDocs, ports.PlatformDocs}
	if err := selection.Publish(e.t.Context(), store, p, "initial", testNowFn()); err != nil {
		e.t.Fatal(err)
	}
}

// call sends a JSON request as principal and returns the status and body.
func (e *env) call(principal, method, path string, body any) (int, []byte) {
	e.t.Helper()
	var rd *bytes.Reader
	if body == nil {
		rd = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(body)
		if err != nil {
			e.t.Fatal(err)
		}
		rd = bytes.NewReader(raw)
	}
	req := httptest.NewRequestWithContext(e.t.Context(), method, path, rd)
	if principal != "" {
		req.Header.Set("X-Test-Principal", principal)
	}
	rec := httptest.NewRecorder()
	e.srv.ServeHTTP(rec, req)
	return rec.Code, rec.Body.Bytes()
}

// reason is a body with only a reason.
func reason(s string) map[string]string { return map[string]string{"reason": s} }

func (e *env) log() []data.LogEntry {
	e.t.Helper()
	var out []data.LogEntry
	cursor := ""
	for {
		page, err := e.store.PublicReadPorts().PublicDocs.ListLog(e.t.Context(), data.Page{Cursor: cursor})
		if err != nil {
			e.t.Fatal(err)
		}
		out = append(out, page.Items...)
		if page.Next == "" {
			return out
		}
		cursor = page.Next
	}
}

func mustContain(t *testing.T, body []byte, want string) {
	t.Helper()
	if !strings.Contains(string(body), want) {
		t.Fatalf("body %s lacks %q", body, want)
	}
}

func newReq(t *testing.T, principal, path string) (*http.Request, *httptest.ResponseRecorder) {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	req.Header.Set("X-Test-Principal", principal)
	return req, httptest.NewRecorder()
}
