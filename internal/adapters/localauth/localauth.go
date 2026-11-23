// Package localauth authenticates admin-api requests for local and memory
// targets only. It is never imported by a gcp build or a deployed admin-api.
// It authenticates every request as one fixed fake operator and models no
// credentials, sessions, or IAM.
package localauth

import (
	"context"
	"fmt"
	"net/http"
)

// OperatorID is the id of the one fake operator.
const OperatorID = "local-operator"

// Operator is the authenticated operator of a request.
type Operator struct{ ID string }

// Authenticator authenticates every request as the fixed local operator.
type Authenticator struct{}

// operatorKey is the unexported context key that holds the Operator.
type operatorKey struct{}

// New returns an Authenticator.
func New() Authenticator {
	return Authenticator{}
}

// Authenticate returns the fixed operator; it never fails except when ctx is done.
func (Authenticator) Authenticate(ctx context.Context) (Operator, error) {
	if err := ctx.Err(); err != nil {
		return Operator{}, fmt.Errorf("authenticate: %w", err)
	}
	return Operator{ID: OperatorID}, nil
}

// Middleware stores the operator in the request context and calls next.
func (Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		op := Operator{ID: OperatorID}
		ctx := context.WithValue(r.Context(), operatorKey{}, op)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// FromContext returns the operator stored by Middleware.
func FromContext(ctx context.Context) (Operator, bool) {
	op, ok := ctx.Value(operatorKey{}).(Operator)
	return op, ok
}
