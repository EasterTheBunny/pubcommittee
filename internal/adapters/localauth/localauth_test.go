package localauth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/adapters/localauth"
)

func TestAuthenticate(t *testing.T) {
	t.Parallel()
	t.Run("returns fixed operator", func(t *testing.T) {
		t.Parallel()
		op, err := localauth.New().Authenticate(context.Background())
		if err != nil {
			t.Fatalf("Authenticate: unexpected error: %v", err)
		}
		if op.ID != localauth.OperatorID {
			t.Errorf("Authenticate ID = %q, want %q", op.ID, localauth.OperatorID)
		}
	})
	t.Run("canceled context errors", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := localauth.New().Authenticate(ctx)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Authenticate with canceled ctx: err = %v, want context.Canceled", err)
		}
	})
}

func TestMiddleware(t *testing.T) {
	t.Parallel()
	var seen localauth.Operator
	var seenOK bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, seenOK = localauth.FromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})
	h := localauth.New().Middleware(next)
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if !seenOK {
		t.Fatal("FromContext in next: ok = false, want true")
	}
	if seen.ID != localauth.OperatorID {
		t.Errorf("operator ID in next = %q, want %q", seen.ID, localauth.OperatorID)
	}
}

func TestFromContextBare(t *testing.T) {
	t.Parallel()
	if _, ok := localauth.FromContext(context.Background()); ok {
		t.Error("FromContext on bare context: ok = true, want false")
	}
}
