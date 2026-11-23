package fsobject

import (
	"context"
	"fmt"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// Feed and export ports are the shared publisher-backed implementations of
// package data over this store's files.

func wrapErr(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", op, err)
}

// ListFeed implements data.FeedReader.
func (s *Store) ListFeed(ctx context.Context, since time.Time, p data.Page) (data.Paged[data.FeedEntry], error) {
	page, err := data.ListFeedFile(ctx, s, since, p)
	return page, wrapErr("list feed", err)
}

// AppendFeed implements data.FeedWriter.
func (s *Store) AppendFeed(ctx context.Context, entries []data.FeedEntry) (int, error) {
	n, err := data.AppendFeedFile(ctx, s, entries)
	return n, wrapErr("append feed", err)
}

// ListExports implements data.ExportReader.
func (s *Store) ListExports(ctx context.Context, p data.Page) (data.Paged[data.ExportEntry], error) {
	page, err := data.ListExportFiles(ctx, s, p)
	return page, wrapErr("list exports", err)
}

// GetExport implements data.ExportReader.
func (s *Store) GetExport(ctx context.Context, id string) (data.ExportEntry, []byte, error) {
	e, body, err := data.GetExportFile(ctx, s, id)
	return e, body, wrapErr("get export", err)
}

// PutExport implements data.ExportWriter.
func (s *Store) PutExport(
	ctx context.Context, e data.ExportEntry, body []byte, replace bool,
) (data.ExportEntry, error) {
	got, err := data.PutExportFile(ctx, s, e, body, replace)
	return got, wrapErr("put export", err)
}

// PutExportIndex implements data.ExportWriter.
func (s *Store) PutExportIndex(ctx context.Context, entries []data.ExportEntry) error {
	return wrapErr("put export index", data.PutExportIndexFile(ctx, s, entries))
}
