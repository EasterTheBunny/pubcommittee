// Package govinfo implements data.BillFetcher on the public govinfo bulk
// data service (no key needed).
//
//	status  {base}/bulkdata/BILLSTATUS/{congress}/{type}/BILLSTATUS-{congress}{type}{number}.xml
//	text    the XML URL each status lists, for example
//	        {base}/content/pkg/BILLS-{congress}{type}{number}{code}/xml/BILLS-{congress}{type}{number}{code}.xml
//
// Politeness: one request at a time (a mutex), at least MinInterval between
// requests, backoff with Retry-After on 429 and 5xx, conditional requests
// (If-None-Match, If-Modified-Since) for a URL fetched before in this process,
// and a descriptive user agent naming the project with no personal details.
// Vendor errors and statuses never cross the port: 404 is data.ErrNotFound,
// 429 data.ErrRateLimited, everything else that fails data.ErrUnavailable.
package govinfo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// Defaults.
const (
	// OfficialHost is the host the statuses name for text versions.
	OfficialHost = "www.govinfo.gov"

	defaultBase        = "https://www.govinfo.gov"
	defaultUserAgent   = "pubcommittee-ingest/0 (open civic project; manual bill pull)"
	defaultMinInterval = 2 * time.Second
	defaultBackoff     = 2 * time.Second
	defaultRetries     = 3
	defaultTimeout     = 60 * time.Second
	maxBackoff         = 60 * time.Second
	maxBody            = 32 << 20
)

// Config configures a Source.
type Config struct {
	// BaseURL is the service root; https://www.govinfo.gov when empty.
	// Tests point it at a local server.
	BaseURL string
	// HTTPClient defaults to a client with a 60 second timeout.
	HTTPClient *http.Client
	// UserAgent defaults to one naming the project.
	UserAgent string
	// MinInterval is the least time between two requests; 2 s when zero.
	// Use a negative value to disable (tests only).
	MinInterval time.Duration
	// Backoff is the first retry delay, doubled each retry; 2 s when zero.
	Backoff time.Duration
	// MaxRetries is the number of retries after the first attempt; 3 when zero.
	MaxRetries int
	// Now and Sleep default to the clock; tests replace them.
	Now   func() time.Time
	Sleep func(ctx context.Context, d time.Duration) error
}

type cached struct {
	etag, lastModified string
	body               []byte
}

// Source is the govinfo BillFetcher.
type Source struct {
	cfg  Config
	base *url.URL

	mu    sync.Mutex // held for the whole of one request: one at a time
	last  time.Time
	cache map[string]cached
}

var _ data.BillFetcher = (*Source)(nil)

// New validates cfg and returns a Source. It makes no request.
func New(cfg Config) (*Source, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBase
	}
	base, err := url.Parse(cfg.BaseURL)
	if err != nil || base.Host == "" || (base.Scheme != "https" && base.Scheme != "http") {
		return nil, fmt.Errorf("govinfo: base url %q: %w", cfg.BaseURL, data.ErrInvalid)
	}
	return &Source{cfg: cfg.withDefaults(), base: base, cache: map[string]cached{}}, nil
}

func (c Config) withDefaults() Config {
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: defaultTimeout}
	}
	if c.UserAgent == "" {
		c.UserAgent = defaultUserAgent
	}
	if c.MinInterval == 0 {
		c.MinInterval = defaultMinInterval
	}
	if c.Backoff == 0 {
		c.Backoff = defaultBackoff
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = defaultRetries
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	if c.Sleep == nil {
		c.Sleep = sleep
	}
	return c
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("govinfo: wait: %w", ctx.Err())
	case <-t.C:
		return nil
	}
}

// statusPath is the BILLSTATUS path of a bill.
func statusPath(b data.BillRef) string {
	return fmt.Sprintf("/bulkdata/BILLSTATUS/%s/%s/BILLSTATUS-%s%s%d.xml", b.Session, b.Type, b.Session, b.Type, b.Number)
}

// FetchRecord fetches and maps the BILLSTATUS file of a bill.
func (s *Source) FetchRecord(ctx context.Context, bill data.BillRef) (data.BillRecord, error) {
	if err := bill.Validate(); err != nil {
		return data.BillRecord{}, fmt.Errorf("govinfo: fetch record: %w", err)
	}
	for _, r := range bill.Session + bill.Type {
		if r < '0' || (r > '9' && r < 'a') || r > 'z' {
			return data.BillRecord{}, fmt.Errorf("govinfo: bill %s: %w", bill, data.ErrInvalid)
		}
	}
	p := statusPath(bill)
	b, err := s.get(ctx, s.base.ResolveReference(&url.URL{Path: p}).String())
	if err != nil {
		return data.BillRecord{}, fmt.Errorf("govinfo: status of %s: %w", bill.ID(), err)
	}
	return parseStatus(b, data.SourceInfo{URL: "https://" + OfficialHost + p, RetrievedAt: s.cfg.Now().UTC()})
}

