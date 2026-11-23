// Package fixturesrc implements data.BillFetcher on the example data in
// fixtures/: bills/<bill-id>/status.xml with bills/<bill-id>/v/<code>/source.xml
// for the bills that carry text, and catalog/<bill-id>.xml for status-only
// catalog records. It makes no network calls, so every environment can run an
// ingest offline. A bill the fixtures hold only as a catalog record lists its
// official text versions but has no text to fetch.
package fixturesrc

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/billstatus"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

// Config configures a Source.
type Config struct {
	// FS holds the fixtures directory: bills/ and catalog/ at its root.
	FS fs.FS
	// Now stamps RetrievedAt; time.Now when nil.
	Now func() time.Time
}

// Source is the fixture-backed BillFetcher.
type Source struct {
	fsys fs.FS
	now  func() time.Time
}

var _ data.BillFetcher = (*Source)(nil)

// New validates cfg. It reads nothing.
func New(cfg Config) (*Source, error) {
	if cfg.FS == nil {
		return nil, fmt.Errorf("fixturesrc: FS is required: %w", data.ErrInvalid)
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Source{fsys: cfg.FS, now: now}, nil
}

// FetchRecord returns the status record of a bill, from its status.xml or its
// catalog record; ErrNotFound when the fixtures hold neither.
func (s *Source) FetchRecord(ctx context.Context, bill data.BillRef) (data.BillRecord, error) {
	if err := ctx.Err(); err != nil {
		return data.BillRecord{}, fmt.Errorf("fixturesrc: fetch record: %w", err)
	}
	if err := bill.Validate(); err != nil {
		return data.BillRecord{}, fmt.Errorf("fixturesrc: fetch record: %w", err)
	}
	for _, p := range []string{
		path.Join("bills", bill.ID(), "status.xml"),
		path.Join("catalog", bill.ID()+".xml"),
	} {
		b, err := fs.ReadFile(s.fsys, p)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return data.BillRecord{}, fmt.Errorf("fixturesrc: read %s: %w: %w", p, data.ErrUnavailable, err)
		}
		rec, err := billstatus.Parse(b, data.SourceInfo{URL: "fixture:" + p, RetrievedAt: s.now().UTC()})
		if err != nil {
			return data.BillRecord{}, fmt.Errorf("fixturesrc: %s: %w", p, err)
		}
		return rec, nil
	}
	return data.BillRecord{}, fmt.Errorf("fixturesrc: bill %s: %w", bill.ID(), data.ErrNotFound)
}

// FetchOriginal returns the fixture bytes of one text version, unchanged;
// ErrNotFound when the fixtures carry no text for it. The recorded URL is the
// official one the status lists, because the bytes are that document's.
func (s *Source) FetchOriginal(ctx context.Context, v data.SourceVersion) (data.SourceDocument, error) {
	if err := ctx.Err(); err != nil {
		return data.SourceDocument{}, fmt.Errorf("fixturesrc: fetch original: %w", err)
	}
	if err := v.Ref.Validate(); err != nil {
		return data.SourceDocument{}, fmt.Errorf("fixturesrc: fetch original: %w", err)
	}
	p := path.Join("bills", v.Ref.Bill.ID(), "v", v.Ref.Version, "source.xml")
	b, err := fs.ReadFile(s.fsys, p)
	if errors.Is(err, fs.ErrNotExist) {
		return data.SourceDocument{}, fmt.Errorf("fixturesrc: version %s: %w", v.Ref, data.ErrNotFound)
	}
	if err != nil {
		return data.SourceDocument{}, fmt.Errorf("fixturesrc: read %s: %w: %w", p, data.ErrUnavailable, err)
	}
	url := v.URL
	if url == "" {
		url = "fixture:" + path.Clean(p)
	}
	return data.SourceDocument{XML: b, Info: data.SourceInfo{URL: strings.TrimSpace(url), RetrievedAt: s.now().UTC()}}, nil
}
