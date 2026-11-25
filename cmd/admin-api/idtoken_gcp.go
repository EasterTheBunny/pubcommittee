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

// tokenValidator checks a Google ID token signature and expiry. It does not
// check the audience: idtoken accepts only one, so idTokenAuth checks the set.
type tokenValidator func(ctx context.Context, token string) (*idtoken.Payload, error)

// googleValidator is the real validator: Google's public keys, fetched and
// cached by the idtoken package. An empty audience makes idtoken skip only
// its audience comparison; signature, issuer, and expiry are still checked.
func googleValidator(ctx context.Context, token string) (*idtoken.Payload, error) {
	p, err := idtoken.Validate(ctx, token, "")
	if err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}
	return p, nil
}

// idTokenAuth authenticates a request by the Google ID token in its
// Authorization: Bearer header. Cloud Run IAM checks the token in front; this
// is defense in depth and the source of the caller's identity.
//
// Accepted audiences: the service URL (service account callers, such as the
// deploy account) plus optional OAuth client ids (a user's gcloud ID token
// carries gcloud's client id as aud, never the service URL).
type idTokenAuth struct {
	audiences map[string]struct{}
	validate  tokenValidator
}

// newIDTokenAuth accepts tokens whose aud is audience or one of clientAudiences.
// Empty entries are ignored, so an empty clientAudiences list accepts only audience.
func newIDTokenAuth(audience string, clientAudiences []string, v tokenValidator) idTokenAuth {
	set := map[string]struct{}{audience: {}}
	for _, a := range clientAudiences {
		if a = strings.TrimSpace(a); a != "" {
			set[a] = struct{}{}
		}
	}
	return idTokenAuth{audiences: set, validate: v}
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
	p, err := a.validate(r.Context(), tok)
	if err != nil || p == nil {
		return "", fmt.Errorf("token rejected: %w", errAuth)
	}
	if _, ok := a.audiences[p.Audience]; !ok {
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
