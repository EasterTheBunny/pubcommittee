// Package uscode implements data.LawSource against the Office of the Law
// Revision Counsel download pages on uscode.house.gov. Its only network use
// is the configured BaseURL: one request at a time, paced by MinInterval,
// identified by User-Agent, with bounded retries on 429 and 503.
package uscode

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

const (
	defaultBaseURL     = "https://uscode.house.gov"
	defaultMinInterval = 2 * time.Second
	defaultTimeout     = 10 * time.Minute
	defaultMaxBytes    = 1 << 30
	maxPageBytes       = 16 << 20
	maxRetries         = 3
	baseBackoff        = 2 * time.Second
	fastBackoff        = time.Millisecond
	maxTitle           = 99
	dateLayout         = "January 2, 2006"

	currentPath = "/download.html"
	priorPath   = "/download/priorreleasepoints.htm"
)

var (
	// zipLinkRE matches one release point zip link; groups 3 and 4 are the
	// congress and law numbers of the release point ID.
	zipLinkRE = regexp.MustCompile(`releasepoints/us/pl/(\d+)/(\d+)/(?:xml|htm)_usc\d+[a-z]?@(\d+)-(\d+)\.zip`)
	// currentRE matches the "current through" sentence with the release
	// point it describes and its date.
	currentRE = regexp.MustCompile(`[Cc]urrent through Public Law (\d+)-(\d+) \(([A-Z][a-z]+ \d{1,2}, \d{4})\)`)
	// idRE matches a release point ID such as "119-142".
	idRE = regexp.MustCompile(`^(\d+)-(\d+)$`)
)

// Config is the typed configuration of Source. Zero values take the defaults.
// A negative MinInterval disables the wait between requests and shortens
// retry backoff to 1ms; it is meant for tests.
type Config struct {
	// BaseURL is the site root; default "https://uscode.house.gov".
	BaseURL string
	// UserAgent identifies the project on every request. Required.
	UserAgent string
	// HTTPClient performs requests; default a client with a 10 minute timeout.
	HTTPClient *http.Client
	// MinInterval is the minimum gap between the end of one request and the
	// start of the next; default 2s.
	MinInterval time.Duration
	// MaxBytes caps the uncompressed XML of one title; default 1<<30.
	MaxBytes int64
}

// Source is a data.LawSource backed by the Office of the Law Revision
// Counsel. Requests are serialized: the mutex is held for the whole request,
// including retries and the pacing wait.
type Source struct {
	baseURL   string
	userAgent string
	client    *http.Client
	pace      time.Duration
	fast      bool
	maxBytes  int64

	mu       sync.Mutex // held for the whole request
	lastDone time.Time  // when the previous request finished
}

var _ data.LawSource = (*Source)(nil)

// New validates cfg and returns a Source. It makes no network call.
func New(cfg Config) (*Source, error) {
	base, err := checkBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.UserAgent) == "" {
		return nil, fmt.Errorf("uscode: user agent is required: %w", data.ErrInvalid)
	}
	maxBytes := cfg.MaxBytes
	switch {
	case maxBytes == 0:
		maxBytes = defaultMaxBytes
	case maxBytes < 0:
		return nil, fmt.Errorf("uscode: max bytes %d is negative: %w", maxBytes, data.ErrInvalid)
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	s := &Source{
		baseURL:   base,
		userAgent: cfg.UserAgent,
		client:    client,
		maxBytes:  maxBytes,
	}
	s.pace, s.fast = pacing(cfg.MinInterval)
	return s, nil
}

// checkBaseURL returns the base URL without a trailing slash, the default
// when raw is empty. It must be an http(s) URL with a host.
func checkBaseURL(raw string) (string, error) {
	if raw == "" {
		raw = defaultBaseURL
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("uscode: base URL %q must be an http(s) URL: %w", raw, data.ErrInvalid)
	}
	return strings.TrimRight(raw, "/"), nil
}

// pacing returns the wait between requests, or fast=true for no wait.
func pacing(minInterval time.Duration) (pace time.Duration, fast bool) {
	switch {
	case minInterval == 0:
		return defaultMinInterval, false
	case minInterval < 0:
		return 0, true
	default:
		return minInterval, false
	}
}

