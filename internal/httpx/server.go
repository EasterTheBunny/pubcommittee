// Package httpx is the shared HTTP bootstrap for PubCommittee services: the
// application and metrics servers, health and readiness routes, and RFC 9457
// problem responses. It uses only the standard library.
package httpx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

const (
	defaultPort            = "8080"
	defaultMetricsPort     = "9464"
	maxPort                = 65535
	defaultShutdownTimeout = 10 * time.Second
	readHeaderTimeout      = 5 * time.Second
	readyTimeout           = 2 * time.Second
	healthyBody            = "ok"
	readyBody              = "ready"
	listenHost             = "0.0.0.0"
	serverCount            = 2
)

// errPort is wrapped by ConfigFromEnv when a port value is out of range.
var errPort = errors.New("must be an integer from 1 to 65535")

// Config holds the listen ports and the graceful shutdown budget of a Server.
type Config struct {
	// Port is the application port.
	Port string
	// MetricsPort is the port that serves /metrics.
	MetricsPort string
	// ShutdownTimeout bounds graceful shutdown of both servers.
	ShutdownTimeout time.Duration
}

// ConfigFromEnv reads PORT and METRICS_PORT through getenv, applying defaults
// for unset values. A port that is not an integer from 1 to 65535 is an error
// that names the variable.
func ConfigFromEnv(getenv func(string) string) (Config, error) {
	cfg := Config{
		Port:            envOr(getenv, "PORT", defaultPort),
		MetricsPort:     envOr(getenv, "METRICS_PORT", defaultMetricsPort),
		ShutdownTimeout: defaultShutdownTimeout,
	}
	if err := checkPort("PORT", cfg.Port); err != nil {
		return Config{}, err
	}
	if err := checkPort("METRICS_PORT", cfg.MetricsPort); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func envOr(getenv func(string) string, name, fallback string) string {
	if v := getenv(name); v != "" {
		return v
	}
	return fallback
}

func checkPort(name, value string) error {
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 || n > maxPort {
		return fmt.Errorf("%s: %w, got %q", name, errPort, value)
	}
	return nil
}

// Checker reports whether one dependency is ready. A nil error means ready.
type Checker func(ctx context.Context) error

// Server runs the application and metrics HTTP servers.
type Server struct {
	cfg     Config
	app     *http.ServeMux
	metrics *http.ServeMux
	checks  []Checker
}

// New builds a Server. handler serves the application routes; metrics serves
// /metrics on the metrics port (nil means a handler that returns 404); checks
// run on /readyz.
func New(cfg Config, handler http.Handler, metrics http.Handler, checks ...Checker) *Server {
	if handler == nil {
		handler = http.NotFoundHandler()
	}
	if metrics == nil {
		metrics = http.NotFoundHandler()
	}
	s := &Server{cfg: cfg, app: http.NewServeMux(), metrics: http.NewServeMux(), checks: checks}
	s.app.HandleFunc("GET /healthz", healthz)
	s.app.HandleFunc("GET /readyz", s.readyz)
	s.app.Handle("/", handler)
	s.metrics.Handle("GET /metrics", metrics)
	s.metrics.Handle("/", http.NotFoundHandler())
	return s
}

// Run listens on 0.0.0.0 at the application and metrics ports, then calls
// Serve.
func (s *Server) Run(ctx context.Context) error {
	var lc net.ListenConfig
	app, err := lc.Listen(ctx, "tcp", net.JoinHostPort(listenHost, s.cfg.Port))
	if err != nil {
		return fmt.Errorf("listen app port %s: %w", s.cfg.Port, err)
	}
	metrics, err := lc.Listen(ctx, "tcp", net.JoinHostPort(listenHost, s.cfg.MetricsPort))
	if err != nil {
		_ = app.Close()
		return fmt.Errorf("listen metrics port %s: %w", s.cfg.MetricsPort, err)
	}
	return s.Serve(ctx, app, metrics)
}

// Serve serves on the given listeners until ctx is canceled, then shuts both
// servers down gracefully within the configured ShutdownTimeout so in-flight
// requests finish. It returns nil on clean shutdown and the serve error
// otherwise.
func (s *Server) Serve(ctx context.Context, app, metrics net.Listener) error {
	appSrv := s.httpServer(s.app)
	metricsSrv := s.httpServer(s.metrics)
	errc := make(chan error, serverCount)
	go func() { errc <- appSrv.Serve(app) }()
	go func() { errc <- metricsSrv.Serve(metrics) }()

	var errs []error
	pending := serverCount
	select {
	case <-ctx.Done():
	case err := <-errc:
		// One server stopped before cancel; stop the other one too.
		pending--
		errs = append(errs, err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cfg.ShutdownTimeout)
	defer cancel()
	errs = append(errs, appSrv.Shutdown(shutdownCtx), metricsSrv.Shutdown(shutdownCtx))
	for ; pending > 0; pending-- {
		errs = append(errs, <-errc)
	}
	return serveResult(errs)
}

// RunSignals runs the Server until SIGTERM or os.Interrupt, then shuts it down.
func (s *Server) RunSignals() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	return s.Run(ctx)
}

func (s *Server) httpServer(h http.Handler) *http.Server {
	srv := new(http.Server)
	srv.Handler = h
	srv.ReadHeaderTimeout = readHeaderTimeout
	return srv
}

// serveResult drops the expected http.ErrServerClosed and joins the rest.
func serveResult(errs []error) error {
	var out []error
	for _, err := range errs {
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			out = append(out, err)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return fmt.Errorf("serve: %w", errors.Join(out...))
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	writeText(w, http.StatusOK, healthyBody)
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	failing := 0
	for _, check := range s.checks {
		if !passes(r.Context(), check) {
			failing++
		}
	}
	if failing > 0 {
		slog.WarnContext(r.Context(), "readiness checks failing", "failing", failing)
		WriteProblem(w, http.StatusServiceUnavailable, "", fmt.Sprintf("%d readiness check(s) failing", failing))
		return
	}
	writeText(w, http.StatusOK, readyBody)
}

// passes runs one check with its own readyTimeout. A check that outlives the
// timeout counts as failing; its goroutine ends when the check returns.
func passes(parent context.Context, check Checker) bool {
	ctx, cancel := context.WithTimeout(parent, readyTimeout)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- check(ctx) }()
	select {
	case err := <-done:
		return err == nil
	case <-ctx.Done():
		return false
	}
}

func writeText(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}
