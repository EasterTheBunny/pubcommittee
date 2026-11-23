package fsobject

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// PutComparison stores a comparison as an immutable file at the path of its
// ref. Identical content is a no-op; different content is ErrConflict.
func (s *Store) PutComparison(ctx context.Context, c data.Comparison) error {
	if err := c.Ref.Validate(); err != nil {
		return fmt.Errorf("put comparison: %w", err)
	}
	body, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("put comparison %s: encode: %w: %w", c.Ref, data.ErrInvalid, err)
	}
	if err := s.PutImmutable(ctx, c.Ref.ObjectPath(), body); err != nil {
		return fmt.Errorf("put comparison %s: %w", c.Ref, err)
	}
	return nil
}

// GetComparison returns the comparison with exactly this ref.
func (s *Store) GetComparison(ctx context.Context, ref data.ComparisonRef) (data.Comparison, error) {
	if err := ref.Validate(); err != nil {
		return data.Comparison{}, fmt.Errorf("get comparison: %w", err)
	}
	body, err := s.GetFile(ctx, ref.ObjectPath())
	if err != nil {
		return data.Comparison{}, fmt.Errorf("get comparison %s: %w", ref, err)
	}
	var c data.Comparison
	if err := json.Unmarshal(body, &c); err != nil {
		return data.Comparison{}, fmt.Errorf("get comparison %s: decode: %w: %w", ref, data.ErrInvalid, err)
	}
	return c, nil
}
