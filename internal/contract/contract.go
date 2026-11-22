// Package contract validates live HTTP responses against api/openapi.yaml.
// Service tests (M1-10) build a Harness against a running read-api and call
// Get for each route; any response that departs from the spec fails the test.
package contract

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"

	apiv1 "github.com/easterthebunny/pubcommittee/api/v1"
)

// Contract departures reported by checkConventions.
var (
	errWeakETag      = errors.New("200 without a strong ETag")
	errNoRetryAfter  = errors.New("429 without Retry-After")
	errProblemFormat = errors.New("error response is not application/problem+json")
)

// Harness checks responses of one running service against the embedded spec.
type Harness struct {
	// BaseURL is the service root, for example http://127.0.0.1:8080.
	BaseURL string
	// HTTP is the client used for requests; nil means http.DefaultClient.
	HTTP *http.Client

	router routers.Router
}

// Spec returns the embedded OpenAPI document with its server list replaced by
// root, so the router matches paths regardless of the documented servers.
func Spec(root string) (*openapi3.T, error) {
	doc, err := apiv1.GetSpec()
	if err != nil {
		return nil, fmt.Errorf("load embedded spec: %w", err)
	}
	doc.Servers = openapi3.Servers{{URL: root}}
	return doc, nil
}

// New builds a Harness for the service at baseURL.
func New(baseURL string) (*Harness, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	doc, err := Spec(baseURL)
	if err != nil {
		return nil, err
	}
	router, err := legacy.NewRouter(doc)
	if err != nil {
		return nil, fmt.Errorf("build router: %w", err)
	}
	return &Harness{BaseURL: baseURL, router: router}, nil
}

// Result is a validated response.
type Result struct {
	Status int
	Header http.Header
	Body   []byte
}

// Get requests path (with optional query and extra request headers), then
// validates the status, headers, and body against the spec. The returned
// error names the first departure from the contract.
func (h *Harness) Get(ctx context.Context, path string, query url.Values, hdr http.Header) (Result, error) {
	target := h.BaseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return Result{}, fmt.Errorf("build request: %w", err)
	}
	maps.Copy(req.Header, hdr)
	client := h.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("get %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{}, fmt.Errorf("read %s: %w", path, err)
	}
	res := Result{Status: resp.StatusCode, Header: resp.Header, Body: body}
	return res, h.validate(ctx, req, resp, body)
}

func (h *Harness) validate(ctx context.Context, req *http.Request, resp *http.Response, body []byte) error {
	route, params, err := h.router.FindRoute(req)
	if err != nil {
		return fmt.Errorf("%s is not a route in the spec: %w", req.URL.Path, err)
	}
	in := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request:    req,
			PathParams: params,
			Route:      route,
			Options:    &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
		},
		Status:  resp.StatusCode,
		Header:  resp.Header,
		Options: &openapi3filter.Options{IncludeResponseStatus: true},
	}
	in.SetBodyBytes(body)
	if err := openapi3filter.ValidateResponse(ctx, in); err != nil {
		return fmt.Errorf("%s %s -> %d: %w", req.Method, req.URL.Path, resp.StatusCode, err)
	}
	return checkConventions(resp, body)
}

// checkConventions enforces rules the schema language cannot: a strong ETag
// on every 200, Retry-After on every 429, problem media type on errors.
func checkConventions(resp *http.Response, body []byte) error {
	if resp.StatusCode == http.StatusOK {
		if etag := resp.Header.Get("ETag"); etag == "" || strings.HasPrefix(etag, "W/") {
			return fmt.Errorf("%w (got %q)", errWeakETag, etag)
		}
	}
	if resp.StatusCode == http.StatusTooManyRequests && resp.Header.Get("Retry-After") == "" {
		return errNoRetryAfter
	}
	if resp.StatusCode >= http.StatusBadRequest && len(body) > 0 {
		if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
			return fmt.Errorf("%w: status %d, content type %q", errProblemFormat, resp.StatusCode, ct)
		}
	}
	return nil
}

// MustGet is Get that fails the test on any contract departure.
func (h *Harness) MustGet(t *testing.T, path string, query url.Values) Result {
	t.Helper()
	res, err := h.Get(t.Context(), path, query, nil)
	if err != nil {
		t.Fatalf("contract: %v", err)
	}
	return res
}
