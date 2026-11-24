// Package gcs implements BillText and Publisher (and the file-backed ports
// that sit on a Publisher) on a Cloud Storage bucket. Objects use the same
// paths as the local fsobject adapter: bills/{bill}/v/{version}/source.xml and
// text.json, generated/{task}/{bill}/v/{version}/{key}.json, plus any exports
// and feeds path a caller names. The package uses the JSON API client of
// cloud.google.com/go/storage (M1-00): credentials come from workload identity
// (Application Default Credentials), or from STORAGE_EMULATOR_HOST for the
// emulator target, never from the configuration.
//
// Write-once is enforced here with the "does not exist" precondition
// (ifGenerationMatch=0), so of several racing writers exactly one creates the
// object and the rest get ErrConflict, or a no-op when their bytes are
// identical. Replaceable objects are written with a generation precondition,
// so a writer never overwrites a write-once object and never loses a race
// silently. Every object carries custom metadata: sha256 (also the strong
// ETag), immutable, and for source.xml the source URL and retrieval time.
//
// Not modelled by the stand-in and left to the dev test bucket (stage C1):
// IAM, ETag format and change rules, read-after-write and listing
// consistency under latency, the per-object write-rate limit, quotas, and
// bucket settings such as retention and versioning.
package gcs

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"cloud.google.com/go/storage"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

const (
	cacheImmutable   = "public, max-age=31536000, immutable"
	cacheReplaceable = "public, max-age=60"

	metaTrue        = "true"
	metaSHA256      = "sha256"
	metaImmutable   = "immutable"
	metaSourceURL   = "source-url"
	metaRetrievedAt = "retrieved-at"

	defaultAttempts  = 4
	defaultRetryBase = 200 * time.Millisecond
	maxRetryBackoff  = 2 * time.Second

	maxPathBytes = 1024
	// replaceTries bounds the retries of a replace that lost a race.
	replaceTries = 5
)

