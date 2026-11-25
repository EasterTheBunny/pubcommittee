//go:build gcp

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/api/idtoken"
)

const testNowUnix = 1_800_000_000

// fakeJWT builds an unsigned token: header and payload are base64url JSON and
// the signature is empty, as Cloud Run leaves it.
func fakeJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	enc := func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return base64.RawURLEncoding.EncodeToString(b)
	}
	return enc(map[string]any{"alg": "RS256"}) + "." + enc(claims) + "."
}

func goodClaims() map[string]any {
	return map[string]any{
		"iss": "https://accounts.google.com", "aud": testAudience,
		"email": "a@example.invalid", "email_verified": true, "sub": "1",
		"exp": time.Unix(testNowUnix, 0).Add(time.Hour).Unix(),
	}
}

var errBadSignature = errors.New("bad signature")

// signed marks a token as carrying a valid signature for fakeFull.
func signed(tok string) string { return tok + "sig-ok" }

// fakeFull stands in for googleValidator: it accepts only signed tokens.
func fakeFull(ctx context.Context, tok string) (*idtoken.Payload, error) {
	if !strings.HasSuffix(tok, ".sig-ok") {
		return nil, errBadSignature
	}
	return unverifiedValidator(func() time.Time { return time.Unix(testNowUnix, 0) })(ctx, tok)
}

func with(c map[string]any, k string, v any) map[string]any {
	if v == nil {
		delete(c, k)
	} else {
		c[k] = v
	}
	return c
}

type cloudRunCase struct {
	name       string
	authz      string
	serverless string
	client     []string
	want       string
	reason     string    // empty: success
	shapes     [2]string // no-bearer refusals: logged authorization and serverless shapes
}

func cloudRunCases(t *testing.T) []cloudRunCase {
	t.Helper()
	good := fakeJWT(t, goodClaims())
	return []cloudRunCase{
		{name: "authorization only", authz: "Bearer " + good, want: "user:a@example.invalid"},
		{name: "serverless only", serverless: "Bearer " + good, want: "user:a@example.invalid"},
		{name: "serverless wins over bad authorization", serverless: "Bearer " + good, authz: "Bearer junk", want: "user:a@example.invalid"},
		{name: "authorization ignored when serverless present", serverless: "Bearer junk", authz: "Bearer " + good, reason: reasonMalformed},
		{name: "service account", authz: "Bearer " + fakeJWT(t, with(goodClaims(), "email", "x@p.iam.gserviceaccount.com")), want: "serviceAccount:x@p.iam.gserviceaccount.com"},
		{name: "bare issuer", authz: "Bearer " + fakeJWT(t, with(goodClaims(), "iss", "accounts.google.com")), want: "user:a@example.invalid"},
		{name: "client audience", authz: "Bearer " + fakeJWT(t, with(goodClaims(), "aud", testClientID)), client: []string{testClientID}, want: "user:a@example.invalid"},
		{name: "within skew", authz: "Bearer " + fakeJWT(t, with(goodClaims(), "exp", time.Unix(testNowUnix, 0).Add(-30*time.Second).Unix())), want: "user:a@example.invalid"},
		{name: "expired", authz: "Bearer " + fakeJWT(t, with(goodClaims(), "exp", time.Unix(testNowUnix, 0).Add(-2*time.Minute).Unix())), reason: reasonExpired},
		{name: "no exp", authz: "Bearer " + fakeJWT(t, with(goodClaims(), "exp", nil)), reason: reasonMalformed},
		{name: "wrong issuer", authz: "Bearer " + fakeJWT(t, with(goodClaims(), "iss", "https://evil.invalid")), reason: reasonIssuer},
		{name: "no issuer", authz: "Bearer " + fakeJWT(t, with(goodClaims(), "iss", nil)), reason: reasonIssuer},
		{name: "wrong audience", authz: "Bearer " + fakeJWT(t, with(goodClaims(), "aud", "https://other.invalid")), reason: reasonAudience},
		{name: "unlisted client audience", authz: "Bearer " + fakeJWT(t, with(goodClaims(), "aud", testClientID)), reason: reasonAudience},
		{name: "no email", authz: "Bearer " + fakeJWT(t, with(goodClaims(), "email", nil)), reason: reasonNoEmail},
		{name: "unverified", authz: "Bearer " + fakeJWT(t, with(goodClaims(), "email_verified", false)), reason: reasonUnverified},
		{name: "verified missing", authz: "Bearer " + fakeJWT(t, with(goodClaims(), "email_verified", nil)), reason: reasonUnverified},
		{name: "lowercase scheme", authz: "bearer " + good, want: "user:a@example.invalid"},
		{name: "signed authorization alone", authz: "Bearer " + signed(good), want: "user:a@example.invalid"},
		{name: "signed authorization and serverless", authz: "Bearer " + signed(good), serverless: "Bearer " + good, want: "user:a@example.invalid"},
		{name: "signed authorization, other subject in serverless", authz: "Bearer " + signed(good), serverless: "Bearer " + fakeJWT(t, with(goodClaims(), "sub", "2")), reason: reasonMismatch},
		{name: "signed authorization, serverless not bearer", authz: "Bearer " + signed(good), serverless: "Basic x", reason: reasonMismatch},
		{name: "serverless not bearer", serverless: "Basic x", authz: "Bearer " + good, reason: reasonNoBearer, shapes: [2]string{shapeBearer, shapeOther}},
		{name: "no header", reason: reasonNoBearer, shapes: [2]string{shapeAbsent, shapeAbsent}},
		{name: "not bearer", authz: "Basic " + good, reason: reasonNoBearer, shapes: [2]string{shapeOther, shapeAbsent}},
		{name: "scheme only", authz: "Bearer", reason: reasonNoBearer, shapes: [2]string{shapeOther, shapeAbsent}},
		{name: "empty bearer", authz: "Bearer  ", reason: reasonEmpty},
		{name: "two segments", authz: "Bearer a.b", reason: reasonMalformed},
		{name: "bad base64", authz: "Bearer a.!!!.c", reason: reasonMalformed},
		{name: "payload not json", authz: "Bearer a." + base64.RawURLEncoding.EncodeToString([]byte("nope")) + ".c", reason: reasonMalformed},
		{name: "audience array", authz: "Bearer " + fakeJWT(t, with(goodClaims(), "aud", []string{testAudience})), reason: reasonMalformed},
	}
}

