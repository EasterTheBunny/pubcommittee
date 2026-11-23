package data

import "errors"

// Typed errors. Adapters wrap vendor failures in these; services check them
// with errors.Is and never inspect vendor error codes.
var (
	// ErrNotFound means the requested record does not exist.
	ErrNotFound = errors.New("not found")
	// ErrConflict means the write conflicts with stored state, for example
	// an overwrite of a write-once path with different content.
	ErrConflict = errors.New("conflict")
	// ErrUnavailable means the backend cannot be reached; retry may succeed.
	ErrUnavailable = errors.New("unavailable")
	// ErrRateLimited means the caller exceeded a limit.
	ErrRateLimited = errors.New("rate limited")
	// ErrInvalid means the input is malformed or fails validation.
	ErrInvalid = errors.New("invalid")
)

// Outcome returns the metric outcome name for err: "ok" for nil, the typed
// error name when err matches one, and "error" otherwise.
func Outcome(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, ErrNotFound):
		return "not_found"
	case errors.Is(err, ErrConflict):
		return "conflict"
	case errors.Is(err, ErrUnavailable):
		return "unavailable"
	case errors.Is(err, ErrRateLimited):
		return "rate_limited"
	case errors.Is(err, ErrInvalid):
		return "invalid"
	default:
		return "error"
	}
}
