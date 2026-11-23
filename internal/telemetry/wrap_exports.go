package telemetry

import (
	"context"
	"fmt"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// failed adds the port and operation to an error; errors.Is still sees the cause.
func failed(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", op, err)
}

// The feed and export wrappers keep a nil port nil: an adapter that stores
// none leaves the port unset and readers answer with an empty page.

func wrapFeedReader(t timer, n data.FeedReader) data.FeedReader {
	if n == nil {
		return nil
	}
	return timedFeedReader{t, n}
}

func wrapFeedWriter(t timer, n data.FeedWriter) data.FeedWriter {
	if n == nil {
		return nil
	}
	return timedFeedWriter{t, n}
}

func wrapExportReader(t timer, n data.ExportReader) data.ExportReader {
	if n == nil {
		return nil
	}
	return timedExportReader{t, n}
}

func wrapExportWriter(t timer, n data.ExportWriter) data.ExportWriter {
	if n == nil {
		return nil
	}
	return timedExportWriter{t, n}
}

type timedFeedReader struct {
	t timer
	n data.FeedReader
}

func (w timedFeedReader) ListFeed(
	ctx context.Context, since time.Time, p data.Page,
) (r data.Paged[data.FeedEntry], err error) {
	defer w.t.start(ctx, "FeedReader", "ListFeed", &err)()
	r, err = w.n.ListFeed(ctx, since, p)
	return r, failed("feed list", err)
}

type timedFeedWriter struct {
	t timer
	n data.FeedWriter
}

func (w timedFeedWriter) AppendFeed(ctx context.Context, entries []data.FeedEntry) (added int, err error) {
	defer w.t.start(ctx, "FeedWriter", "AppendFeed", &err)()
	added, err = w.n.AppendFeed(ctx, entries)
	return added, failed("feed append", err)
}

type timedExportReader struct {
	t timer
	n data.ExportReader
}

func (w timedExportReader) ListExports(ctx context.Context, p data.Page) (r data.Paged[data.ExportEntry], err error) {
	defer w.t.start(ctx, "ExportReader", "ListExports", &err)()
	r, err = w.n.ListExports(ctx, p)
	return r, failed("exports list", err)
}

func (w timedExportReader) GetExport(ctx context.Context, id string) (e data.ExportEntry, body []byte, err error) {
	defer w.t.start(ctx, "ExportReader", "GetExport", &err)()
	e, body, err = w.n.GetExport(ctx, id)
	return e, body, failed("export get", err)
}

type timedExportWriter struct {
	t timer
	n data.ExportWriter
}

func (w timedExportWriter) PutExport(
	ctx context.Context, e data.ExportEntry, body []byte, replace bool,
) (r data.ExportEntry, err error) {
	defer w.t.start(ctx, "ExportWriter", "PutExport", &err)()
	r, err = w.n.PutExport(ctx, e, body, replace)
	return r, failed("export put", err)
}

func (w timedExportWriter) PutExportIndex(ctx context.Context, entries []data.ExportEntry) (err error) {
	defer w.t.start(ctx, "ExportWriter", "PutExportIndex", &err)()
	return failed("export index put", w.n.PutExportIndex(ctx, entries))
}