// ListReleasePoints returns the release points on the current download page
// and, when the site still publishes it, the prior release points page. The
// result is de-duplicated and sorted newest first.
func (s *Source) ListReleasePoints(ctx context.Context) ([]data.ReleasePoint, error) {
	current, err := s.get(ctx, currentPath, maxPageBytes)
	if err != nil {
		return nil, fmt.Errorf("uscode: list release points: %w", err)
	}
	set := releaseSet{}
	if err := set.addPage(current); err != nil {
		return nil, err
	}

	prior, err := s.get(ctx, priorPath, maxPageBytes)
	switch {
	case errors.Is(err, data.ErrNotFound):
		// The site is sometimes in a reduced format without the prior list.
	case err != nil:
		return nil, fmt.Errorf("uscode: list prior release points: %w", err)
	default:
		if err := set.addPage(prior); err != nil {
			return nil, err
		}
	}

	if len(set) == 0 {
		return nil, fmt.Errorf("uscode: no release ids found; release point page layout changed: %w", data.ErrUnavailable)
	}
	return set.sorted()
}

// FetchTitle returns the XML of one US Code title at a release point. The
// bytes are the usc{TT}.xml entry of the release point zip, unchanged.
func (s *Source) FetchTitle(ctx context.Context, rp data.ReleasePoint, title int) ([]byte, error) {
	if title < 1 || title > maxTitle {
		return nil, fmt.Errorf("uscode: title %d outside 1-%d: %w", title, maxTitle, data.ErrInvalid)
	}
	congress, law, err := splitID(rp.ID)
	if err != nil {
		return nil, err
	}
	entry := fmt.Sprintf("usc%02d.xml", title)
	p := fmt.Sprintf("/download/releasepoints/us/pl/%s/%s/xml_usc%02d@%s-%s.zip",
		congress, law, title, congress, law)
	archive, err := s.get(ctx, p, s.maxBytes)
	if err != nil {
		return nil, fmt.Errorf("uscode: fetch title %d at %s: %w", title, rp.ID, err)
	}
	return extractEntry(archive, entry, s.maxBytes)
}

// releaseSet collects release points by ID, keeping the first known date.
type releaseSet map[string]data.ReleasePoint

// addPage adds the release ids linked from page and the page's
// "current through" date, when it names a release point in the set.
func (set releaseSet) addPage(page []byte) error {
	for _, m := range zipLinkRE.FindAllSubmatch(page, -1) {
		id := string(m[3]) + "-" + string(m[4])
		if _, ok := set[id]; !ok {
			set[id] = data.ReleasePoint{ID: id}
		}
	}
	m := currentRE.FindSubmatch(page)
	if m == nil {
		return nil
	}
	id := string(m[1]) + "-" + string(m[2])
	rp, ok := set[id]
	if !ok || !rp.Date.IsZero() {
		return nil
	}
	date, err := time.Parse(dateLayout, string(m[3]))
	if err != nil {
		return fmt.Errorf("uscode: current date %q: %w", m[3], data.ErrUnavailable)
	}
	rp.Date = date
	set[id] = rp
	return nil
}

// sorted returns the release points newest first by (congress, law).
func (set releaseSet) sorted() ([]data.ReleasePoint, error) {
	type keyed struct {
		rp            data.ReleasePoint
		congress, law int
	}
	list := make([]keyed, 0, len(set))
	for _, rp := range set {
		congress, law, err := splitID(rp.ID)
		if err != nil {
			return nil, fmt.Errorf("uscode: release id from page: %w", data.ErrUnavailable)
		}
		c, errC := strconv.Atoi(congress)
		l, errL := strconv.Atoi(law)
		if errC != nil || errL != nil {
			return nil, fmt.Errorf("uscode: release id %q out of range: %w", rp.ID, data.ErrUnavailable)
		}
		list = append(list, keyed{rp: rp, congress: c, law: l})
	}
	slices.SortFunc(list, func(a, b keyed) int {
		if a.congress != b.congress {
			return b.congress - a.congress
		}
		return b.law - a.law
	})
	out := make([]data.ReleasePoint, 0, len(list))
	for _, k := range list {
		out = append(out, k.rp)
	}
	return out, nil
}

// splitID splits a release point ID such as "119-142" into its congress and
// law numbers, keeping the digits as written.
func splitID(id string) (congress, law string, err error) {
	m := idRE.FindStringSubmatch(id)
	if m == nil {
		return "", "", fmt.Errorf("uscode: release point ID %q: %w", id, data.ErrInvalid)
	}
	return m[1], m[2], nil
}

