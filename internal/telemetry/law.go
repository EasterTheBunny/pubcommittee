package telemetry

import (
	"context"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// The citation and law wrappers keep a nil port nil: an adapter that stores
// none leaves the port unset and readers answer not found.

func wrapCitationReader(t timer, n data.CitationReader) data.CitationReader {
	if n == nil {
		return nil
	}
	return timedCitationReader{t, n}
}

func wrapCitationWriter(t timer, n data.CitationWriter) data.CitationWriter {
	if n == nil {
		return nil
	}
	return timedCitationWriter{t, n}
}

func wrapLawReader(t timer, n data.LawReader) data.LawReader {
	if n == nil {
		return nil
	}
	return timedLawReader{t, n}
}

func wrapLawWriter(t timer, n data.LawWriter) data.LawWriter {
	if n == nil {
		return nil
	}
	return timedLawWriter{t, n}
}

type timedCitationReader struct {
	t timer
	n data.CitationReader
}

func (w timedCitationReader) GetCitations(ctx context.Context, ref data.CitationRef) (s data.CitationSet, err error) {
	defer w.t.start(ctx, "CitationReader", "GetCitations", &err)()
	return w.n.GetCitations(ctx, ref)
}

type timedCitationWriter struct {
	t timer
	n data.CitationWriter
}

func (w timedCitationWriter) PutCitations(ctx context.Context, s data.CitationSet) (err error) {
	defer w.t.start(ctx, "CitationWriter", "PutCitations", &err)()
	return w.n.PutCitations(ctx, s)
}

type timedLawReader struct {
	t timer
	n data.LawReader
}

func (w timedLawReader) CurrentRelease(ctx context.Context) (r data.Release, err error) {
	defer w.t.start(ctx, "LawReader", "CurrentRelease", &err)()
	return w.n.CurrentRelease(ctx)
}

func (w timedLawReader) GetRelease(ctx context.Context, id string) (r data.Release, err error) {
	defer w.t.start(ctx, "LawReader", "GetRelease", &err)()
	return w.n.GetRelease(ctx, id)
}

func (w timedLawReader) GetLawSection(ctx context.Context, ref data.ProvisionRef) (s data.LawSection, err error) {
	defer w.t.start(ctx, "LawReader", "GetLawSection", &err)()
	return w.n.GetLawSection(ctx, ref)
}

type timedLawWriter struct {
	t timer
	n data.LawWriter
}

func (w timedLawWriter) PutLawSection(ctx context.Context, s data.LawSection) (err error) {
	defer w.t.start(ctx, "LawWriter", "PutLawSection", &err)()
	return w.n.PutLawSection(ctx, s)
}

func (w timedLawWriter) PutRelease(ctx context.Context, r data.Release) (err error) {
	defer w.t.start(ctx, "LawWriter", "PutRelease", &err)()
	return w.n.PutRelease(ctx, r)
}

func (w timedLawWriter) SetCurrentRelease(ctx context.Context, id string) (err error) {
	defer w.t.start(ctx, "LawWriter", "SetCurrentRelease", &err)()
	return w.n.SetCurrentRelease(ctx, id)
}
