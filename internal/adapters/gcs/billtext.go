package gcs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// versionDir returns the bucket-relative directory of a version.
func versionDir(ref data.VersionRef) string {
	return path.Join("bills", ref.Bill.String(), "v", ref.Version)
}

// ListVersions returns the listed versions of bill in stage rank order (see
// data.PublicVersions): codes outside the rank table and collapsed referrals
// are left out. A version whose source.xml is written but whose
// text.json is not yet is not published and is left out.
func (s *Store) ListVersions(ctx context.Context, bill data.BillRef) ([]data.VersionSummary, error) {
	if err := bill.Validate(); err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	prefix := path.Join("bills", bill.String(), "v") + "/"
	it := s.bucket.Objects(ctx, &storage.Query{Prefix: prefix, Delimiter: "/"})
	out := []data.VersionSummary{}
	for {
		a, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, mapErr("list versions", err)
		}
		if a.Prefix == "" {
			continue
		}
		code := strings.TrimSuffix(strings.TrimPrefix(a.Prefix, prefix), "/")
		v, err := s.GetVersion(ctx, data.VersionRef{Bill: bill, Version: code})
		if errors.Is(err, data.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, data.VersionSummary{Ref: v.Ref, Type: v.Type, PublishedAt: v.PublishedAt})
	}
	set, err := data.LoadReferrals(ctx, s.GetFile, bill)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	out, _ = data.PublicVersions(out, set)
	return out, nil
}

// GetVersion returns one stored version.
func (s *Store) GetVersion(ctx context.Context, ref data.VersionRef) (data.OfficialVersion, error) {
	if err := ref.Validate(); err != nil {
		return data.OfficialVersion{}, fmt.Errorf("get version: %w", err)
	}
	b, err := readAll(ctx, s.bucket.Object(path.Join(versionDir(ref), "text.json")))
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
// version, the section, or any line of the range is missing. It reads the
// whole text.json (at most about 2 MB, served with a long cache lifetime) and
// slices in memory: line anchors are not byte offsets, and a stored line index
// would be a second source of truth.
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
	if sec == nil || a.To-a.From+1 > len(sec.Lines) {
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
	if err := ref.Validate(); err != nil {
		return nil, fmt.Errorf("get source: %w", err)
	}
	b, err := readAll(ctx, s.bucket.Object(path.Join(versionDir(ref), "source.xml")))
	if err != nil {
		return nil, mapErr("get source "+ref.String(), err)
	}
	return b, nil
}

// PutVersion stores source.xml and text.json at write-once paths. Identical
// content is a no-op; different content is ErrConflict. text.json is written
// last, so a version is visible only once complete; a crash between the two
// writes is repaired by repeating the call. The source URL, retrieval time,
// and SHA-256 of the original are kept as metadata of source.xml.
func (s *Store) PutVersion(ctx context.Context, v data.OfficialVersion, sourceXML []byte) error {
	if err := v.Ref.Validate(); err != nil {
		return fmt.Errorf("put version: %w", err)
	}
	text, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("put version: encode: %w: %w", data.ErrInvalid, err)
	}
	dir := versionDir(v.Ref)
	if err := s.putOnce(ctx, path.Join(dir, "source.xml"), sourceXML, v.Source); err != nil {
		return fmt.Errorf("put version %s: %w", v.Ref, err)
	}
	if err := s.putOnce(ctx, path.Join(dir, "text.json"), text, data.SourceInfo{}); err != nil {
		return fmt.Errorf("put version %s: %w", v.Ref, err)
	}
	return nil
}

// GetSourceInfo returns the source URL and retrieval time recorded with the
// original XML of a version; both are zero when the version was written
// without them.
func (s *Store) GetSourceInfo(ctx context.Context, ref data.VersionRef) (data.SourceInfo, error) {
	if err := ref.Validate(); err != nil {
		return data.SourceInfo{}, fmt.Errorf("get source info: %w", err)
	}
	a, err := s.bucket.Object(path.Join(versionDir(ref), "source.xml")).Attrs(ctx)
	if err != nil {
		return data.SourceInfo{}, mapErr("get source info "+ref.String(), err)
	}
	info := data.SourceInfo{URL: a.Metadata[metaSourceURL]}
	if at := a.Metadata[metaRetrievedAt]; at != "" {
		t, err := time.Parse(time.RFC3339, at)
		if err != nil {
			return data.SourceInfo{}, fmt.Errorf("get source info %s: %w: %w", ref, data.ErrUnavailable, err)
		}
		info.RetrievedAt = t
	}
	return info, nil
}
