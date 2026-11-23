package httpx_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/httpx"
)

const (
	shutdownBudget = 10 * time.Second
	slowDelay      = 300 * time.Millisecond
	clientTimeout  = 5 * time.Second
	problemType    = "application/problem+json"
	envPort        = "PORT"
	envMetricsPort = "METRICS_PORT"
)

// errSecret stands in for a checker error that must never reach a response.
var errSecret = errors.New("db password hunter2 at 10.0.0.1")

// errBoom stands in for an untyped error with internal detail.
var errBoom = errors.New("boom: internal detail")

func testConfig() httpx.Config {
	return httpx.Config{Port: "0", MetricsPort: "0", ShutdownTimeout: shutdownBudget}
}

func listen(t *testing.T) net.Listener {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return ln
}

// running is a Server serving on loopback listeners started by start.
type running struct {
	app     string
	metrics string
	cancel  context.CancelFunc
	done    chan struct{}
	err     error
}

func start(t *testing.T, srv *httpx.Server) *running {
	t.Helper()
	app, met := listen(t), listen(t)
	ctx, cancel := context.WithCancel(t.Context())
	r := new(running)
	r.app = "http://" + app.Addr().String()
	r.metrics = "http://" + met.Addr().String()
	r.cancel = cancel
	r.done = make(chan struct{})
	go func() {
		defer close(r.done)
		r.err = srv.Serve(ctx, app, met)
	}()
	t.Cleanup(func() {
		cancel()
		<-r.done
	})
	return r
}

// stop cancels the server and returns Serve's result, failing if Serve takes
// longer than the shutdown budget.
func (r *running) stop(t *testing.T) error {
	t.Helper()
	r.cancel()
	select {
	case <-r.done:
		return r.err
	case <-time.After(shutdownBudget):
		t.Fatal("Serve did not return within the shutdown budget")
		return nil
	}
}

// fetch performs a GET without failing the test, so it is safe in goroutines.
func fetch(ctx context.Context, url string) (int, string, error) {
	ctx, cancel := context.WithTimeout(ctx, clientTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, "", fmt.Errorf("new request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("get %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", fmt.Errorf("read body: %w", err)
	}
	return resp.StatusCode, string(body), nil
}

func get(t *testing.T, url string) (int, string) {
	t.Helper()
	status, body, err := fetch(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}
	return status, body
}

func writeBody(text string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, text)
	})
}

func TestHealthz(t *testing.T) {
	t.Parallel()
	r := start(t, httpx.New(testConfig(), nil, nil))
	status, body := get(t, r.app+"/healthz")
	if status != http.StatusOK || body != "ok" {
		t.Fatalf("GET /healthz = %d %q, want 200 \"ok\"", status, body)
	}
}

func TestReadyz(t *testing.T) {
	t.Parallel()
	var failing atomic.Bool
	check := func(_ context.Context) error {
		if failing.Load() {
			return errSecret
		}
		return nil
	}
	r := start(t, httpx.New(testConfig(), nil, nil, check))

	status, body := get(t, r.app+"/readyz")
	if status != http.StatusOK || body != "ready" {
		t.Fatalf("GET /readyz passing = %d %q, want 200 \"ready\"", status, body)
	}

	failing.Store(true)
	status, body = get(t, r.app+"/readyz")
	if status != http.StatusServiceUnavailable {
		t.Fatalf("GET /readyz failing = %d, want 503", status)
	}
	if strings.Contains(body, errSecret.Error()) || strings.Contains(body, "hunter2") {
		t.Fatalf("503 body leaks checker error text: %q", body)
	}
	var p httpx.Problem
	if err := json.Unmarshal([]byte(body), &p); err != nil {
		t.Fatalf("503 body is not JSON: %v", err)
	}
	if p.Status != http.StatusServiceUnavailable || !strings.Contains(p.Detail, "1 readiness") {
		t.Fatalf("503 problem = %+v, want status 503 naming 1 failing check", p)
	}
}

func TestAppPortRoutes(t *testing.T) {
	t.Parallel()
	r := start(t, httpx.New(testConfig(), writeBody("app"), writeBody("metrics")))

	for _, path := range []string{"/api/things", "/metrics"} {
		status, body := get(t, r.app+path)
		if status != http.StatusOK || body != "app" {
			t.Fatalf("GET %s = %d %q, want 200 from app handler", path, status, body)
		}
	}
}

func TestMetricsPortRoutes(t *testing.T) {
	t.Parallel()
	r := start(t, httpx.New(testConfig(), writeBody("app"), writeBody("metrics-data")))

	status, body := get(t, r.metrics+"/metrics")
	if status != http.StatusOK || body != "metrics-data" {
		t.Fatalf("metrics GET /metrics = %d %q, want 200 from metrics handler", status, body)
	}
	status, _ = get(t, r.metrics+"/healthz")
	if status != http.StatusNotFound {
		t.Fatalf("metrics GET /healthz = %d, want 404", status)
	}
}

func TestGracefulShutdown(t *testing.T) {
	t.Parallel()
	started := make(chan struct{})
	slow := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		time.Sleep(slowDelay)
		_, _ = io.WriteString(w, "done")
	})
	r := start(t, httpx.New(testConfig(), slow, nil))

	type result struct {
		status int
		body   string
		err    error
	}
	res := make(chan result, 1)
	go func() {
		status, body, err := fetch(t.Context(), r.app+"/slow")
		res <- result{status: status, body: body, err: err}
	}()

	select {
	case <-started:
	case <-time.After(shutdownBudget):
		t.Fatal("slow request never reached the handler")
	}
	r.cancel()

	select {
	case out := <-res:
		if out.err != nil {
			t.Fatalf("in-flight request failed during shutdown: %v", out.err)
		}
		if out.status != http.StatusOK || out.body != "done" {
			t.Fatalf("in-flight request = %d %q, want 200 \"done\"", out.status, out.body)
		}
	case <-time.After(shutdownBudget):
		t.Fatal("in-flight request did not complete")
	}

	if err := r.stop(t); err != nil {
		t.Fatalf("Serve on clean shutdown = %v, want nil", err)
	}
}

