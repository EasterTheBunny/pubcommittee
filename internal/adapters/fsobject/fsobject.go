// Package fsobject implements BillText and Publisher on a local data root,
// standing in for the Cloud Storage bucket. Objects use the bucket's paths:
// bills/{bill}/v/{version}/source.xml and text.json, plus any exports and
// feeds path a caller names. It is for local runs and tests only and must
// never be imported by a gcp build.
//
// Write-once objects are created by writing a temporary file and hard-linking
// it to the final path, so readers never see a partial file and of several
// racing writers exactly one creates it. Replaceable objects are written by
// temporary file, fsync, and atomic rename under a per-path lock. Each object
// has a sidecar "<path>.meta.json" with content type, cache control, whether
// it is write-once, and the SHA-256 that is also the strong ETag.
//
// Not modelled (for M1-03b to verify against Cloud Storage): object
// generations and preconditions beyond PutReplaceableIf, bucket and object
// IAM, retention locks, Cloud CDN behavior, listing consistency limits,
// object size and request rate limits, and source URL and retrieved-at
// metadata (the Publisher port does not carry them; the sidecar fields stay
// empty until the port does). Paths ending in ".meta.json" are reserved.
package fsobject

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/easterthebunny/pubcommittee/internal/adapters/fsutil"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

const (
	metaSuffix = ".meta.json"
	lockDir    = ".locks"

	cacheImmutable   = "public, max-age=31536000, immutable"
	cacheReplaceable = "public, max-age=60"
)

// Config configures a Store.
type Config struct {
	// Root is the data root; it is created when missing.
	Root string
}

// Store is the filesystem implementation of BillText and Publisher.
type Store struct {
	root string
}

var (
	_ data.BillText  = (*Store)(nil)
	_ data.Publisher = (*Store)(nil)
)

// New validates cfg and returns a Store. It touches only the data root.
func New(cfg Config) (*Store, error) {
	root, err := fsutil.OpenRoot(cfg.Root)
	if err != nil {
		return nil, fmt.Errorf("fsobject: %w", err)
	}
	return &Store{root: root}, nil
}

// Meta is the content of a sidecar file.
type Meta struct {
	ContentType  string `json:"content_type"`
	CacheControl string `json:"cache_control"`
	Immutable    bool   `json:"immutable"`
	SourceURL    string `json:"source_url,omitempty"`
	RetrievedAt  string `json:"retrieved_at,omitempty"`
	SHA256       string `json:"sha256"`
}

// versionDir returns the bucket-relative directory of a version.
func versionDir(ref data.VersionRef) string {
	return path.Join("bills", ref.Bill.String(), "v", ref.Version)
}

func mapErr(op string, err error) error { return fmt.Errorf("%w", fsutil.MapErr(op, err)) }

func replace(p string, b []byte) error {
	if err := fsutil.Replace(p, b); err != nil {
		return fmt.Errorf("replace: %w", err)
	}
	return nil
}

func writeOnce(p string, b []byte) (bool, error) {
	created, err := fsutil.WriteOnce(p, b)
	if err != nil {
		return false, fmt.Errorf("write once: %w", err)
	}
	return created, nil
}

func lock(p string) (func(), error) {
	unlock, err := fsutil.Lock(p)
	if err != nil {
		return nil, fmt.Errorf("lock: %w", err)
	}
	return unlock, nil
}

func (s *Store) abs(rel string) string { return filepath.Join(s.root, filepath.FromSlash(rel)) }

// ListVersions returns the stored versions of bill ordered by PublishedAt,
// then version code.
func (s *Store) ListVersions(ctx context.Context, bill data.BillRef) ([]data.VersionSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	dir := s.abs(path.Join("bills", bill.String(), "v"))
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []data.VersionSummary{}, nil
	}
	if err != nil {
		return nil, mapErr("list versions", err)
	}
	out := []data.VersionSummary{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		v, err := s.GetVersion(ctx, data.VersionRef{Bill: bill, Version: e.Name()})
		if errors.Is(err, data.ErrNotFound) {
			continue // source written, text.json not yet: not published
		}
		if err != nil {
			return nil, err
		}
		out = append(out, data.VersionSummary{Ref: v.Ref, Type: v.Type, PublishedAt: v.PublishedAt})
	}
	slices.SortFunc(out, func(a, b data.VersionSummary) int {
		if c := a.PublishedAt.Compare(b.PublishedAt); c != 0 {
			return c
		}
		return strings.Compare(a.Ref.Version, b.Ref.Version)
	})
	return out, nil
}

