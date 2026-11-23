package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

const (
	envToken     = "PUBCTL_TOKEN" //nolint:gosec // the name of an environment variable, not a credential
	requestLimit = 2 * time.Minute
	maxResponse  = 4 << 20
)

var (
	errBase   = errors.New("--url must be an http(s) URL of admin-api")
	errNoBody = errors.New("empty response")
)

// apiError is an error response of admin-api (application/problem+json).
type apiError struct {
	Status int
	Title  string
	Detail string
}

func (e *apiError) Error() string {
	if e.Detail != "" && e.Detail != e.Title {
		return fmt.Sprintf("admin-api: %d %s: %s", e.Status, e.Title, e.Detail)
	}
	return fmt.Sprintf("admin-api: %d %s", e.Status, e.Title)
}

// client calls admin-api with the operator's identity token. Locally (a
// loopback URL) it sends none: localauth authenticates every request.
type client struct {
	base  *url.URL
	hc    *http.Client
	token func(ctx context.Context) (string, error)
}

// newClient builds a client for the admin-api at base. The token comes from
// PUBCTL_TOKEN when set, else from `gcloud auth print-identity-token` for a
// non-loopback URL; a loopback URL needs none.
func newClient(base string, getenv func(string) string, hc *http.Client) (*client, error) {
	u, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errBase
	}
	if hc == nil {
		hc = &http.Client{Timeout: requestLimit}
	}
	c := &client{base: u, hc: hc}
	switch {
	case getenv(envToken) != "":
		tok := getenv(envToken)
		c.token = func(context.Context) (string, error) { return tok, nil }
	case isLoopback(u.Hostname()):
		c.token = func(context.Context) (string, error) { return "", nil }
	default:
		audience := u.Scheme + "://" + u.Host
		c.token = func(ctx context.Context) (string, error) { return gcloudToken(ctx, audience) }
	}
	return c, nil
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// gcloudToken asks the gcloud CLI for an identity token for the audience.
func gcloudToken(ctx context.Context, audience string) (string, error) {
	//nolint:gosec // fixed program and flags; the audience is the operator's own --url
	cmd := exec.CommandContext(ctx, "gcloud", "auth", "print-identity-token", "--audiences="+audience)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gcloud auth print-identity-token: %w (or set %s)", err, envToken)
	}
	return strings.TrimSpace(string(out)), nil
}

// newRequest builds one JSON request with the bearer token, if any.
func (c *client) newRequest(ctx context.Context, method, path string, in any) (*http.Request, error) {
	var body io.Reader = http.NoBody
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return nil, fmt.Errorf("encode request: %w", err)
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base.String()+path, body)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	tok, err := c.token(ctx)
	if err != nil {
		return nil, err
	}
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	return req, nil
}

// do sends one JSON request. in and out may be nil. A non-2xx answer is an *apiError.
func (c *client) do(ctx context.Context, method, path string, in, out any) error {
	req, err := c.newRequest(ctx, method, path, in)
	if err != nil {
		return err
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("call admin-api: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return problemOf(resp.StatusCode, raw)
	}
	if out == nil {
		return nil
	}
	if len(raw) == 0 {
		return errNoBody
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// problemOf decodes an application/problem+json body into an *apiError.
func problemOf(status int, raw []byte) error {
	var p struct {
		Title  string `json:"title"`
		Detail string `json:"detail"`
	}
	_ = json.Unmarshal(raw, &p)
	if p.Title == "" {
		p.Title = http.StatusText(status)
	}
	return &apiError{Status: status, Title: p.Title, Detail: p.Detail}
}
