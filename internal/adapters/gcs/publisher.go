package gcs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"path"
	"time"

	"cloud.google.com/go/storage"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// FileInfo describes a stored file for strong ETags and cache headers.
type FileInfo struct {
	// ETag is the lowercase hex SHA-256 of the content, the same value
	// PutReplaceable returns. Objects written by other tools fall back to
	// the hex MD5 the service reports.
	ETag string
	// Generation changes on every rewrite of the path.
	Generation   int64
	Size         int64
	ContentType  string
	CacheControl string
	// Immutable is true for write-once files.
	Immutable bool
}

// PutImmutable stores a file at a write-once path. Identical bytes are a
// no-op; different bytes, or a path written by PutReplaceable, are ErrConflict.
func (s *Store) PutImmutable(ctx context.Context, p string, body []byte) error {
	if err := checkPath(p); err != nil {
		return fmt.Errorf("put immutable: %w", err)
	}
	if err := s.putOnce(ctx, p, body, data.SourceInfo{}); err != nil {
		return fmt.Errorf("put immutable %s: %w", p, err)
	}
	return nil
}

// PutReplaceable stores a file that later calls may replace and returns the
// lowercase hex SHA-256 of its content as ETag. A write-once path is ErrConflict.
func (s *Store) PutReplaceable(ctx context.Context, p string, body []byte) (string, error) {
	return s.replace(ctx, p, body, nil)
}

// PutReplaceableIf is PutReplaceable with a check: it replaces the file only
// when its current ETag equals ifMatch ("" means the file must not exist yet);
// otherwise a stale writer gets ErrConflict.
func (s *Store) PutReplaceableIf(ctx context.Context, p string, body []byte, ifMatch string) (string, error) {
	return s.replace(ctx, p, body, &ifMatch)
}

func (s *Store) replace(ctx context.Context, p string, body []byte, ifMatch *string) (string, error) {
	if err := checkPath(p); err != nil {
		return "", fmt.Errorf("put replaceable: %w", err)
	}
	obj := s.bucket.Object(p)
	etag := sum(body)
	for range replaceTries {
		cond, err := replaceCond(ctx, obj, p, ifMatch)
		if err != nil {
			return "", err
		}
		meta := map[string]string{metaSHA256: etag}
		err = write(ctx, obj.If(cond), body, contentType(p), cacheReplaceable, meta)
		switch {
		case err == nil:
			return etag, nil
		case !isStatus(err, http.StatusPreconditionFailed):
			return "", mapErr("put replaceable "+p, err)
		case ifMatch != nil:
			return "", fmt.Errorf("put replaceable %s: changed meanwhile: %w", p, data.ErrConflict)
		}
		// Unconditional replace lost a race with another writer: look again.
	}
	return "", fmt.Errorf("put replaceable %s: too many concurrent writers: %w", p, data.ErrConflict)
}

// replaceCond returns the precondition of a replace of p: the current
// generation, or "does not exist". A write-once path and a stale ifMatch are
// ErrConflict.
func replaceCond(
	ctx context.Context, obj *storage.ObjectHandle, p string, ifMatch *string,
) (storage.Conditions, error) {
	cond := storage.Conditions{DoesNotExist: true}
	var cur string
	attrs, err := obj.Attrs(ctx)
	switch {
	case err == nil:
		if attrs.Metadata[metaImmutable] == metaTrue {
			return cond, fmt.Errorf("put replaceable %s: path is write-once: %w", p, data.ErrConflict)
		}
		cond = storage.Conditions{GenerationMatch: attrs.Generation}
		cur = etagOf(attrs)
	case !isNotExist(err):
		return cond, mapErr("put replaceable "+p, err)
	}
	if ifMatch != nil && cur != *ifMatch {
		return cond, fmt.Errorf("put replaceable %s: etag %q is stale: %w", p, *ifMatch, data.ErrConflict)
	}
	return cond, nil
}

