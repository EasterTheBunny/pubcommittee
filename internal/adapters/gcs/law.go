package gcs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// PutLawSection stores a US Code section as an immutable file at the path of
// its ref. Identical content is a no-op; different content is ErrConflict.
func (s *Store) PutLawSection(ctx context.Context, sec data.LawSection) error {
	if err := sec.Ref.Validate(); err != nil {
		return fmt.Errorf("put law section: %w", err)
	}
	body, err := json.Marshal(sec)
	if err != nil {
		return fmt.Errorf("put law section %s: encode: %w: %w", sec.Ref, data.ErrInvalid, err)
	}
	if err := s.PutImmutable(ctx, sec.Ref.ObjectPath(), body); err != nil {
		return fmt.Errorf("put law section %s: %w", sec.Ref, err)
	}
	return nil
}

// GetLawSection returns one section with its subdivisions.
func (s *Store) GetLawSection(ctx context.Context, ref data.ProvisionRef) (data.LawSection, error) {
	if err := ref.Validate(); err != nil {
		return data.LawSection{}, fmt.Errorf("get law section: %w", err)
	}
	body, err := s.GetFile(ctx, ref.ObjectPath())
	if err != nil {
		return data.LawSection{}, fmt.Errorf("get law section %s: %w", ref, err)
	}
	var sec data.LawSection
	if err := json.Unmarshal(body, &sec); err != nil {
		return data.LawSection{}, fmt.Errorf("get law section %s: decode: %w: %w", ref, data.ErrInvalid, err)
	}
	return sec, nil
}

// PutRelease stores a release record, write-once.
func (s *Store) PutRelease(ctx context.Context, r data.Release) error {
	if err := r.Validate(); err != nil {
		return fmt.Errorf("put release: %w", err)
	}
	body, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("put release %s: encode: %w: %w", r.ID, data.ErrInvalid, err)
	}
	if err := s.PutImmutable(ctx, r.ObjectPath(), body); err != nil {
		return fmt.Errorf("put release %s: %w", r.ID, err)
	}
	return nil
}

// GetRelease returns the release with this id.
func (s *Store) GetRelease(ctx context.Context, id string) (data.Release, error) {
	body, err := s.GetFile(ctx, data.ReleasePath(id))
	if err != nil {
		return data.Release{}, fmt.Errorf("get release %s: %w", id, err)
	}
	var r data.Release
	if err := json.Unmarshal(body, &r); err != nil {
		return data.Release{}, fmt.Errorf("get release %s: decode: %w: %w", id, data.ErrInvalid, err)
	}
	return r, nil
}

// CurrentRelease returns the release the current pointer names.
func (s *Store) CurrentRelease(ctx context.Context) (data.Release, error) {
	body, err := s.GetFile(ctx, data.CurrentReleasePath)
	if err != nil {
		return data.Release{}, fmt.Errorf("current release: %w", err)
	}
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return data.Release{}, fmt.Errorf("current release: decode: %w: %w", data.ErrInvalid, err)
	}
	return s.GetRelease(ctx, p.ID)
}

// SetCurrentRelease points the current pointer at a stored release, unless
// the current one is newer.
func (s *Store) SetCurrentRelease(ctx context.Context, id string) error {
	r, err := s.GetRelease(ctx, id)
	if err != nil {
		return fmt.Errorf("set current release: %w", err)
	}
	if cur, err := s.CurrentRelease(ctx); err == nil && cur.Date.After(r.Date) {
		return nil
	}
	body, err := json.Marshal(struct {
		ID string `json:"id"`
	}{id})
	if err != nil {
		return fmt.Errorf("set current release: encode: %w: %w", data.ErrInvalid, err)
	}
	if _, err := s.PutReplaceable(ctx, data.CurrentReleasePath, body); err != nil {
		return fmt.Errorf("set current release: %w", err)
	}
	return nil
}

// PutCitations stores a citation set as an immutable file at the path of its
// ref. Identical content is a no-op; different content is ErrConflict.
func (s *Store) PutCitations(ctx context.Context, set data.CitationSet) error {
	if err := set.Ref.Validate(); err != nil {
		return fmt.Errorf("put citations: %w", err)
	}
	body, err := json.Marshal(set)
	if err != nil {
		return fmt.Errorf("put citations %s: encode: %w: %w", set.Ref, data.ErrInvalid, err)
	}
	if err := s.PutImmutable(ctx, set.Ref.ObjectPath(), body); err != nil {
		return fmt.Errorf("put citations %s: %w", set.Ref, err)
	}
	return nil
}

// GetCitations returns the set with exactly this ref.
func (s *Store) GetCitations(ctx context.Context, ref data.CitationRef) (data.CitationSet, error) {
	if err := ref.Validate(); err != nil {
		return data.CitationSet{}, fmt.Errorf("get citations: %w", err)
	}
	body, err := s.GetFile(ctx, ref.ObjectPath())
	if err != nil {
		return data.CitationSet{}, fmt.Errorf("get citations %s: %w", ref, err)
	}
	var set data.CitationSet
	if err := json.Unmarshal(body, &set); err != nil {
		return data.CitationSet{}, fmt.Errorf("get citations %s: decode: %w: %w", ref, data.ErrInvalid, err)
	}
	return set, nil
}

// PutFreshness stores or replaces the freshness flags of a citation set.
func (s *Store) PutFreshness(ctx context.Context, d data.FreshnessDoc) error {
	if err := d.Ref.Validate(); err != nil {
		return fmt.Errorf("put freshness: %w", err)
	}
	body, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("put freshness %s: encode: %w: %w", d.Ref, data.ErrInvalid, err)
	}
	if _, err := s.PutReplaceable(ctx, d.Ref.FreshnessPath(), body); err != nil {
		return fmt.Errorf("put freshness %s: %w", d.Ref, err)
	}
	return nil
}

// GetFreshness returns the current freshness flags of the set with this ref.
func (s *Store) GetFreshness(ctx context.Context, ref data.CitationRef) (data.FreshnessDoc, error) {
	if err := ref.Validate(); err != nil {
		return data.FreshnessDoc{}, fmt.Errorf("get freshness: %w", err)
	}
	body, err := s.GetFile(ctx, ref.FreshnessPath())
	if err != nil {
		return data.FreshnessDoc{}, fmt.Errorf("get freshness %s: %w", ref, err)
	}
	var d data.FreshnessDoc
	if err := json.Unmarshal(body, &d); err != nil {
		return data.FreshnessDoc{}, fmt.Errorf("get freshness %s: decode: %w: %w", ref, data.ErrInvalid, err)
	}
	return d, nil
}