// GetVersion returns one stored version.
func (s *Store) GetVersion(ctx context.Context, ref data.VersionRef) (data.OfficialVersion, error) {
	if err := ctx.Err(); err != nil {
		return data.OfficialVersion{}, fmt.Errorf("get version: %w", err)
	}
	if err := ref.Validate(); err != nil {
		return data.OfficialVersion{}, fmt.Errorf("get version: %w", err)
	}
	b, err := os.ReadFile(s.abs(path.Join(versionDir(ref), "text.json")))
	if err != nil {
		return data.OfficialVersion{}, mapErr("get version "+ref.String(), err)
	}
	var v data.OfficialVersion
	if err := json.Unmarshal(b, &v); err != nil {
		return data.OfficialVersion{}, fmt.Errorf("get version %s: decode: %w: %w", ref, data.ErrUnavailable, err)
	}
	return v, nil
}

// GetLines returns the lines an anchor addresses; ErrNotFound when the
// version, the section, or any line of the range is missing.
func (s *Store) GetLines(ctx context.Context, a data.Anchor) ([]data.Line, error) {
	if err := a.Validate(); err != nil {
		return nil, fmt.Errorf("get lines: %w", err)
	}
	v, err := s.GetVersion(ctx, a.Version)
	if err != nil {
		return nil, fmt.Errorf("get lines: %w", err)
	}
	var sec *data.Section
	for i := range v.Sections {
		if slices.Equal(v.Sections[i].Path, a.Section) {
			sec = &v.Sections[i]
			break
		}
	}
	if sec == nil {
		return nil, fmt.Errorf("get lines %s section %v: %w", a.Version, a.Section, data.ErrNotFound)
	}
	if a.To-a.From+1 > len(sec.Lines) {
		return nil, fmt.Errorf("get lines %s section %v: %w", a.Version, a.Section, data.ErrNotFound)
	}
	out := make([]data.Line, 0, a.To-a.From+1)
	for n := a.From; n <= a.To; n++ {
		i := slices.IndexFunc(sec.Lines, func(l data.Line) bool { return l.N == n })
		if i < 0 {
			return nil, fmt.Errorf("get lines %s line %d: %w", a.Version, n, data.ErrNotFound)
		}
		out = append(out, sec.Lines[i])
	}
	return out, nil
}

// GetSource returns the original source XML of a version, byte for byte.
func (s *Store) GetSource(ctx context.Context, ref data.VersionRef) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("get source: %w", err)
	}
	if err := ref.Validate(); err != nil {
		return nil, fmt.Errorf("get source: %w", err)
	}
	b, err := os.ReadFile(s.abs(path.Join(versionDir(ref), "source.xml")))
	if err != nil {
		return nil, mapErr("get source "+ref.String(), err)
	}
	return b, nil
}

// PutVersion stores source.xml and text.json at write-once paths. Identical
// content is a no-op; different content is ErrConflict. text.json is written
// last, so a version is visible only once complete; a crash between the two
// writes is repaired by repeating the call.
func (s *Store) PutVersion(ctx context.Context, v data.OfficialVersion, sourceXML []byte) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("put version: %w", err)
	}
	if err := v.Ref.Validate(); err != nil {
		return fmt.Errorf("put version: %w", err)
	}
	text, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("put version: encode: %w: %w", data.ErrInvalid, err)
	}
	dir := versionDir(v.Ref)
	if err := s.putOnce(path.Join(dir, "source.xml"), sourceXML); err != nil {
		return fmt.Errorf("put version %s: %w", v.Ref, err)
	}
	if err := s.putOnce(path.Join(dir, "text.json"), text); err != nil {
		return fmt.Errorf("put version %s: %w", v.Ref, err)
	}
	return nil
}

// PutImmutable stores a file at a write-once path. Identical bytes are a
// no-op; different bytes, or a path written by PutReplaceable, are ErrConflict.
func (s *Store) PutImmutable(ctx context.Context, p string, body []byte) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("put immutable: %w", err)
	}
	if err := s.checkPath(p); err != nil {
		return fmt.Errorf("put immutable: %w", err)
	}
	unlock, err := lock(s.lockPath(p))
	if err != nil {
		return fmt.Errorf("put immutable %s: %w", p, err)
	}
	defer unlock()
	if err := s.putOnce(p, body); err != nil {
		return fmt.Errorf("put immutable %s: %w", p, err)
	}
	return nil
}

// PutReplaceable stores a file that later calls may replace and returns the
// lowercase hex SHA-256 of its content as ETag. A write-once path is ErrConflict.
func (s *Store) PutReplaceable(ctx context.Context, p string, body []byte) (string, error) {
	return s.put(ctx, p, body, nil)
}

// PutReplaceableIf is PutReplaceable with a generation check: it replaces the
// file only when its current ETag equals ifMatch ("" means the file must not
// exist yet); otherwise a stale writer gets ErrConflict.
func (s *Store) PutReplaceableIf(ctx context.Context, p string, body []byte, ifMatch string) (string, error) {
	return s.put(ctx, p, body, &ifMatch)
}