// GetFile reads a published file; ErrNotFound if absent.
func (s *Store) GetFile(ctx context.Context, p string) ([]byte, error) {
	if err := checkPath(p); err != nil {
		return nil, fmt.Errorf("get file: %w", err)
	}
	b, err := readAll(ctx, s.bucket.Object(p))
	if err != nil {
		return nil, mapErr("get file "+p, err)
	}
	return b, nil
}

// ETag returns the strong ETag (SHA-256 hex) of a stored file.
func (s *Store) ETag(ctx context.Context, p string) (string, error) {
	info, err := s.Stat(ctx, p)
	if err != nil {
		return "", err
	}
	return info.ETag, nil
}

// Stat returns the metadata of a stored file without reading its content.
func (s *Store) Stat(ctx context.Context, p string) (FileInfo, error) {
	if err := checkPath(p); err != nil {
		return FileInfo{}, fmt.Errorf("stat: %w", err)
	}
	a, err := s.bucket.Object(p).Attrs(ctx)
	if err != nil {
		return FileInfo{}, mapErr("stat "+p, err)
	}
	return FileInfo{
		ETag: etagOf(a), Generation: a.Generation, Size: a.Size, ContentType: a.ContentType,
		CacheControl: a.CacheControl, Immutable: a.Metadata[metaImmutable] == metaTrue,
	}, nil
}

// putOnce writes body at a write-once path. An existing identical write-once
// object is a no-op; anything else already there is ErrConflict.
func (s *Store) putOnce(ctx context.Context, p string, body []byte, info data.SourceInfo) error {
	meta := map[string]string{metaSHA256: sum(body), metaImmutable: metaTrue}
	if info.URL != "" {
		meta[metaSourceURL] = info.URL
	}
	if !info.RetrievedAt.IsZero() {
		meta[metaRetrievedAt] = info.RetrievedAt.UTC().Format(time.RFC3339)
	}
	obj := s.bucket.Object(p)
	err := write(ctx, obj.If(storage.Conditions{DoesNotExist: true}), body, contentType(p), cacheImmutable, meta)
	if err == nil {
		return nil
	}
	if !isStatus(err, http.StatusPreconditionFailed) {
		return mapErr("write once", err)
	}
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return mapErr("write once: read existing", err)
	}
	if attrs.Metadata[metaImmutable] == metaTrue && attrs.Metadata[metaSHA256] == meta[metaSHA256] {
		old, err := readAll(ctx, obj.Generation(attrs.Generation))
		if err != nil {
			return mapErr("write once: read existing", err)
		}
		if bytes.Equal(old, body) {
			return nil
		}
	}
	return fmt.Errorf("%s: different content: %w", p, data.ErrConflict)
}

// write uploads body with the given conditions and metadata, and has the
// service verify the CRC32C of what it received.
func write(
	ctx context.Context, obj *storage.ObjectHandle, body []byte, ctype, cache string, meta map[string]string,
) error {
	w := obj.NewWriter(ctx)
	w.ContentType = ctype
	w.CacheControl = cache
	w.Metadata = meta
	w.CRC32C = crc32.Checksum(body, crc32.MakeTable(crc32.Castagnoli))
	w.SendCRC32C = true
	if _, err := w.Write(body); err != nil {
		_ = w.Close() // the write error is the one to report
		return err    //nolint:wrapcheck // callers map it with mapErr
	}
	return w.Close() //nolint:wrapcheck // callers map it with mapErr
}

func readAll(ctx context.Context, obj *storage.ObjectHandle) ([]byte, error) {
	r, err := obj.NewReader(ctx)
	if err != nil {
		return nil, err //nolint:wrapcheck // callers map it with mapErr
	}
	defer func() { _ = r.Close() }()
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err //nolint:wrapcheck // callers map it with mapErr
	}
	return b, nil
}

func isNotExist(err error) bool { return errors.Is(err, storage.ErrObjectNotExist) }

// etagOf returns the SHA-256 recorded at write time, or the hex MD5 for an
// object written by another tool.
func etagOf(a *storage.ObjectAttrs) string {
	if h := a.Metadata[metaSHA256]; h != "" {
		return h
	}
	return hex.EncodeToString(a.MD5)
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
