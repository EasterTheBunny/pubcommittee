//go:build gcp

package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/api/idtoken"
)

const (
	testAudience = "https://admin.example.invalid"
	testClientID = "1234.apps.example.invalid"
)

var errFake = errors.New("fake invalid token")

func fakeValidator(p *idtoken.Payload, err error) tokenValidator {
	return func(context.Context, string) (*idtoken.Payload, error) { return p, err }
}

func payload(aud, email string, verified any) *idtoken.Payload {
	claims := map[string]any{}
	if email != "" {
		claims["email"] = email
	}
	if verified != nil {
		claims["email_verified"] = verified
	}
	return &idtoken.Payload{Audience: aud, Claims: claims}
}

func TestIDTokenAuth(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		header string
		pl     *idtoken.Payload
		client []string
		err    error
		want   string
		fail   bool
	}{
		{name: "user", header: "Bearer t", pl: payload(testAudience, "a@example.invalid", true), want: "user:a@example.invalid"},
		{name: "service account", header: "Bearer t", pl: payload(testAudience, "x@p.iam.gserviceaccount.com", true), want: "serviceAccount:x@p.iam.gserviceaccount.com"},
		{name: "client audience ok", header: "Bearer t", pl: payload(testClientID, "a@example.invalid", true), client: []string{"other", testClientID}, want: "user:a@example.invalid"},
		{name: "url audience ok with client list", header: "Bearer t", pl: payload(testAudience, "a@example.invalid", true), client: []string{testClientID}, want: "user:a@example.invalid"},
		{name: "unlisted audience", header: "Bearer t", pl: payload("999.apps.example.invalid", "a@example.invalid", true), client: []string{testClientID}, fail: true},
		{name: "client audience with empty list", header: "Bearer t", pl: payload(testClientID, "a@example.invalid", true), client: []string{"", " "}, fail: true},
		{name: "empty audience claim", header: "Bearer t", pl: payload("", "a@example.invalid", true), client: []string{""}, fail: true},
		{name: "no header", pl: payload(testAudience, "a@example.invalid", true), fail: true},
		{name: "not bearer", header: "Basic t", pl: payload(testAudience, "a@example.invalid", true), fail: true},
		{name: "empty bearer", header: "Bearer  ", pl: payload(testAudience, "a@example.invalid", true), fail: true},
		{name: "invalid token", header: "Bearer t", err: errFake, fail: true},
		{name: "wrong audience", header: "Bearer t", pl: payload("https://other.invalid", "a@example.invalid", true), fail: true},
		{name: "unverified", header: "Bearer t", pl: payload(testAudience, "a@example.invalid", false), fail: true},
		{name: "verified missing", header: "Bearer t", pl: payload(testAudience, "a@example.invalid", nil), fail: true},
		{name: "no email", header: "Bearer t", pl: payload(testAudience, "", true), fail: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := newIDTokenAuth(testAudience, tc.client, fakeValidator(tc.pl, tc.err))
			r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/v1/jobs", http.NoBody)
			if tc.header != "" {
				r.Header.Set("Authorization", tc.header)
			}
			got, err := a.Authenticate(r)
			if tc.fail {
				if !errors.Is(err, errAuth) {
					t.Fatalf("want errAuth, got %v", err)
				}
				if strings.Contains(err.Error(), "example.invalid") {
					t.Fatalf("error leaks identity: %v", err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestGCPConfig(t *testing.T) {
	t.Parallel()
	const ops = `{"operators":[{"id":"op-1","name":"One","principal":"user:a@example.invalid"}]}`
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	good := map[string]string{envOperators: ops, envDeployPrincipal: "serviceAccount:d@p.iam.gserviceaccount.com"}
	cfg, err := gcpConfig(env(good))
	if err != nil || cfg.DeployPrincipal != good[envDeployPrincipal] || len(cfg.Operators) != 1 {
		t.Fatalf("good config: %+v, %v", cfg, err)
	}
	bad := []map[string]string{
		{envDeployPrincipal: good[envDeployPrincipal]},
		{envOperators: ops},
		{envOperators: "{", envDeployPrincipal: "d"},
		{envOperators: ops, envDeployPrincipal: "user:a@example.invalid"},
		{envOperators: `{"operators":[],"deploy_principal":"x"}`, envDeployPrincipal: "y"},
	}
	for i, m := range bad {
		if _, err := gcpConfig(env(m)); err == nil {
			t.Errorf("case %d: want error", i)
		}
	}
}
