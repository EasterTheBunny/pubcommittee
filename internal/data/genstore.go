package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

// FileStore is the part of Publisher the generation store needs. Every
// object-store adapter satisfies it.
type FileStore interface {
	PutImmutable(ctx context.Context, path string, data []byte) error
	PutReplaceable(ctx context.Context, path string, data []byte) (etag string, err error)
	GetFile(ctx context.Context, path string) ([]byte, error)
}

const (
	pendingPath = "generated/batches/pending.json"
	batchDir    = "generated/batches/"
)

// GeneratedStore implements GeneratedReader and GeneratedWriter over a file
// store, so one implementation serves every adapter that stores files. Outputs
// and grades live at generated/{task}/{bill}/v/{version}/{key}.json.
type GeneratedStore struct{ files FileStore }

var (
	_ GeneratedReader = (*GeneratedStore)(nil)
	_ GeneratedWriter = (*GeneratedStore)(nil)
)

// NewGeneratedStore returns a store over files.
func NewGeneratedStore(files FileStore) *GeneratedStore { return &GeneratedStore{files: files} }

type currentPointer struct {
	Key GenKey `json:"key"`
}

// PutGenerated stores the output at the write-once path of its key.
func (s *GeneratedStore) PutGenerated(ctx context.Context, o GeneratedOutput) error {
	if err := o.Key.Validate(); err != nil {
		return fmt.Errorf("put generated: %w", err)
	}
	body, err := json.Marshal(o)
	if err != nil {
		return fmt.Errorf("put generated %s: encode: %w: %w", o.Key, ErrInvalid, err)
	}
	if err := s.files.PutImmutable(ctx, o.Key.ObjectPath(), body); err != nil {
		return fmt.Errorf("put generated %s: %w", o.Key, err)
	}
	return nil
}

// PutRevised stores the revised record of an output beside its write-once record.
func (s *GeneratedStore) PutRevised(ctx context.Context, o GeneratedOutput) error {
	if err := o.Key.Validate(); err != nil {
		return fmt.Errorf("put revised: %w", err)
	}
	if _, err := s.files.GetFile(ctx, o.Key.ObjectPath()); err != nil {
		return fmt.Errorf("put revised %s: write-once record: %w", o.Key, err)
	}
	body, err := json.Marshal(o)
	if err != nil {
		return fmt.Errorf("put revised %s: encode: %w: %w", o.Key, ErrInvalid, err)
	}
	if _, err := s.files.PutReplaceable(ctx, o.Key.RevisedPath(), body); err != nil {
		return fmt.Errorf("put revised %s: %w", o.Key, err)
	}
	return nil
}

// GetGenerated returns the output stored under exactly this key: the revised record when there is one.
func (s *GeneratedStore) GetGenerated(ctx context.Context, key GenKey) (GeneratedOutput, error) {
	var o GeneratedOutput
	if err := key.Validate(); err != nil {
		return o, fmt.Errorf("get generated: %w", err)
	}
	err := s.read(ctx, key.RevisedPath(), &o)
	if errors.Is(err, ErrNotFound) {
		err = s.read(ctx, key.ObjectPath(), &o)
	}
	if err != nil {
		return o, fmt.Errorf("get generated %s: %w", key, err)
	}
	return o, nil
}

// SetCurrent points readers of the output's target at the output.
func (s *GeneratedStore) SetCurrent(ctx context.Context, o GeneratedOutput) error {
	if err := o.Key.Validate(); err != nil {
		return fmt.Errorf("set current: %w", err)
	}
	if o.Status != GenReady {
		return fmt.Errorf("set current %s: status %q is not published: %w", o.Key, o.Status, ErrInvalid)
	}
	body, err := json.Marshal(currentPointer{Key: o.Key})
	if err != nil {
		return fmt.Errorf("set current %s: encode: %w: %w", o.Key, ErrInvalid, err)
	}
	if _, err := s.files.PutReplaceable(ctx, currentPath(o.Key.Task, o.Key.Version, o.Key.Target), body); err != nil {
		return fmt.Errorf("set current %s: %w", o.Key, err)
	}
	return nil
}