// bucketName follows the Cloud Storage naming rules for single-label names
// (dots are allowed by the service but not used here).
var bucketName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,61}[a-z0-9]$`)

// Config configures a Store.
type Config struct {
	// Bucket is the bucket name; the bucket must exist.
	Bucket string
	// Transport, when set, carries every request. Tests inject faults
	// through it; production leaves it nil.
	Transport http.RoundTripper
	// MaxAttempts bounds the attempts of one retryable call (default 4);
	// RetryBase is the first backoff (default 200 ms, doubling, at most 2 s).
	// The client retries only idempotent calls: reads, and writes that carry a
	// precondition, which is every write this adapter makes.
	MaxAttempts int
	RetryBase   time.Duration
}

// Store is the Cloud Storage implementation of BillText, Publisher, and the
// file-backed ports.
type Store struct {
	client *storage.Client
	bucket *storage.BucketHandle
	name   string
}

var (
	_ data.BillText         = (*Store)(nil)
	_ data.Publisher        = (*Store)(nil)
	_ data.FileStore        = (*Store)(nil)
	_ data.ComparisonReader = (*Store)(nil)
	_ data.ComparisonWriter = (*Store)(nil)
	_ data.SourceInfoReader = (*Store)(nil)
	_ data.CitationReader   = (*Store)(nil)
	_ data.CitationWriter   = (*Store)(nil)
	_ data.LawReader        = (*Store)(nil)
	_ data.LawWriter        = (*Store)(nil)
	_ data.FeedReader       = (*Store)(nil)
	_ data.FeedWriter       = (*Store)(nil)
	_ data.ExportReader     = (*Store)(nil)
	_ data.ExportWriter     = (*Store)(nil)
)

// New validates cfg and returns a Store. It makes no network call; a missing
// bucket shows on first use.
func New(ctx context.Context, cfg Config) (*Store, error) {
	if !bucketName.MatchString(cfg.Bucket) {
		return nil, fmt.Errorf("gcs: bucket name %q: %w", cfg.Bucket, data.ErrInvalid)
	}
	var opts []option.ClientOption
	if cfg.Transport != nil {
		opts = append(opts, option.WithHTTPClient(&http.Client{Transport: cfg.Transport}))
	}
	c, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, mapErr("gcs: new client", err)
	}
	c.SetRetry(
		storage.WithPolicy(storage.RetryIdempotent),
		storage.WithMaxAttempts(cmp.Or(cfg.MaxAttempts, defaultAttempts)),
		storage.WithBackoff(gax.Backoff{Initial: cmp.Or(cfg.RetryBase, defaultRetryBase), Max: maxRetryBackoff}),
	)
	return &Store{client: c, bucket: c.Bucket(cfg.Bucket), name: cfg.Bucket}, nil
}

// Close releases the client.
func (s *Store) Close() error {
	if err := s.client.Close(); err != nil {
		return mapErr("gcs: close", err)
	}
	return nil
}

// EnsureBucket creates the bucket in project when it does not exist. It is for
// the emulator target and tests only: production buckets belong to
// infrastructure code.
func (s *Store) EnsureBucket(ctx context.Context, project string) error {
	if project == "" {
		return fmt.Errorf("ensure bucket: empty project: %w", data.ErrInvalid)
	}
	_, err := s.bucket.Attrs(ctx)
	if err == nil {
		return nil
	}
	if !errors.Is(err, storage.ErrBucketNotExist) {
		return mapErr("ensure bucket", err)
	}
	if err := s.bucket.Create(ctx, project, nil); err != nil && !isStatus(err, http.StatusConflict) {
		return mapErr("ensure bucket", err)
	}
	return nil
}

// isStatus reports whether err is a Google API error with this HTTP status.
func isStatus(err error, code int) bool {
	var ge *googleapi.Error
	return errors.As(err, &ge) && ge.Code == code
}

// mapErr wraps a storage failure in the typed data errors: a missing object
// is ErrNotFound, a failed precondition ErrConflict, a throttle ErrRateLimited,
// a malformed request ErrInvalid, and everything else, including permission
// and transport failures, ErrUnavailable. Context errors pass through.
func mapErr(op string, err error) error {
	var ge *googleapi.Error
	switch {
	case err == nil:
		return nil
	case errors.Is(err, data.ErrNotFound), errors.Is(err, data.ErrConflict), errors.Is(err, data.ErrInvalid),
		errors.Is(err, data.ErrUnavailable), errors.Is(err, data.ErrRateLimited),
		errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("%s: %w", op, err)
	case errors.Is(err, storage.ErrObjectNotExist):
		return fmt.Errorf("%s: %w", op, data.ErrNotFound)
	case errors.As(err, &ge):
		return fmt.Errorf("%s: %w: %w", op, statusErr(ge.Code), err)
	default:
		return fmt.Errorf("%s: %w: %w", op, data.ErrUnavailable, err)
	}
}

func statusErr(code int) error {
	switch code {
	case http.StatusNotFound:
		return data.ErrNotFound
	case http.StatusPreconditionFailed, http.StatusConflict:
		return data.ErrConflict
	case http.StatusTooManyRequests:
		return data.ErrRateLimited
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge:
		return data.ErrInvalid
	default:
		return data.ErrUnavailable
	}
}

// checkPath validates an object path: relative, slash-separated, no empty, "."
// or ".." segment, valid UTF-8 without control characters or backslash, and
// at most 1024 bytes.
func checkPath(p string) error {
	if p == "" || len(p) > maxPathBytes || !utf8.ValidString(p) || strings.HasPrefix(p, "/") ||
		strings.ContainsFunc(p, badPathRune) || slices.ContainsFunc(strings.Split(p, "/"), badSegment) {
		return fmt.Errorf("path %q: %w", p, data.ErrInvalid)
	}
	return nil
}

func badPathRune(r rune) bool { return r < ' ' || r == 0x7f || r == '\\' }

func badSegment(seg string) bool { return seg == "" || seg == "." || seg == ".." }
