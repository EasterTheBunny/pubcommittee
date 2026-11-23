package adminapi_test

import (
	"net/http"
	"testing"
)

func TestAuthorization(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.publishV1()
	for _, c := range []struct {
		name, principal, method, path string
		want                          int
	}{
		{"no credentials", "", http.MethodGet, "/admin/v1/jobs", http.StatusUnauthorized},
		{"stranger with project access", principalOther, http.MethodGet, "/admin/v1/jobs", http.StatusForbidden},
		{"stranger tracks", principalOther, http.MethodPost, trackPath + "track", http.StatusForbidden},
		{"deploy account on a bill route", principalDeploy, http.MethodPost, trackPath + "track", http.StatusForbidden},
		{"deploy account on jobs", principalDeploy, http.MethodGet, "/admin/v1/jobs", http.StatusForbidden},
		{"deploy account on policy", principalDeploy, http.MethodPost, "/admin/v1/selection-policy", http.StatusForbidden},
		{"operator on the roster route", principalOp1, http.MethodPost, "/admin/v1/operators/roster", http.StatusForbidden},
		{"stranger on the roster route", principalOther, http.MethodPost, "/admin/v1/operators/roster", http.StatusForbidden},
		{"operator reads jobs", principalOp1, http.MethodGet, "/admin/v1/jobs", http.StatusOK},
	} {
		code, body := e.call(c.principal, c.method, c.path, reason("x"))
		if code != c.want {
			t.Errorf("%s: status %d, want %d: %s", c.name, code, c.want, body)
		}
	}
	if len(e.log()) != 1 {
		t.Fatalf("a refused call wrote the log: %d entries", len(e.log()))
	}
}

func TestNoCORSHeaders(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	req, rec := newReq(t, principalOp1, "/admin/v1/jobs")
	req.Header.Set("Origin", "https://example.test")
	e.srv.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("admin-api sent a CORS header")
	}
}
