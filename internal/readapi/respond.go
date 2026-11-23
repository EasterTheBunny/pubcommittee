// Package readapi implements the public read-only HTTP API of
// api/openapi.yaml on top of the read ports. It holds no SQL or key adapter.
package readapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/httpx"
)

// cacheClass is a route's cache lifetime class (x-cache in the spec).
type cacheClass int

const (
	// cacheShort is for live documents: catalog, bills, explanations, policy, log.
	cacheShort cacheClass = iota
	// cacheImmutable is for official version text and policy versions.
	cacheImmutable
	// cacheBatch is for feed and exports, which change at batch boundaries.
	cacheBatch
)

// Cache-Control values per class. Short serves stale for five minutes while
// revalidating, so review changes show within minutes without waking the
// backend on every view.
const (
	cacheControlShort     = "public, max-age=60, stale-while-revalidate=300"
	cacheControlImmutable = "public, max-age=31536000, immutable"
	cacheControlBatch     = "public, max-age=900"
	cacheControlNoStore   = "no-store"
)

func (c cacheClass) header() string {
	switch c {
	case cacheImmutable:
		return cacheControlImmutable
	case cacheBatch:
		return cacheControlBatch
	case cacheShort:
		return cacheControlShort
	}
	return cacheControlShort
}

// response is one successful body with its headers.
type response struct {
	status      int
	contentType string
	body        []byte
	class       cacheClass
	location    string // Content-Location, when the answer is pinned from a floating address
}

// etagOf returns the strong ETag of body: the quoted SHA-256 of its bytes.
func etagOf(body []byte) string {
	sum := sha256.Sum256(body)
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

// matches reports whether an If-None-Match header lists etag (weak tags
// compare equal, per RFC 9110 for GET).
func matches(header, etag string) bool {
	for part := range strings.SplitSeq(header, ",") {
		part = strings.TrimSpace(part)
		if part == "*" || strings.TrimPrefix(part, "W/") == etag {
			return true
		}
	}
	return false
}

// send writes the response with ETag and Cache-Control, answering 304 with no
// body when If-None-Match names the current tag.
func send(w http.ResponseWriter, r *http.Request, res response) {
	etag := etagOf(res.body)
	h := w.Header()
	h.Set("ETag", etag)
	h.Set("Cache-Control", res.class.header())
	if res.location != "" {
		h.Set("Content-Location", res.location)
	}
	if inm := r.Header.Get("If-None-Match"); inm != "" && matches(inm, etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	h.Set("Content-Type", res.contentType)
	h.Set("Content-Length", strconv.Itoa(len(res.body)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	if _, err := w.Write(res.body); err != nil {
		slog.WarnContext(r.Context(), "write response", "err", err)
	}
}

// sendJSON marshals v and sends it. A marshal failure is a 500 problem.
func sendJSON(w http.ResponseWriter, r *http.Request, class cacheClass, v any) {
	sendJSONAt(w, r, class, "", v)
}

// sendJSONAt is sendJSON with a Content-Location for floating addresses.
func sendJSONAt(w http.ResponseWriter, r *http.Request, class cacheClass, location string, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		slog.ErrorContext(r.Context(), "marshal response", "err", err)
		fail(w, r, err)
		return
	}
	send(w, r, response{
		status: http.StatusOK, contentType: "application/json", body: append(body, '\n'),
		class: class, location: location,
	})
}

// fail answers err as a problem. Typed data errors keep their status; the
// detail is generic, so no vendor error or stack trace reaches the client.
func fail(w http.ResponseWriter, r *http.Request, err error) {
	if !errors.Is(err, data.ErrNotFound) && !errors.Is(err, data.ErrInvalid) {
		slog.ErrorContext(r.Context(), "request failed", "route", r.Pattern, "err", err)
	}
	if errors.Is(err, data.ErrUnavailable) || !isTyped(err) {
		w.Header().Set("Cache-Control", cacheControlNoStore)
	}
	httpx.ProblemFor(w, err)
}

func isTyped(err error) bool {
	typed := []error{data.ErrNotFound, data.ErrInvalid, data.ErrConflict, data.ErrRateLimited, data.ErrUnavailable}
	for _, t := range typed {
		if errors.Is(err, t) {
			return true
		}
	}
	return false
}

// invalid answers 400 with a detail written by the handler (never err text).
func invalid(w http.ResponseWriter, detail string) {
	httpx.WriteProblem(w, http.StatusBadRequest, "", detail)
}

// notFound answers 404 with a handler-written detail.
func notFound(w http.ResponseWriter, detail string) {
	httpx.WriteProblem(w, http.StatusNotFound, "", detail)
}
