//go:build gcp

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/api/idtoken"
)

const (
	bearerPrefix   = "Bearer "
	serviceAccount = ".gserviceaccount.com"
)

var errAuth = errors.New("authentication failed")

// tokenValidator checks a Google ID token signature, expiry, and audience.
type tokenValidator func(ctx context.Context, token, audience string) (*idtoken.Payload, error)

// googleValidator is the real validator: Google's public keys, fetched and
// cached by the idtoken package.
func googleValidator(ctx context.Context, token, audience string) (*idtoken.Payload, error) {
	p, err := idtoken.Validate(ctx, token, audience)
	if err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}
	return p, nil
}

// idTokenAuth authenticates a request by the Google ID token in its
// Authorization: Bearer header. Cloud Run IAM checks the token in front; this
// is defense in depth and the source of the caller's identity.
type idTokenAuth struct {
	audience string
	validate tokenValidator
}

func newIDTokenAuth(audience string, v tokenValidator) idTokenAuth {
	return idTokenAuth{audience: audience, validate: v}
}

// Authenticate returns the caller as an IAM member: "serviceAccount:<email>"
// for service account emails, otherwise "user:<email>". Errors carry no token
// or email.
func (a idTokenAuth) Authenticate(r *http.Request) (string, error) {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, bearerPrefix) {
		return "", fmt.Errorf("no bearer token: %w", errAuth)
	}
	tok := strings.TrimSpace(strings.TrimPrefix(h, bearerPrefix))
	if tok == "" {
		return "", fmt.Errorf("empty bearer token: %w", errAuth)
	}
	p, err := a.validate(r.Context(), tok, a.audience)
	if err != nil || p == nil {
		return "", fmt.Errorf("token rejected: %w", errAuth)
	}
	if p.Audience != a.audience {
		return "", fmt.Errorf("wrong audience: %w", errAuth)
	}
	email, _ := p.Claims["email"].(string)
	if email == "" {
		return "", fmt.Errorf("no email claim: %w", errAuth)
	}
	if verified, _ := p.Claims["email_verified"].(bool); !verified {
		return "", fmt.Errorf("email not verified: %w", errAuth)
	}
	if strings.HasSuffix(email, serviceAccount) {
		return "serviceAccount:" + email, nil
	}
	return "user:" + email, nil
}
