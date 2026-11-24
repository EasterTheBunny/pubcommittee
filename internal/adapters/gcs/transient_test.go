package gcs_test

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/adapters/gcs"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

var errReset = errors.New("connection reset by peer")

// faultTransport stands in for the network. The first fail requests get err or
// a 503 answer (when err is nil); later requests get ok. It needs no
// stand-in server, so these tests run everywhere: they replace the testbench
// fault test of M1-00 (operator decision: transient errors through an
// injected http.RoundTripper).
type faultTransport struct {
	fail  int32
	err   error
	ok    func(*http.Request) *http.Response
	calls atomic.Int32
}

func (f *faultTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	n := f.calls.Add(1)
	if n > f.fail {
		return f.ok(r), nil
	}
	if f.err != nil {
		return nil, f.err
	}
	return reply(r, http.StatusServiceUnavailable, `{"error":{"code":503,"message":"backend unavailable"}}`), nil
}

func reply(r *http.Request, code int, body string) *http.Response {
	h := http.Header{"Content-Type": {"application/json"}}
	return &http.Response{
		StatusCode: code, Status: http.StatusText(code), Header: h, Request: r,
		Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body)),
	}
}

func media(r *http.Request) *http.Response {
	resp := reply(r, http.StatusOK, "data")
	resp.Header.Set("Content-Type", "text/plain")
	return resp
}

func faultStore(t *testing.T, tr *faultTransport) *gcs.Store {
	t.Helper()
	s, err := gcs.New(t.Context(), gcs.Config{
		Bucket: "pc-fault-bucket", Transport: tr, MaxAttempts: 3, RetryBase: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestTransientErrorsAreRetried(t *testing.T) {
	t.Parallel()
	for name, tr := range map[string]*faultTransport{
		"503 twice": {fail: 2, ok: media},
		"reset":     {fail: 2, err: errReset, ok: media},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := faultStore(t, tr)
			got, err := s.GetFile(t.Context(), "exports/a.txt")
			if err != nil || string(got) != "data" {
				t.Fatalf("GetFile = %q, %v; want data", got, err)
			}
			if c := tr.calls.Load(); c != 3 {
				t.Errorf("calls = %d, want 3", c)
			}
		})
	}
}

func TestPersistentFailureIsUnavailable(t *testing.T) {
	t.Parallel()
	for name, tr := range map[string]*faultTransport{
		"503":   {fail: 1 << 20, ok: media},
		"reset": {fail: 1 << 20, err: errReset, ok: media},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := faultStore(t, tr)
			_, err := s.GetFile(t.Context(), "exports/a.txt")
			if !errors.Is(err, data.ErrUnavailable) {
				t.Fatalf("err = %v, want ErrUnavailable", err)
			}
			if c := tr.calls.Load(); c != 3 {
				t.Errorf("calls = %d, want the 3 attempts of MaxAttempts", c)
			}
		})
	}
}

func TestStatusMapping(t *testing.T) {
	t.Parallel()
	for code, want := range map[int]error{
		http.StatusNotFound:        data.ErrNotFound,
		http.StatusTooManyRequests: data.ErrRateLimited,
		http.StatusBadRequest:      data.ErrInvalid,
		http.StatusForbidden:       data.ErrUnavailable,
		http.StatusUnauthorized:    data.ErrUnavailable,
	} {
		tr := &faultTransport{ok: func(r *http.Request) *http.Response {
			return reply(r, code, `{"error":{"code":0,"message":"x"}}`)
		}}
		s := faultStore(t, tr)
		_, err := s.Stat(t.Context(), "exports/a.txt")
		if !errors.Is(err, want) {
			t.Errorf("status %d: err = %v, want %v", code, err, want)
		}
	}
}