// GetCurrent returns the published output for a target of a version.
func (s *GeneratedStore) GetCurrent(
	ctx context.Context, task string, v VersionRef, target string,
) (GeneratedOutput, error) {
	if err := v.Validate(); err != nil {
		return GeneratedOutput{}, fmt.Errorf("get current: %w", err)
	}
	var p currentPointer
	if err := s.read(ctx, currentPath(task, v, target), &p); err != nil {
		return GeneratedOutput{}, fmt.Errorf("get current %s %s: %w", task, v, err)
	}
	return s.GetGenerated(ctx, p.Key)
}

// PutGrade stores or replaces the grade of an output.
func (s *GeneratedStore) PutGrade(ctx context.Context, g Grade) error {
	if err := g.Key.Validate(); err != nil {
		return fmt.Errorf("put grade: %w", err)
	}
	body, err := json.Marshal(g)
	if err != nil {
		return fmt.Errorf("put grade %s: encode: %w: %w", g.Key, ErrInvalid, err)
	}
	if _, err := s.files.PutReplaceable(ctx, g.Key.GradePath(), body); err != nil {
		return fmt.Errorf("put grade %s: %w", g.Key, err)
	}
	return nil
}

// GetGrade returns the grade of the output with this key.
func (s *GeneratedStore) GetGrade(ctx context.Context, key GenKey) (Grade, error) {
	var g Grade
	if err := key.Validate(); err != nil {
		return g, fmt.Errorf("get grade: %w", err)
	}
	if err := s.read(ctx, key.GradePath(), &g); err != nil {
		return g, fmt.Errorf("get grade %s: %w", key, err)
	}
	return g, nil
}

// SaveBatch stores the manifest and updates the pending list.
func (s *GeneratedStore) SaveBatch(ctx context.Context, b GenBatch) error {
	if b.ID == "" {
		return fmt.Errorf("save batch: empty ID: %w", ErrInvalid)
	}
	body, err := json.Marshal(b)
	if err != nil {
		return fmt.Errorf("save batch %s: encode: %w: %w", b.ID, ErrInvalid, err)
	}
	if _, err := s.files.PutReplaceable(ctx, batchDir+b.ID+".json", body); err != nil {
		return fmt.Errorf("save batch %s: %w", b.ID, err)
	}
	return s.updatePending(ctx, b)
}

// updatePending adds the batch to the pending list, or removes it once Done.
func (s *GeneratedStore) updatePending(ctx context.Context, b GenBatch) error {
	ids, err := s.pendingIDs(ctx)
	if err != nil {
		return err
	}
	has := slices.Contains(ids, b.ID)
	switch {
	case b.Done && has:
		ids = slices.DeleteFunc(ids, func(id string) bool { return id == b.ID })
	case !b.Done && !has:
		ids = append(ids, b.ID)
	default:
		return nil
	}
	list, err := json.Marshal(ids)
	if err != nil {
		return fmt.Errorf("save batch %s: encode pending: %w: %w", b.ID, ErrInvalid, err)
	}
	if _, err := s.files.PutReplaceable(ctx, pendingPath, list); err != nil {
		return fmt.Errorf("save batch %s: pending: %w", b.ID, err)
	}
	return nil
}

// PendingBatches returns the batches not yet collected, oldest first.
func (s *GeneratedStore) PendingBatches(ctx context.Context) ([]GenBatch, error) {
	ids, err := s.pendingIDs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]GenBatch, 0, len(ids))
	for _, id := range ids {
		var b GenBatch
		if err := s.read(ctx, batchDir+id+".json", &b); err != nil {
			return nil, fmt.Errorf("pending batch %s: %w", id, err)
		}
		out = append(out, b)
	}
	return out, nil
}

func (s *GeneratedStore) pendingIDs(ctx context.Context) ([]string, error) {
	var ids []string
	err := s.read(ctx, pendingPath, &ids)
	if err != nil && !isNotFound(err) {
		return nil, fmt.Errorf("pending batches: %w", err)
	}
	return ids, nil
}

func (s *GeneratedStore) read(ctx context.Context, path string, into any) error {
	body, err := s.files.GetFile(ctx, path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(body, into); err != nil {
		return fmt.Errorf("decode %s: %w: %w", path, ErrInvalid, err)
	}
	return nil
}

func isNotFound(err error) bool { return errors.Is(err, ErrNotFound) }
