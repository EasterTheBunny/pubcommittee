package telemetry

import (
	"context"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// timer records one port operation on return. Use:
//
//	defer w.t.done(ctx, "BillText", "GetVersion", &err)()
type timer struct {
	ins     *Instruments
	adapter string
}

func (t timer) start(ctx context.Context, port, op string, err *error) func() {
	begin := time.Now()
	return func() { t.ins.RecordData(ctx, port, op, t.adapter, data.Outcome(*err), time.Since(begin)) }
}

// WrapPlatform wraps every port of g so each operation records
// pubcommittee.data.operation.duration with the adapter name (the adapter
// package, for example "memory"). Adapters do not instrument themselves.
func (t *Telemetry) WrapPlatform(adapter string, g data.PlatformPorts) data.PlatformPorts {
	tm := timer{ins: t.Instruments, adapter: adapter}
	return data.PlatformPorts{
		PublicReadPorts: t.WrapPublicRead(adapter, g.PublicReadPorts),
		PlatformDocs:    timedPlatformDocs{tm, g.PlatformDocs},
		Moderation:      timedModeration{tm, g.Moderation},
		Publisher:       timedPublisher{tm, g.Publisher},
		ComparisonStore: wrapComparisonWriter(tm, g.ComparisonStore),
		CitationStore:   wrapCitationWriter(tm, g.CitationStore),
		LawStore:        wrapLawWriter(tm, g.LawStore),
		GeneratedStore:  wrapGeneratedWriter(tm, g.GeneratedStore),
	}
}

// WrapPublicRead wraps the public read group.
func (t *Telemetry) WrapPublicRead(adapter string, g data.PublicReadPorts) data.PublicReadPorts {
	tm := timer{ins: t.Instruments, adapter: adapter}
	return data.PublicReadPorts{
		BillText:    timedBillText{tm, g.BillText},
		PublicDocs:  timedPublicDocs{tm, g.PublicDocs},
		Comparisons: wrapComparisonReader(tm, g.Comparisons),
		Citations:   wrapCitationReader(tm, g.Citations),
		Law:         wrapLawReader(tm, g.Law),
		Generated:   wrapGeneratedReader(tm, g.Generated),
	}
}

// WrapBillSource wraps a BillSource.
func (t *Telemetry) WrapBillSource(adapter string, s data.BillSource) data.BillSource {
	return timedBillSource{timer{ins: t.Instruments, adapter: adapter}, s}
}

// WrapLawSource wraps a LawSource.
func (t *Telemetry) WrapLawSource(adapter string, s data.LawSource) data.LawSource {
	return timedLawSource{timer{ins: t.Instruments, adapter: adapter}, s}
}

// WrapGenerator wraps a Generator.
func (t *Telemetry) WrapGenerator(adapter string, g data.Generator) data.Generator {
	return timedGenerator{timer{ins: t.Instruments, adapter: adapter}, g}
}

type timedBillText struct {
	t timer
	n data.BillText
}

func (w timedBillText) ListVersions(ctx context.Context, b data.BillRef) (r []data.VersionSummary, err error) {
	defer w.t.start(ctx, "BillText", "ListVersions", &err)()
	return w.n.ListVersions(ctx, b)
}

func (w timedBillText) GetVersion(ctx context.Context, ref data.VersionRef) (r data.OfficialVersion, err error) {
	defer w.t.start(ctx, "BillText", "GetVersion", &err)()
	return w.n.GetVersion(ctx, ref)
}

func (w timedBillText) GetLines(ctx context.Context, a data.Anchor) (r []data.Line, err error) {
	defer w.t.start(ctx, "BillText", "GetLines", &err)()
	return w.n.GetLines(ctx, a)
}

func (w timedBillText) GetSource(ctx context.Context, ref data.VersionRef) (r []byte, err error) {
	defer w.t.start(ctx, "BillText", "GetSource", &err)()
	return w.n.GetSource(ctx, ref)
}

type timedPublicDocs struct {
	t timer
	n data.PublicDocs
}

func (w timedPublicDocs) GetBill(ctx context.Context, b data.BillRef) (r data.Bill, err error) {
	defer w.t.start(ctx, "PublicDocs", "GetBill", &err)()
	return w.n.GetBill(ctx, b)
}

func (w timedPublicDocs) ListCatalog(ctx context.Context, p data.Page) (r data.Paged[data.CatalogEntry], err error) {
	defer w.t.start(ctx, "PublicDocs", "ListCatalog", &err)()
	return w.n.ListCatalog(ctx, p)
}

func (w timedPublicDocs) GetExplanations(
	ctx context.Context, ref data.VersionRef,
) (r []data.ChangeExplanation, err error) {
	defer w.t.start(ctx, "PublicDocs", "GetExplanations", &err)()
	return w.n.GetExplanations(ctx, ref)
}

func (w timedPublicDocs) GetSelectionPolicy(ctx context.Context) (r data.SelectionPolicy, err error) {
	defer w.t.start(ctx, "PublicDocs", "GetSelectionPolicy", &err)()
	return w.n.GetSelectionPolicy(ctx)
}

func (w timedPublicDocs) GetSelectionPolicyVersion(
	ctx context.Context, version int,
) (r data.SelectionPolicy, err error) {
	defer w.t.start(ctx, "PublicDocs", "GetSelectionPolicyVersion", &err)()
	return w.n.GetSelectionPolicyVersion(ctx, version)
}

func (w timedPublicDocs) ListLog(ctx context.Context, p data.Page) (r data.Paged[data.LogEntry], err error) {
	defer w.t.start(ctx, "PublicDocs", "ListLog", &err)()
	return w.n.ListLog(ctx, p)
}

type timedModeration struct {
	t timer
	n data.Moderation
}

func (w timedModeration) Record(ctx context.Context, e data.LogEntry) (err error) {
	defer w.t.start(ctx, "Moderation", "Record", &err)()
	return w.n.Record(ctx, e)
}

type timedPlatformDocs struct {
	t timer
	n data.PlatformDocs
}

func (w timedPlatformDocs) PutBill(ctx context.Context, b data.Bill) (err error) {
	defer w.t.start(ctx, "PlatformDocs", "PutBill", &err)()
	return w.n.PutBill(ctx, b)
}

func (w timedPlatformDocs) PutCatalogEntry(ctx context.Context, e data.CatalogEntry) (err error) {
	defer w.t.start(ctx, "PlatformDocs", "PutCatalogEntry", &err)()
	return w.n.PutCatalogEntry(ctx, e)
}

func (w timedPlatformDocs) PutSelectionPolicy(ctx context.Context, p data.SelectionPolicy) (err error) {
	defer w.t.start(ctx, "PlatformDocs", "PutSelectionPolicy", &err)()
	return w.n.PutSelectionPolicy(ctx, p)
}

func (w timedPlatformDocs) PutExplanation(ctx context.Context, e data.ChangeExplanation) (err error) {
	defer w.t.start(ctx, "PlatformDocs", "PutExplanation", &err)()
	return w.n.PutExplanation(ctx, e)
}

func (w timedPlatformDocs) PutGeneration(ctx context.Context, g data.GenerationRecord) (err error) {
	defer w.t.start(ctx, "PlatformDocs", "PutGeneration", &err)()
	return w.n.PutGeneration(ctx, g)
}

func (w timedPlatformDocs) GetIngestState(ctx context.Context, source string) (r data.IngestState, err error) {
	defer w.t.start(ctx, "PlatformDocs", "GetIngestState", &err)()
	return w.n.GetIngestState(ctx, source)
}

func (w timedPlatformDocs) PutIngestState(ctx context.Context, s data.IngestState) (err error) {
	defer w.t.start(ctx, "PlatformDocs", "PutIngestState", &err)()
	return w.n.PutIngestState(ctx, s)
}

func (w timedPlatformDocs) ApplyLogged(ctx context.Context, c data.Change, e data.LogEntry) (err error) {
	defer w.t.start(ctx, "PlatformDocs", "ApplyLogged", &err)()
	return w.n.ApplyLogged(ctx, c, e)
}

type timedPublisher struct {
	t timer
	n data.Publisher
}

func (w timedPublisher) PutVersion(ctx context.Context, v data.OfficialVersion, xml []byte) (err error) {
	defer w.t.start(ctx, "Publisher", "PutVersion", &err)()
	return w.n.PutVersion(ctx, v, xml)
}

func (w timedPublisher) PutImmutable(ctx context.Context, path string, b []byte) (err error) {
	defer w.t.start(ctx, "Publisher", "PutImmutable", &err)()
	return w.n.PutImmutable(ctx, path, b)
}

func (w timedPublisher) PutReplaceable(ctx context.Context, path string, b []byte) (etag string, err error) {
	defer w.t.start(ctx, "Publisher", "PutReplaceable", &err)()
	return w.n.PutReplaceable(ctx, path, b)
}

func (w timedPublisher) GetFile(ctx context.Context, path string) (r []byte, err error) {
	defer w.t.start(ctx, "Publisher", "GetFile", &err)()
	return w.n.GetFile(ctx, path)
}

type timedBillSource struct {
	t timer
	n data.BillSource
}

func (w timedBillSource) ListChanged(
	ctx context.Context, since string, p data.Page,
) (r data.Paged[data.ChangedBill], err error) {
	defer w.t.start(ctx, "BillSource", "ListChanged", &err)()
	return w.n.ListChanged(ctx, since, p)
}

func (w timedBillSource) FetchStatus(ctx context.Context, b data.BillRef) (r data.BillStatus, err error) {
	defer w.t.start(ctx, "BillSource", "FetchStatus", &err)()
	return w.n.FetchStatus(ctx, b)
}

func (w timedBillSource) FetchVersionXML(ctx context.Context, ref data.VersionRef) (r []byte, err error) {
	defer w.t.start(ctx, "BillSource", "FetchVersionXML", &err)()
	return w.n.FetchVersionXML(ctx, ref)
}

type timedLawSource struct {
	t timer
	n data.LawSource
}

func (w timedLawSource) ListReleasePoints(ctx context.Context) (r []data.ReleasePoint, err error) {
	defer w.t.start(ctx, "LawSource", "ListReleasePoints", &err)()
	return w.n.ListReleasePoints(ctx)
}

func (w timedLawSource) FetchTitle(ctx context.Context, rp data.ReleasePoint, title int) (r []byte, err error) {
	defer w.t.start(ctx, "LawSource", "FetchTitle", &err)()
	return w.n.FetchTitle(ctx, rp, title)
}

type timedGenerator struct {
	t timer
	n data.Generator
}

func (w timedGenerator) Submit(ctx context.Context, reqs []data.GenRequest) (id string, err error) {
	defer w.t.start(ctx, "Generator", "Submit", &err)()
	return w.n.Submit(ctx, reqs)
}

func (w timedGenerator) Collect(ctx context.Context, id string) (r []data.GenResult, done bool, err error) {
	defer w.t.start(ctx, "Generator", "Collect", &err)()
	return w.n.Collect(ctx, id)
}

func (w timedGenerator) Generate(ctx context.Context, req data.GenRequest) (r data.GenResult, err error) {
	defer w.t.start(ctx, "Generator", "Generate", &err)()
	return w.n.Generate(ctx, req)
}

// wrapComparisonReader keeps a nil reader nil: an adapter that stores no
// comparisons leaves the port unset and readers answer not found.
func wrapComparisonReader(t timer, n data.ComparisonReader) data.ComparisonReader {
	if n == nil {
		return nil
	}
	return timedComparisonReader{t, n}
}

func wrapComparisonWriter(t timer, n data.ComparisonWriter) data.ComparisonWriter {
	if n == nil {
		return nil
	}
	return timedComparisonWriter{t, n}
}

type timedComparisonReader struct {
	t timer
	n data.ComparisonReader
}

func (w timedComparisonReader) GetComparison(
	ctx context.Context, ref data.ComparisonRef,
) (c data.Comparison, err error) {
	defer w.t.start(ctx, "ComparisonReader", "GetComparison", &err)()
	return w.n.GetComparison(ctx, ref)
}

type timedComparisonWriter struct {
	t timer
	n data.ComparisonWriter
}

func (w timedComparisonWriter) PutComparison(ctx context.Context, c data.Comparison) (err error) {
	defer w.t.start(ctx, "ComparisonWriter", "PutComparison", &err)()
	return w.n.PutComparison(ctx, c)
}

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
