//go:build gcp

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"google.golang.org/api/idtoken"
)

const (
	bearerPrefix   = "Bearer "
	serviceAccount = ".gserviceaccount.com"
	// serverlessHeader is checked by Cloud Run in precedence over Authorization.
	serverlessHeader = "X-Serverless-Authorization"
	// clockSkew is the leeway on exp in the cloud-run mode.
	clockSkew = time.Minute
)

var errAuth = errors.New("authentication failed")

// Refusal reasons. Only these classes are logged, never a token, email,
// principal, or audience value.
const (
	reasonNoBearer   = "no bearer"
	reasonEmpty      = "empty bearer"
	reasonRejected   = "token rejected"
	reasonMalformed  = "malformed token"
	reasonIssuer     = "wrong issuer"
	reasonExpired    = "expired"
	reasonAudience   = "wrong audience"
	reasonNoEmail    = "no email"
	reasonUnverified = "unverified email"
)

var (
	errMalformed = errors.New("malformed token")
	errIssuer    = errors.New("wrong issuer")
	errExpired   = errors.New("expired")
)

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
//
// Two modes. Full (default): idtoken verifies the signature with Google's keys
// and the token is read from Authorization. Cloud Run
// (ADMIN_API_TOKEN_VERIFIED_BY=cloud-run): see newCloudRunAuth.
type idTokenAuth struct {
	audiences map[string]struct{}
	validate  tokenValidator
	// cloudRun reads X-Serverless-Authorization first.
	cloudRun bool
	// log receives one warning per refusal, with the reason class only. May be nil.
	log *slog.Logger
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

// newCloudRunAuth trusts Cloud Run's signature check.
//
// TRUST BOUNDARY: Cloud Run verifies the Google ID token (the IAM invoker
// check) and then removes the signature before the request reaches the
// container, so idtoken.Validate can never succeed there. This mode is safe
// only because the service allows no unauthenticated invokers: every request
// that reaches the container passed Cloud Run's IAM check of exactly the
// header read here. Cloud Run checks X-Serverless-Authorization in precedence
// over Authorization; when that header is present, Authorization reaches the
// container unchanged and unverified, so it is never read. Never enable this
// mode on a host that does not do this check. The token payload is decoded
// without verifying the signature; issuer, expiry (with a small skew),
// audience, and email claims are still checked.
func newCloudRunAuth(audience string, clientAudiences []string, now func() time.Time, log *slog.Logger) idTokenAuth {
	a := newIDTokenAuth(audience, clientAudiences, unverifiedValidator(now))
	a.cloudRun = true
	a.log = log
	return a
}

// unverifiedValidator decodes the JWT payload and checks issuer and expiry. It
// never checks the signature.
func unverifiedValidator(now func() time.Time) tokenValidator {
	return func(_ context.Context, token string) (*idtoken.Payload, error) {
		parts := strings.Split(token, ".")
		if len(parts) != 3 || parts[1] == "" {
			return nil, errMalformed
		}
		raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
		if err != nil {
			return nil, errMalformed
		}
		var c struct {
			Iss string `json:"iss"`
			Aud string `json:"aud"`
			Sub string `json:"sub"`
			Exp *int64 `json:"exp"`
			Iat int64  `json:"iat"`
		}
		claims := map[string]any{}
		if err := json.Unmarshal(raw, &c); err != nil || c.Exp == nil {
			return nil, errMalformed
		}
		if err := json.Unmarshal(raw, &claims); err != nil {
			return nil, errMalformed
		}
		if c.Iss != "accounts.google.com" && c.Iss != "https://accounts.google.com" {
			return nil, errIssuer
		}
		if !time.Unix(*c.Exp, 0).Add(clockSkew).After(now()) {
			return nil, errExpired
		}
		return &idtoken.Payload{
			Issuer: c.Iss, Audience: c.Aud, Subject: c.Sub, Expires: *c.Exp, IssuedAt: c.Iat, Claims: claims,
		}, nil
	}
}

// validationReason maps a validator error to its reason class.
func validationReason(err error) string {
	switch {
	case errors.Is(err, errMalformed):
		return reasonMalformed
	case errors.Is(err, errIssuer):
		return reasonIssuer
	case errors.Is(err, errExpired):
		return reasonExpired
	}
	return reasonRejected
}

// refuse logs the reason class at warning level and returns the error.
func (a idTokenAuth) refuse(ctx context.Context, reason string) error {
	if a.log != nil {
		a.log.WarnContext(ctx, "authentication refused", "reason", reason)
	}
	return fmt.Errorf("%s: %w", reason, errAuth)
}

// header returns the request header the token is read from. In cloud-run mode
// X-Serverless-Authorization wins and Authorization is then never read.
func (a idTokenAuth) header(r *http.Request) string {
	if a.cloudRun {
		if h := r.Header.Get(serverlessHeader); h != "" {
			return h
		}
	}
	return r.Header.Get("Authorization")
}

// Authenticate returns the caller as an IAM member: "serviceAccount:<email>"
// for service account emails, otherwise "user:<email>". Errors carry no token
// or email. Each refusal logs its reason class only.
func (a idTokenAuth) Authenticate(r *http.Request) (string, error) {
	ctx := r.Context()
	h := a.header(r)
	if !strings.HasPrefix(h, bearerPrefix) {
		return "", a.refuse(ctx, reasonNoBearer)
	}
	tok := strings.TrimSpace(strings.TrimPrefix(h, bearerPrefix))
	if tok == "" {
		return "", a.refuse(ctx, reasonEmpty)
	}
	p, err := a.validate(ctx, tok)
	if err != nil || p == nil {
		return "", a.refuse(ctx, validationReason(err))
	}
	if _, ok := a.audiences[p.Audience]; !ok {
		return "", a.refuse(ctx, reasonAudience)
	}
	email, _ := p.Claims["email"].(string)
	if email == "" {
		return "", a.refuse(ctx, reasonNoEmail)
	}
	if verified, _ := p.Claims["email_verified"].(bool); !verified {
		return "", a.refuse(ctx, reasonUnverified)
	}
	if strings.HasSuffix(email, serviceAccount) {
		return "serviceAccount:" + email, nil
	}
	return "user:" + email, nil
}