// extractEntry returns the bytes of the single zip entry whose base name
// equals name, case-insensitively. Reads are capped at maxBytes.
func extractEntry(archive []byte, name string, maxBytes int64) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("uscode: open release zip: %w", data.ErrUnavailable)
	}
	var match *zip.File
	for _, f := range zr.File {
		if !strings.EqualFold(path.Base(f.Name), name) {
			continue
		}
		if match != nil {
			return nil, fmt.Errorf("uscode: release zip holds more than one %s: %w", name, data.ErrUnavailable)
		}
		match = f
	}
	if match == nil {
		return nil, fmt.Errorf("uscode: release zip has no %s: %w", name, data.ErrUnavailable)
	}
	rc, err := match.Open()
	if err != nil {
		return nil, fmt.Errorf("uscode: open %s in release zip: %w", name, data.ErrUnavailable)
	}
	defer func() { _ = rc.Close() }()
	body, err := io.ReadAll(io.LimitReader(rc, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("uscode: read %s: %w", name, data.ErrUnavailable)
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("uscode: %s exceeds %d bytes: %w", name, maxBytes, data.ErrUnavailable)
	}
	return body, nil
}

// response is the outcome of one HTTP exchange.
type response struct {
	status     int
	retryAfter time.Duration
	body       []byte
}

// get performs a paced, serialized GET of path under BaseURL and returns the
// body of a 200 response. 429 and 503 are retried up to maxRetries times.
func (s *Source) get(ctx context.Context, p string, limit int64) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for attempt := 0; ; attempt++ {
		if err := s.wait(ctx); err != nil {
			return nil, err
		}
		resp, err := s.send(ctx, s.baseURL+p, limit)
		s.lastDone = time.Now()
		if err != nil {
			return nil, err
		}
		switch resp.status {
		case http.StatusOK:
			return resp.body, nil
		case http.StatusNotFound:
			return nil, fmt.Errorf("uscode: GET %s: status 404: %w", p, data.ErrNotFound)
		case http.StatusTooManyRequests, http.StatusServiceUnavailable:
			if attempt >= maxRetries {
				return nil, exhausted(p, resp.status, attempt+1)
			}
			if err := sleep(ctx, backoff(attempt, resp.retryAfter, s.fast)); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("uscode: GET %s: status %d: %w", p, resp.status, data.ErrUnavailable)
		}
	}
}

// exhausted is the error after the retries of a 429 or 503 ran out: 429 is
// ErrRateLimited, 503 is ErrUnavailable.
func exhausted(p string, status, attempts int) error {
	sentinel := data.ErrUnavailable
	if status == http.StatusTooManyRequests {
		sentinel = data.ErrRateLimited
	}
	return fmt.Errorf("uscode: GET %s: status %d after %d attempts: %w", p, status, attempts, sentinel)
}

// send performs one GET and reads a 200 body capped at limit bytes. The
// caller holds s.mu.
func (s *Source) send(ctx context.Context, target string, limit int64) (response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return response{}, fmt.Errorf("uscode: build request: %w", data.ErrInvalid)
	}
	req.Header.Set("User-Agent", s.userAgent)
	res, err := s.client.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return response{}, fmt.Errorf("uscode: request canceled: %w", ctxErr)
		}
		return response{}, fmt.Errorf("uscode: request to %s failed: %w", target, data.ErrUnavailable)
	}
	defer func() { _ = res.Body.Close() }()
	out := response{
		status:     res.StatusCode,
		retryAfter: parseRetryAfter(res.Header.Get("Retry-After")),
	}
	if res.StatusCode != http.StatusOK {
		return out, nil
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil {
		return response{}, fmt.Errorf("uscode: read response body: %w", data.ErrUnavailable)
	}
	if int64(len(body)) > limit {
		return response{}, fmt.Errorf("uscode: response exceeds %d bytes: %w", limit, data.ErrUnavailable)
	}
	out.body = body
	return out, nil
}

// wait blocks until MinInterval has passed since the previous request ended.
// The caller holds s.mu.
func (s *Source) wait(ctx context.Context) error {
	if s.fast || s.lastDone.IsZero() {
		return nil
	}
	remaining := s.pace - time.Since(s.lastDone)
	if remaining <= 0 {
		return nil
	}
	return sleep(ctx, remaining)
}

// sleep waits for d or until ctx is done.
func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("uscode: wait canceled: %w", ctx.Err())
	case <-t.C:
		return nil
	}
}

// backoff returns the wait before retry number attempt+1: Retry-After when
// the server gave one, else 2s, 4s, 8s. The fast mode always waits 1ms.
func backoff(attempt int, retryAfter time.Duration, fast bool) time.Duration {
	switch {
	case fast:
		return fastBackoff
	case retryAfter > 0:
		return retryAfter
	default:
		return baseBackoff << attempt
	}
}

// parseRetryAfter reads a Retry-After header given in seconds. Other forms
// and bad values yield zero.
func parseRetryAfter(v string) time.Duration {
	secs, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || secs <= 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}
