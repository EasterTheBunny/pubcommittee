package adminapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/httpx"
	"github.com/easterthebunny/pubcommittee/internal/ingest"
)

const (
	maxBody      = 1 << 20
	maxReasonLen = 500
	hoursPerDay  = 24
)

// Authenticator verifies the caller of a request and returns its principal.
// On gcp it verifies a Google ID token; on local and memory targets a local
// adapter stands in. Any error is answered 401.
type Authenticator interface {
	Authenticate(r *http.Request) (principal string, err error)
}

// Deps are the collaborators of the handler.
type Deps struct {
	Ports data.PlatformPorts
	// Fetcher fetches bills from the official source (or fixtures); nil
	// where the target has none, which makes track and reingest unavailable.
	Fetcher data.BillFetcher
	// Generator submits generation tasks; nil until M1-09 provides one.
	Generator data.Generator
	Auth      Authenticator
	Config    Config
	// Source names the ingest_state key prefix, for example "govinfo".
	Source string
	// Steps builds the pull pipeline; nil means the default pipeline with the
	// stored selection policy.
	Steps func(ctx context.Context, d ingest.Deps) (ingest.Pipeline, error)
	Log   *slog.Logger
	Now   func() time.Time
}

// Server is the admin-api handler.
type Server struct {
	Deps
}

// New returns a Server. Auth is required.
func New(d Deps) (*Server, error) {
	if d.Auth == nil {
		return nil, fmt.Errorf("admin-api needs an authenticator: %w", data.ErrInvalid)
	}
	if err := d.Config.Validate(); err != nil {
		return nil, err
	}
	if d.Log == nil {
		d.Log = slog.New(slog.DiscardHandler)
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Server{Deps: d}, nil
}

type ctxKey struct{}

// operatorOf returns the operator that authenticated the request.
func operatorOf(ctx context.Context) Operator {
	op, _ := ctx.Value(ctxKey{}).(Operator)
	return op
}

// Handler returns the routes. Every route authenticates first; no route sends
// CORS headers.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	op := func(h http.HandlerFunc) http.Handler { return s.guard(RoleOperator, h) }
	mux.Handle("POST "+PathPrefix+"/bills/{bill}/track", op(s.track))
	mux.Handle("POST "+PathPrefix+"/bills/{bill}/untrack", op(s.untrack))
	mux.Handle("PATCH "+PathPrefix+"/bills/{bill}/catalog", op(s.patchCatalog))
	mux.Handle("POST "+PathPrefix+"/bills/{bill}/reingest", op(s.reingest))
	mux.Handle("POST "+PathPrefix+"/bills/{bill}/v/{n}/reparse", op(s.reparse))
	mux.Handle("POST "+GenerationRoute, op(s.generate))
	mux.Handle("POST "+PolicyRoute, op(s.publishPolicy))
	mux.Handle("GET "+FailuresRoute, op(s.failures))
	mux.Handle("GET "+JobsRoute, op(s.jobs))
	mux.Handle("POST "+RosterRoute, s.guard(RoleDeploy, s.roster))
	return mux
}

// guard authenticates the request and lets through only the wanted role: a
// principal not on the list is refused even with project IAM access, the
// deploy principal reaches the roster route only, and operators do not.
func (s *Server) guard(want Role, next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, err := s.Auth.Authenticate(r)
		if err != nil {
			httpx.WriteProblem(w, http.StatusUnauthorized, "", "authentication failed")
			return
		}
		role, op := s.Config.Identify(principal)
		if role != want {
			// Neither the principal nor the role wanted is logged: a refusal is not an action.
			httpx.WriteProblem(w, http.StatusForbidden, "", "this caller may not use this route")
			return
		}
		ctx := context.WithValue(r.Context(), ctxKey{}, op)
		next(w, r.WithContext(ctx))
	})
}

// decode reads a JSON body, refusing unknown fields and a missing reason.
func decode(w http.ResponseWriter, r *http.Request, v any, reason func() string) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil && !errors.Is(err, io.EOF) {
		httpx.WriteProblem(w, http.StatusBadRequest, "", "request body is not valid JSON for this action")
		return false
	}
	if !validReason(reason()) {
		httpx.WriteProblem(w, http.StatusUnprocessableEntity, "",
			fmt.Sprintf("a reason is required (1 to %d characters); it is published", maxReasonLen))
		return false
	}
	return true
}

func validReason(s string) bool {
	n := utf8.RuneCountInString(strings.TrimSpace(s))
	return n > 0 && n <= maxReasonLen
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("write response", "err", err)
	}
}

// fail answers an error: typed data errors map to a status and keep their
// message, which is for operators; anything else is 500 with a generic text.
func (s *Server) fail(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, data.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, data.ErrInvalid):
		status = http.StatusUnprocessableEntity
	case errors.Is(err, data.ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, data.ErrUnavailable), errors.Is(err, data.ErrRateLimited):
		status = http.StatusServiceUnavailable
	}
	detail := http.StatusText(status)
	if status != http.StatusInternalServerError {
		detail = err.Error()
	} else {
		s.Log.Error("admin action failed", "error", err.Error())
	}
	httpx.WriteProblem(w, status, "", detail)
}

// day coarsens a time to the public log's day.
func day(t time.Time) time.Time {
	return t.UTC().Truncate(hoursPerDay * time.Hour)
}

// logID makes a unique id for an action's log entry.
func (s *Server) logID(kind, subject string) string {
	stamp := s.Now().UTC().Format("20060102T150405.000000000Z")
	return fmt.Sprintf("%s-%s-%s", kind, strings.ReplaceAll(subject, "/", "-"), stamp)
}

func (s *Server) meta() data.Meta {
	return data.Meta{
		Version: 1, AsOf: s.Now().UTC(), Provenance: data.ProvenanceSource, ContentClass: data.ContentPlatform,
	}
}

// entry builds the public log entry of an action by operator op.
func (s *Server) entry(kind, subject, reason string, op Operator) data.LogEntry {
	return data.LogEntry{
		ID: s.logID(kind, subject), At: day(s.Now()), Kind: kind, Subject: subject,
		Reason: strings.TrimSpace(reason), Actor: op.logActor(), Meta: s.meta(),
	}
}