func TestCloudRunAuth(t *testing.T) {
	t.Parallel()
	for _, tc := range cloudRunCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			log := slog.New(slog.NewJSONHandler(&buf, nil))
			a := newCloudRunAuth(testAudience, tc.client, fakeFull, func() time.Time { return time.Unix(testNowUnix, 0) }, log)
			r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/v1/jobs", http.NoBody)
			if tc.authz != "" {
				r.Header.Set("Authorization", tc.authz)
			}
			if tc.serverless != "" {
				r.Header.Set(serverlessHeader, tc.serverless)
			}
			got, err := a.Authenticate(r)
			if tc.reason == "" {
				if err != nil || got != tc.want {
					t.Fatalf("got %q, %v; want %q", got, err, tc.want)
				}
				if buf.Len() != 0 {
					t.Fatalf("success logged: %s", buf.String())
				}
				return
			}
			if !errors.Is(err, errAuth) || got != "" {
				t.Fatalf("want errAuth, got %q, %v", got, err)
			}
			checkRefusal(t, tc, buf.String(), err)
		})
	}
}

// checkRefusal checks the error and the one warning log line: reason class
// only, no token, email, principal, or audience value.
func checkRefusal(t *testing.T, tc cloudRunCase, logged string, err error) {
	t.Helper()
	if !strings.Contains(err.Error(), tc.reason) {
		t.Fatalf("error %q lacks reason %q", err, tc.reason)
	}
	var rec map[string]any
	if e := json.Unmarshal([]byte(logged), &rec); e != nil {
		t.Fatalf("log: %v: %s", e, logged)
	}
	checkRecord(t, tc, rec)
	for _, secret := range []string{"example.invalid", "evil.invalid", "Bearer", tc.authz, "iam.gserviceaccount"} {
		if secret != "" && (strings.Contains(logged, secret) || strings.Contains(err.Error(), secret)) {
			t.Fatalf("leak of %q: %s / %v", secret, logged, err)
		}
	}
}

// checkRecord checks the log record's fields: a no-bearer refusal also carries
// the shape class of each header.
func checkRecord(t *testing.T, tc cloudRunCase, rec map[string]any) {
	t.Helper()
	fields := 4
	if tc.reason == reasonNoBearer {
		fields = 6
		if rec["authorization"] != tc.shapes[0] || rec["serverless"] != tc.shapes[1] {
			t.Fatalf("shapes %v, want %v", rec, tc.shapes)
		}
	}
	if rec["level"] != "WARN" || rec["reason"] != tc.reason || len(rec) != fields {
		t.Fatalf("log record %v", rec)
	}
}

func TestGCPAuthMode(t *testing.T) {
	t.Parallel()
	env := func(v string) func(string) string {
		return func(k string) string {
			if k == envVerifiedBy {
				return v
			}
			return ""
		}
	}
	for v, wantCloud := range map[string]bool{"": false, " ": false, "cloud-run": true} {
		a, err := gcpAuth(testAudience, env(v))
		if err != nil || a.cloudRun != wantCloud {
			t.Errorf("%q: cloudRun=%v err=%v", v, a.cloudRun, err)
		}
	}
	for _, v := range []string{"Cloud-Run", "true", "gcp"} {
		if _, err := gcpAuth(testAudience, env(v)); !errors.Is(err, errEnv) {
			t.Errorf("%q: want errEnv, got %v", v, err)
		}
	}
}
