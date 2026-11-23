package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

const (
	problemContentType = "application/problem+json"
	aboutBlank         = "about:blank"
)

// Problem is an RFC 9457 problem details object.
type Problem struct {
	Type   string `json:"type,omitempty"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// problemType returns the stable problem type for a status: the path form
// /problems/<name> for statuses that map to a typed data error, about:blank
// otherwise.
func problemType(status int) string {
	switch status {
	case http.StatusNotFound:
		return "/problems/not-found"
	case http.StatusConflict:
		return "/problems/conflict"
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return "/problems/invalid"
	case http.StatusTooManyRequests:
		return "/problems/rate-limited"
	case http.StatusServiceUnavailable:
		return "/problems/unavailable"
	}
	return aboutBlank
}

// WriteProblem writes an RFC 9457 application/problem+json response with the
// given status. An empty title becomes http.StatusText(status); Type is
// /problems/<name> for 400, 404, 409, 422, 429, and 503, and about:blank for
// every other status.
func WriteProblem(w http.ResponseWriter, status int, title, detail string) {
	if title == "" {
		title = http.StatusText(status)
	}
	w.Header().Set("Content-Type", problemContentType)
	w.WriteHeader(status)
	p := Problem{Type: problemType(status), Title: title, Status: status, Detail: detail}
	if err := json.NewEncoder(w).Encode(p); err != nil {
		slog.Warn("write problem response", "err", err)
	}
}

// ProblemFor writes the problem response for err. Typed errors from
// internal/data map to their status; anything else is 500. The detail is the
// generic status text, never err.Error(), because errors may hold internal
// detail.
func ProblemFor(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, data.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, data.ErrInvalid):
		status = http.StatusBadRequest
	case errors.Is(err, data.ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, data.ErrRateLimited):
		status = http.StatusTooManyRequests
	case errors.Is(err, data.ErrUnavailable):
		status = http.StatusServiceUnavailable
	}
	WriteProblem(w, status, "", http.StatusText(status))
}