func (s *Store) put(ctx context.Context, p string, body []byte, ifMatch *string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("put replaceable: %w", err)
	}
	if err := s.checkPath(p); err != nil {
		return "", fmt.Errorf("put replaceable: %w", err)
	}
	unlock, err := lock(s.lockPath(p))
	if err != nil {
		return "", fmt.Errorf("put replaceable %s: %w", p, err)
	}
	defer unlock()
	if err := s.checkReplace(p, ifMatch); err != nil {
		return "", fmt.Errorf("put replaceable %s: %w", p, err)
	}
	etag := sum(body)
	m := Meta{ContentType: contentType(p), CacheControl: cacheReplaceable, SHA256: etag}
	// The sidecar goes first: a crash between the two leaves a sidecar that
	// names the new content while the body is old, which the next write fixes.
	if err := s.writeMeta(p, m); err != nil {
		return "", fmt.Errorf("put replaceable %s: %w", p, err)
	}
	if err := replace(s.abs(p), body); err != nil {
		return "", fmt.Errorf("put replaceable %s: %w", p, err)
	}
	return etag, nil
}

// checkReplace returns ErrConflict when p is write-once or ifMatch is stale.
func (s *Store) checkReplace(p string, ifMatch *string) error {
	cur, err := s.readMeta(p)
	exists := err == nil
	if err != nil && !errors.Is(err, data.ErrNotFound) {
		return err
	}
	if exists && cur.Immutable {
		return fmt.Errorf("path is write-once: %w", data.ErrConflict)
	}
	if ifMatch != nil && cur.SHA256 != *ifMatch {
		return fmt.Errorf("etag %q is stale: %w", *ifMatch, data.ErrConflict)
	}
	return nil
}

// GetFile reads a published file; ErrNotFound if absent.
func (s *Store) GetFile(ctx context.Context, p string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("get file: %w", err)
	}
	if err := s.checkPath(p); err != nil {
		return nil, fmt.Errorf("get file: %w", err)
	}
	b, err := os.ReadFile(s.abs(p))
	if err != nil {
		return nil, mapErr("get file "+p, err)
	}
	return b, nil
}

// ETag returns the strong ETag (SHA-256 hex) of a stored file.
func (s *Store) ETag(ctx context.Context, p string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("etag: %w", err)
	}
	if err := s.checkPath(p); err != nil {
		return "", fmt.Errorf("etag: %w", err)
	}
	m, err := s.readMeta(p)
	if err != nil {
		return "", fmt.Errorf("etag %s: %w", p, err)
	}
	return m.SHA256, nil
}

// putOnce writes body at a write-once path. An existing identical write-once
// file is a no-op; anything else already there is ErrConflict.
func (s *Store) putOnce(p string, body []byte) error {
	abs := s.abs(p)
	created, err := writeOnce(abs, body)
	if err != nil {
		return err
	}
	if created {
		// The body is complete before its sidecar; readers of the body need none.
		return s.writeMeta(p, Meta{
			ContentType: contentType(p), CacheControl: cacheImmutable, Immutable: true, SHA256: sum(body),
		})
	}
	if s.sameOnce(p, body) {
		return nil
	}
	return fmt.Errorf("%s: different content: %w", p, data.ErrConflict)
}

// sameOnce reports whether p already holds body as a write-once object.
func (s *Store) sameOnce(p string, body []byte) bool {
	old, err := os.ReadFile(s.abs(p))
	if err != nil || !bytes.Equal(old, body) {
		return false
	}
	m, err := s.readMeta(p)
	if errors.Is(err, data.ErrNotFound) {
		// A crash between body and sidecar: only version files are repaired.
		return isVersionFile(p)
	}
	return err == nil && m.Immutable
}

func isVersionFile(p string) bool {
	return strings.HasPrefix(p, "bills/") && (strings.HasSuffix(p, "/source.xml") || strings.HasSuffix(p, "/text.json"))
}

func (s *Store) readMeta(p string) (Meta, error) {
	b, err := os.ReadFile(s.abs(p + metaSuffix))
	if err != nil {
		return Meta{}, mapErr("read meta "+p, err)
	}
	var m Meta
	if err := json.Unmarshal(b, &m); err != nil {
		return Meta{}, fmt.Errorf("read meta %s: %w: %w", p, data.ErrUnavailable, err)
	}
	return m, nil
}

func (s *Store) writeMeta(p string, m Meta) error {
	b, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("encode meta: %w: %w", data.ErrInvalid, err)
	}
	return replace(s.abs(p+metaSuffix), b)
}

func (s *Store) checkPath(p string) error {
	if err := fsutil.CleanRel(p); err != nil {
		return err //nolint:wrapcheck // already typed and names the path
	}
	if strings.HasSuffix(p, metaSuffix) || strings.HasPrefix(p, lockDir+"/") {
		return fmt.Errorf("path %q is reserved: %w", p, data.ErrInvalid)
	}
	return nil
}

// lockPath maps an object path to its lock file under the data root.
func (s *Store) lockPath(p string) string {
	return filepath.Join(s.root, lockDir, sum([]byte(p))+".lock")
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func contentType(p string) string {
	switch path.Ext(p) {
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".atom":
		return "application/atom+xml"
	case ".txt":
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}