func TestConfigFromEnv(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		env         map[string]string
		wantPort    string
		wantMetrics string
		wantErr     string
	}{
		{name: "defaults", env: nil, wantPort: "8080", wantMetrics: "9464", wantErr: ""},
		{
			name:        "valid",
			env:         map[string]string{envPort: "9000", envMetricsPort: "9100"},
			wantPort:    "9000",
			wantMetrics: "9100",
			wantErr:     "",
		},
		{name: "port not a number", env: map[string]string{envPort: "abc"}, wantPort: "", wantMetrics: "", wantErr: envPort},
		{name: "port zero", env: map[string]string{envPort: "0"}, wantPort: "", wantMetrics: "", wantErr: envPort},
		{name: "port too large", env: map[string]string{envPort: "70000"}, wantPort: "", wantMetrics: "", wantErr: envPort},
		{
			name:        "metrics port not a number",
			env:         map[string]string{envMetricsPort: "abc"},
			wantPort:    "",
			wantMetrics: "",
			wantErr:     envMetricsPort,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			getenv := func(k string) string { return tt.env[k] }
			cfg, err := httpx.ConfigFromEnv(getenv)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want one naming %s", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Port != tt.wantPort || cfg.MetricsPort != tt.wantMetrics {
				t.Fatalf("ports = %q/%q, want %q/%q", cfg.Port, cfg.MetricsPort, tt.wantPort, tt.wantMetrics)
			}
			if cfg.ShutdownTimeout != 10*time.Second {
				t.Fatalf("ShutdownTimeout = %v, want 10s", cfg.ShutdownTimeout)
			}
		})
	}
}

func TestWriteProblem(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	httpx.WriteProblem(rec, http.StatusUnprocessableEntity, "", "name is required")

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != problemType {
		t.Fatalf("Content-Type = %q, want %q", got, problemType)
	}
	var p httpx.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	want := httpx.Problem{
		Type:   "/problems/invalid",
		Title:  http.StatusText(http.StatusUnprocessableEntity),
		Status: http.StatusUnprocessableEntity,
		Detail: "name is required",
	}
	if p != want {
		t.Fatalf("problem = %+v, want %+v", p, want)
	}
}

func TestWriteProblemTitle(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	httpx.WriteProblem(rec, http.StatusTeapot, "Short and Stout", "")
	var p httpx.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if p.Title != "Short and Stout" || p.Type != "about:blank" || p.Detail != "" {
		t.Fatalf("problem = %+v, want explicit title, about:blank type, no detail", p)
	}
}

func TestProblemTypes(t *testing.T) {
	t.Parallel()
	want := map[int]string{
		http.StatusBadRequest:          "/problems/invalid",
		http.StatusNotFound:            "/problems/not-found",
		http.StatusConflict:            "/problems/conflict",
		http.StatusTooManyRequests:     "/problems/rate-limited",
		http.StatusServiceUnavailable:  "/problems/unavailable",
		http.StatusInternalServerError: "about:blank",
	}
	for status, typ := range want {
		rec := httptest.NewRecorder()
		httpx.WriteProblem(rec, status, "", "")
		var p httpx.Problem
		if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
			t.Fatalf("body is not JSON: %v", err)
		}
		if p.Type != typ {
			t.Errorf("status %d type = %q, want %q", status, p.Type, typ)
		}
	}
}

func TestProblemFor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "not found", err: data.ErrNotFound, status: http.StatusNotFound},
		{name: "wrapped not found", err: fmt.Errorf("x: %w", data.ErrNotFound), status: http.StatusNotFound},
		{name: "invalid", err: data.ErrInvalid, status: http.StatusBadRequest},
		{name: "wrapped invalid", err: fmt.Errorf("x: %w", data.ErrInvalid), status: http.StatusBadRequest},
		{name: "conflict", err: data.ErrConflict, status: http.StatusConflict},
		{name: "wrapped conflict", err: fmt.Errorf("x: %w", data.ErrConflict), status: http.StatusConflict},
		{name: "rate limited", err: data.ErrRateLimited, status: http.StatusTooManyRequests},
		{name: "wrapped rate limited", err: fmt.Errorf("x: %w", data.ErrRateLimited), status: http.StatusTooManyRequests},
		{name: "unavailable", err: data.ErrUnavailable, status: http.StatusServiceUnavailable},
		{name: "wrapped unavailable", err: fmt.Errorf("x: %w", data.ErrUnavailable), status: http.StatusServiceUnavailable},
		{name: "untyped", err: errBoom, status: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			httpx.ProblemFor(rec, tt.err)
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}
			body := rec.Body.String()
			if strings.Contains(body, errBoom.Error()) || strings.Contains(body, "internal detail") {
				t.Fatalf("body leaks error text: %q", body)
			}
			var p httpx.Problem
			if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
				t.Fatalf("body is not JSON: %v", err)
			}
			if p.Status != tt.status || p.Detail != http.StatusText(tt.status) {
				t.Fatalf("problem = %+v, want status %d and generic detail", p, tt.status)
			}
		})
	}
}
