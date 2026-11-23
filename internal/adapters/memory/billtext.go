package memory

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"slices"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// ListVersions returns the stored versions of bill, ordered by PublishedAt
// then version code. A bill with no versions gives an empty list.
func (s *Store) ListVersions(ctx context.Context, bill data.BillRef) ([]data.VersionSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []data.VersionSummary{}
	for _, v := range s.versions {
		if v.Ref.Bill != bill {
			continue
		}
		out = append(out, data.VersionSummary{Ref: v.Ref, Type: v.Type, PublishedAt: v.PublishedAt})
	}
	slices.SortFunc(out, func(a, b data.VersionSummary) int {
		if c := a.PublishedAt.Compare(b.PublishedAt); c != 0 {
			return c
		}
		switch {
		case a.Ref.Version < b.Ref.Version:
			return -1
		case a.Ref.Version > b.Ref.Version:
			return 1
		default:
			return 0
		}
	})
	return out, nil
}

// GetVersion returns a copy of one stored version.
func (s *Store) GetVersion(ctx context.Context, ref data.VersionRef) (data.OfficialVersion, error) {
	if err := ctx.Err(); err != nil {
		return data.OfficialVersion{}, fmt.Errorf("get version: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.versions[ref.String()]
	if !ok {
		return data.OfficialVersion{}, fmt.Errorf("get version %s: %w", ref, data.ErrNotFound)
	}
	return cloneVersion(v), nil
}

// GetLines returns copies of the lines the anchor addresses. ErrNotFound
// when the version, the section, or any line of the range is missing.
func (s *Store) GetLines(ctx context.Context, a data.Anchor) ([]data.Line, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("get lines: %w", err)
	}
	if err := a.Validate(); err != nil {
		return nil, fmt.Errorf("get lines: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.versions[a.Version.String()]
	if !ok {
		return nil, fmt.Errorf("get lines %s: %w", a.Version, data.ErrNotFound)
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
	// Line numbers are unique within a version, so a range longer than the
	// section cannot be satisfied; this also bounds the allocation.
	if a.To-a.From+1 > len(sec.Lines) {
		return nil, fmt.Errorf("get lines %s section %v: %w", a.Version, a.Section, data.ErrNotFound)
	}
	out := make([]data.Line, 0, a.To-a.From+1)
	for n := a.From; n <= a.To; n++ {
		i := findLine(sec.Lines, n)
		if i < 0 {
			return nil, fmt.Errorf("get lines %s line %d: %w", a.Version, n, data.ErrNotFound)
		}
		out = append(out, sec.Lines[i])
	}
	return out, nil
}

// GetSource returns a copy of the original source XML of a version.
func (s *Store) GetSource(ctx context.Context, ref data.VersionRef) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("get source: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	src, ok := s.sources[ref.String()]
	if !ok {
		return nil, fmt.Errorf("get source %s: %w", ref, data.ErrNotFound)
	}
	return cloneBytes(src), nil
}

// PutVersion stores a version and its source XML at a write-once path.
// Identical content is a no-op; different content is ErrConflict.
func (s *Store) PutVersion(ctx context.Context, v data.OfficialVersion, sourceXML []byte) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("put version: %w", err)
	}
	if err := v.Ref.Validate(); err != nil {
		return fmt.Errorf("put version: %w", err)
	}
	key := v.Ref.String()
	info := v.Source
	v.Source = data.SourceInfo{} // provenance is kept beside the source, not in the version
	want := cloneVersion(v)
	wantSrc := cloneBytes(sourceXML)
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.versions[key]; ok {
		if reflect.DeepEqual(old, want) && bytes.Equal(s.sources[key], wantSrc) {
			return nil
		}
		return fmt.Errorf("put version %s: different content: %w", key, data.ErrConflict)
	}
	s.versions[key] = want
	s.sources[key] = wantSrc
	s.sourceInfo[key] = info
	return nil
}

// GetSourceInfo returns where the original of a version came from; the first
// write wins, as with the source itself.
func (s *Store) GetSourceInfo(ctx context.Context, ref data.VersionRef) (data.SourceInfo, error) {
	if err := ctx.Err(); err != nil {
		return data.SourceInfo{}, fmt.Errorf("get source info: %w", err)
	}
	if err := ref.Validate(); err != nil {
		return data.SourceInfo{}, fmt.Errorf("get source info: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	info, ok := s.sourceInfo[ref.String()]
	if !ok {
		return data.SourceInfo{}, fmt.Errorf("get source info %s: %w", ref, data.ErrNotFound)
	}
	return info, nil
}

// PutImmutable stores a file at a write-once path. Identical bytes are a
// no-op; different bytes, or a path written by PutReplaceable, are ErrConflict.
func (s *Store) PutImmutable(ctx context.Context, path string, body []byte) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("put immutable: %w", err)
	}
	if path == "" {
		return fmt.Errorf("put immutable: empty path: %w", data.ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.files[path]; ok {
		if s.once[path] && bytes.Equal(old, body) {
			return nil
		}
		return fmt.Errorf("put immutable %s: different content: %w", path, data.ErrConflict)
	}
	s.files[path] = cloneBytes(body)
	s.once[path] = true
	return nil
}

// PutReplaceable stores a file that later calls may replace and returns the
// lowercase hex SHA-256 of its content as ETag. A path written by
// PutImmutable is ErrConflict.
func (s *Store) PutReplaceable(ctx context.Context, path string, body []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("put replaceable: %w", err)
	}
	if path == "" {
		return "", fmt.Errorf("put replaceable: empty path: %w", data.ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.once[path] {
		return "", fmt.Errorf("put replaceable %s: path is write-once: %w", path, data.ErrConflict)
	}
	s.files[path] = cloneBytes(body)
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

// GetFile returns a copy of a published file.
func (s *Store) GetFile(ctx context.Context, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("get file: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	body, ok := s.files[path]
	if !ok {
		return nil, fmt.Errorf("get file %s: %w", path, data.ErrNotFound)
	}
	return cloneBytes(body), nil
}

// findLine returns the index of the line numbered n, or -1 when there is none.
func findLine(lines []data.Line, n int) int {
	for i, l := range lines {
		if l.N == n {
			return i
		}
	}
	return -1
}

// cloneVersion returns a deep copy of v. Nil slices stay nil.
func cloneVersion(v data.OfficialVersion) data.OfficialVersion {
	out := v
	if v.Sections != nil {
		out.Sections = make([]data.Section, len(v.Sections))
		for i, sec := range v.Sections {
			out.Sections[i] = cloneSection(sec)
		}
	}
	return out
}

// cloneSection returns a deep copy of sec. Nil slices stay nil.
func cloneSection(sec data.Section) data.Section {
	out := sec
	out.Path = slices.Clone(sec.Path)
	if sec.Lines != nil {
		out.Lines = make([]data.Line, len(sec.Lines))
		copy(out.Lines, sec.Lines)
	}
	return out
}

// cloneBytes returns a copy of b. A nil slice stays nil.
func cloneBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	return bytes.Clone(b)
}
