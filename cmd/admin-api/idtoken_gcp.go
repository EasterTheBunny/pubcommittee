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
	reasonMismatch   = "subject mismatch"
)

// Header shape classes, logged with a no-bearer refusal so the operator can
// see what reached the container. Never the header value.
const (
	shapeAbsent = "absent"
	shapeBearer = "bearer"
	shapeOther  = "other scheme"
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
	// full, in cloud-run mode, verifies an Authorization token that still has
	// its signature (Cloud Run passes Authorization on unchanged when the
	// caller also sends X-Serverless-Authorization). May be nil.
	full tokenValidator
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
// Token selection, in order:
//  1. Authorization, if full (when not nil) verifies its signature. If
//     X-Serverless-Authorization is also present, its subject must match.
//  2. X-Serverless-Authorization, decoded without the signature.
//  3. Authorization, decoded without the signature, only when
//     X-Serverless-Authorization is absent.
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
func newCloudRunAuth(
	audience string, clientAudiences []string, full tokenValidator, now func() time.Time, log *slog.Logger,
) idTokenAuth {
	a := newIDTokenAuth(audience, clientAudiences, unverifiedValidator(now))
	a.cloudRun = true
	a.full = full
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

// bearer returns the token of a "Bearer" header (scheme case-insensitive,
// RFC 9110) and the header's shape class.
func bearer(h string) (string, string) {
	if h == "" {
		return "", shapeAbsent
	}
	if len(h) < len(bearerPrefix) || !strings.EqualFold(h[:len(bearerPrefix)], bearerPrefix) {
		return "", shapeOther
	}
	return strings.TrimSpace(h[len(bearerPrefix):]), shapeBearer
}

// refuseNoBearer logs which headers arrived, by shape class only.
func (a idTokenAuth) refuseNoBearer(ctx context.Context, authz, serverless string) error {
	if a.log != nil {
		a.log.WarnContext(ctx, "authentication refused", "reason", reasonNoBearer,
			"authorization", authz, "serverless", serverless)
	}
	return fmt.Errorf("%s: %w", reasonNoBearer, errAuth)
}

// payload picks and validates the token. See newCloudRunAuth for the order in
// cloud-run mode; full mode reads Authorization only.
func (a idTokenAuth) payload(r *http.Request) (*idtoken.Payload, error) {
	ctx := r.Context()
	az, azShape := bearer(r.Header.Get("Authorization"))
	slTok, slShape := "", shapeAbsent
	if a.cloudRun {
		slTok, slShape = bearer(r.Header.Get(serverlessHeader))
		if p, ok := a.fullyVerified(ctx, az); ok {
			return a.matchSubject(ctx, p, slTok, slShape)
		}
	}
	tok, shape := az, azShape
	if slShape != shapeAbsent {
		// Cloud Run checked this header, not Authorization.
		tok, shape = slTok, slShape
	}
	if shape != shapeBearer {
		return nil, a.refuseNoBearer(ctx, azShape, slShape)
	}
	if tok == "" {
		return nil, a.refuse(ctx, reasonEmpty)
	}
	p, err := a.validate(ctx, tok)
	if err != nil || p == nil {
		return nil, a.refuse(ctx, validationReason(err))
	}
	return p, nil
}

// fullyVerified reports whether full (if set) verifies tok's signature.
func (a idTokenAuth) fullyVerified(ctx context.Context, tok string) (*idtoken.Payload, bool) {
	if tok == "" || a.full == nil {
		return nil, false
	}
	p, err := a.full(ctx, tok)
	return p, err == nil && p != nil
}

// matchSubject requires a fully verified Authorization token to name the same
// subject as the X-Serverless-Authorization token Cloud Run checked, if any.
func (a idTokenAuth) matchSubject(
	ctx context.Context, p *idtoken.Payload, slTok, slShape string,
) (*idtoken.Payload, error) {
	if slShape == shapeAbsent {
		return p, nil
	}
	if slShape != shapeBearer || slTok == "" {
		return nil, a.refuse(ctx, reasonMismatch)
	}
	q, err := a.validate(ctx, slTok)
	if err != nil || q == nil {
		return nil, a.refuse(ctx, validationReason(err))
	}
	if q.Subject == "" || q.Subject != p.Subject {
		return nil, a.refuse(ctx, reasonMismatch)
	}
	return p, nil
}

// Authenticate returns the caller as an IAM member: "serviceAccount:<email>"
// for service account emails, otherwise "user:<email>". Errors carry no token
// or email. Each refusal logs its reason class only.
func (a idTokenAuth) Authenticate(r *http.Request) (string, error) {
	ctx := r.Context()
	p, err := a.payload(r)
	if err != nil {
		return "", err
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
