package readapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	apiv1 "github.com/easterthebunny/pubcommittee/api/v1"
	"github.com/easterthebunny/pubcommittee/internal/httpx"
)

// Options configures Handler.
type Options struct {
	// RateLimits per route class; nil means the defaults.
	RateLimits RateLimits
	// ClientIPHeader names a header whose last comma-separated value is the
	// client address, for a deployment behind a trusted proxy. Empty uses the
	// socket address. The address is only ever hashed in memory.
	ClientIPHeader string
	// Logger receives one line per request; nil means slog.Default.
	Logger *slog.Logger
	// Now is the clock; nil means time.Now.
	Now func() time.Time
	// UIURL is the public origin of the web UI, which serves the
	// machine-access notice; discovery links to it. Empty means the link is
	// the bare well-known path.
	UIURL string
}

// Handler returns the full read-api handler: routes from the spec, then
// rate limiting, CORS, and request logging.
func Handler(s *Server, opts Options) (http.Handler, error) {
	if opts.RateLimits == nil {
		opts.RateLimits = DefaultRateLimits()
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	s.noticeHref = strings.TrimRight(opts.UIURL, "/") + NoticePath
	lim, err := newLimiter(opts.RateLimits, opts.ClientIPHeader, opts.Now)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	apiv1.HandlerWithOptions(s, apiv1.StdHTTPServerOptions{BaseRouter: mux, ErrorHandlerFunc: bindError})
	mux.HandleFunc("GET /robots.txt", robots)
	mux.HandleFunc("/", fallback)
	return logRequests(opts.Logger, cors(lim.middleware(mux))), nil
}

// fallback answers every request no spec route matched.
func fallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD, OPTIONS")
		httpx.WriteProblem(w, http.StatusMethodNotAllowed, "", "this API is read-only")
		return
	}
	notFound(w, "no such resource")
}

// cors lets any origin read, without credentials, and answers preflights.
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Expose-Headers", "ETag, Cache-Control, Content-Location, Retry-After")
		if r.Method == http.MethodOptions {
			h.Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "If-None-Match")
			h.Set("Access-Control-Max-Age", "86400")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// statusWriter records the status and size of a response.
type statusWriter struct {
	http.ResponseWriter

	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	if err != nil {
		return n, fmt.Errorf("write: %w", err)
	}
	return n, nil
}

// Unwrap lets http.ResponseController reach the real writer.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// logRequests writes one line per request: method, route pattern, status,
// size, and duration. It never logs client addresses, cookies, headers,
// queries, or bodies.
func logRequests(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		pattern := r.Pattern
		if pattern == "" {
			pattern = "unmatched"
		}
		log.LogAttrs(r.Context(), slog.LevelInfo, "request",
			slog.String("method", r.Method), slog.String("route", pattern),
			slog.Int("status", sw.status), slog.Int("bytes", sw.bytes),
			slog.String("duration", strconv.FormatInt(time.Since(start).Milliseconds(), 10)+"ms"),
		)
	})
}

// NoticePath is the well-known path of the machine-access notice, served by
// the web UI (design/10 § Published policy).
const NoticePath = "/.well-known/machine-access.json"

// robotsTxt allows every public path. The API has no sign-in and no write
// routes; the web UI's own robots.txt disallows those of the UI.
const robotsTxt = `# All public content may be read by machines (design/10 Published policy).
# This host has no sign-in and no write routes.
User-agent: *
Allow: /
`

// robots answers the crawler directives of the API host.
func robots(w http.ResponseWriter, r *http.Request) {
	send(w, r, response{
		status: http.StatusOK, contentType: "text/plain; charset=utf-8", body: []byte(robotsTxt), class: cacheShort,
	})
}