// FetchOriginal fetches the original XML of one text version at the URL the
// status lists. Only URLs on the official host are fetched.
func (s *Source) FetchOriginal(ctx context.Context, v data.SourceVersion) (data.SourceDocument, error) {
	u, err := url.Parse(v.URL)
	if err != nil || u.Host != OfficialHost || u.Scheme != "https" || !strings.HasSuffix(u.Path, ".xml") {
		return data.SourceDocument{}, fmt.Errorf("govinfo: text url %q: %w", v.URL, data.ErrInvalid)
	}
	target := s.base.ResolveReference(&url.URL{Path: u.Path}).String()
	b, err := s.get(ctx, target)
	if err != nil {
		return data.SourceDocument{}, fmt.Errorf("govinfo: text of %s: %w", v.Ref, err)
	}
	return data.SourceDocument{XML: b, Info: data.SourceInfo{URL: v.URL, RetrievedAt: s.cfg.Now().UTC()}}, nil
}

// get fetches one URL politely, retrying transient failures.
func (s *Source) get(ctx context.Context, target string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delay := s.cfg.Backoff
	for attempt := 0; ; attempt++ {
		if err := s.pace(ctx); err != nil {
			return nil, err
		}
		body, retryAfter, err := s.once(ctx, target)
		s.last = s.cfg.Now()
		if err == nil || !retryable(err) || attempt >= s.cfg.MaxRetries {
			return body, err
		}
		wait := min(max(delay, retryAfter), maxBackoff)
		if err := s.cfg.Sleep(ctx, wait); err != nil {
			return nil, err
		}
		delay *= 2
	}
}

// pace waits until MinInterval has passed since the previous request.
func (s *Source) pace(ctx context.Context) error {
	if s.cfg.MinInterval < 0 || s.last.IsZero() {
		return nil
	}
	if wait := s.cfg.MinInterval - s.cfg.Now().Sub(s.last); wait > 0 {
		return s.cfg.Sleep(ctx, wait)
	}
	return nil
}

func retryable(err error) bool {
	return errors.Is(err, data.ErrRateLimited) || errors.Is(err, data.ErrUnavailable)
}

// once makes one request. retryAfter is the server's Retry-After, if any.
func (s *Source) once(ctx context.Context, target string) (body []byte, retryAfter time.Duration, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("request: %w: %w", data.ErrInvalid, err)
	}
	old, have := s.cache[target]
	s.headers(req, old, have)
	resp, err := s.cfg.HTTPClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, 0, fmt.Errorf("request: %w", ctx.Err())
		}
		return nil, 0, fmt.Errorf("request: %w: %w", data.ErrUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotModified && have {
		return bytes.Clone(old.body), 0, nil
	}
	if err := statusError(resp); err != nil {
		return nil, parseRetryAfter(resp.Header.Get("Retry-After")), err
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, 0, fmt.Errorf("read body: %w: %w", data.ErrUnavailable, err)
	}
	if len(b) > maxBody {
		return nil, 0, fmt.Errorf("body over %d bytes: %w", maxBody, data.ErrInvalid)
	}
	s.cache[target] = cached{
		etag: resp.Header.Get("ETag"), lastModified: resp.Header.Get("Last-Modified"), body: bytes.Clone(b),
	}
	return b, 0, nil
}

// headers sets the polite request headers, conditional when old is known.
func (s *Source) headers(req *http.Request, old cached, have bool) {
	req.Header.Set("User-Agent", s.cfg.UserAgent)
	req.Header.Set("Accept", "application/xml")
	if have && old.etag != "" {
		req.Header.Set("If-None-Match", old.etag)
	}
	if have && old.lastModified != "" {
		req.Header.Set("If-Modified-Since", old.lastModified)
	}
}

// statusError maps a non-200 status to a typed error; nil for 200.
func statusError(resp *http.Response) error {
	switch {
	case resp.StatusCode == http.StatusOK:
		return nil
	case resp.StatusCode == http.StatusNotFound:
		return fmt.Errorf("http 404: %w", data.ErrNotFound)
	case resp.StatusCode == http.StatusTooManyRequests:
		return fmt.Errorf("http 429: %w", data.ErrRateLimited)
	case resp.StatusCode >= http.StatusInternalServerError:
		return fmt.Errorf("http %d: %w", resp.StatusCode, data.ErrUnavailable)
	}
	return fmt.Errorf("http %d: %w", resp.StatusCode, data.ErrInvalid)
}

func parseRetryAfter(h string) time.Duration {
	if n, err := strconv.Atoi(strings.TrimSpace(h)); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	return 0
}
