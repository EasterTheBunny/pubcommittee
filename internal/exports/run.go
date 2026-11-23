package exports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/billxml"
	"github.com/easterthebunny/pubcommittee/internal/citations"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/selection"
)

// Deps is what a run needs.
type Deps struct {
	Ports data.PlatformPorts
	Log   *slog.Logger
	Now   func() time.Time
}

// Options of a run.
type Options struct {
	// DryRun builds everything and reports it, and writes nothing.
	DryRun bool
}

// Report says what a run did. Written lists every export file the run built;
// with DryRun nothing was written.
type Report struct {
	DryRun    bool     `json:"dry_run"`
	Bills     int      `json:"tracked_bills"`
	Versions  int      `json:"versions"`
	Written   []string `json:"exports"`
	FeedNew   int      `json:"feed_new"`
	FeedTotal int      `json:"feed_candidates"`
	Skipped   []string `json:"skipped,omitempty"`
}

var errNoPorts = errors.New("exports: feed and export stores are required")

type run struct {
	Deps

	opts    Options
	rep     Report
	entries []data.ExportEntry
	events  []data.FeedEntry
}

// Run builds every export and the feed from the stored data, then writes the
// index. It is idempotent per window: unchanged data gives identical files
// and no new feed entries. Two overlapping runs write the same bytes.
func Run(ctx context.Context, d Deps, opts Options) (Report, error) {
	if d.Ports.ExportStore == nil || d.Ports.FeedStore == nil {
		return Report{}, errNoPorts
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	r := &run{Deps: d, opts: opts, rep: Report{DryRun: opts.DryRun}}
	if err := r.build(ctx); err != nil {
		return r.rep, err
	}
	if err := r.otherEvents(ctx); err != nil {
		return r.rep, err
	}
	if err := r.finish(ctx); err != nil {
		return r.rep, err
	}
	return r.rep, nil
}

// build exports every tracked bill, the catalog and the tracked set.
func (r *run) build(ctx context.Context) error {
	catalog, err := r.listCatalog(ctx)
	if err != nil {
		return err
	}
	for _, e := range catalog {
		if !e.Tracked {
			continue
		}
		if err := r.trackedBill(ctx, e); err != nil {
			return err
		}
	}
	if err := r.catalogExport(ctx, catalog); err != nil {
		return err
	}
	return r.trackedSetExport(ctx)
}

// otherEvents records the policy and log events.
func (r *run) otherEvents(ctx context.Context) error {
	if err := r.policyEvents(ctx); err != nil {
		return err
	}
	return r.logEvents(ctx)
}

func (r *run) finish(ctx context.Context) error {
	r.rep.FeedTotal = len(r.events)
	for _, e := range r.entries {
		r.rep.Written = append(r.rep.Written, e.ID)
	}
	if r.opts.DryRun {
		return nil
	}
	if err := r.Ports.ExportStore.PutExportIndex(ctx, r.entries); err != nil {
		return fmt.Errorf("export index: %w", err)
	}
	added, err := r.Ports.FeedStore.AppendFeed(ctx, r.events)
	if err != nil {
		return fmt.Errorf("feed: %w", err)
	}
	r.rep.FeedNew = added
	return nil
}

func (r *run) listCatalog(ctx context.Context) ([]data.CatalogEntry, error) {
	var out []data.CatalogEntry
	cursor := ""
	for {
		pg, err := r.Ports.PublicDocs.ListCatalog(ctx, data.Page{Cursor: cursor})
		if err != nil {
			return nil, fmt.Errorf("list catalog: %w", err)
		}
		out = append(out, pg.Items...)
		if pg.Next == "" {
			return out, nil
		}
		cursor = pg.Next
	}
}

// put stores one export and remembers its index entry. With DryRun the entry
// is built from the body and nothing is stored.
func (r *run) put(ctx context.Context, e data.ExportEntry, v any, replace bool) error {
	body, err := encode(v)
	if err != nil {
		return err
	}
	return r.putBytes(ctx, e, body, replace)
}

func (r *run) putBytes(ctx context.Context, e data.ExportEntry, body []byte, replace bool) error {
	if r.opts.DryRun {
		r.entries = append(r.entries, e)
		return nil
	}
	got, err := r.Ports.ExportStore.PutExport(ctx, e, body, replace)
	if err != nil {
		return fmt.Errorf("export %s: %w", e.ID, err)
	}
	r.entries = append(r.entries, got)
	return nil
}

func entryOf(id, kind, title, contentType, licence string, asOf time.Time, class data.ContentClass) data.ExportEntry {
	prov := data.ProvenanceSource
	if class == data.ContentPlatform {
		prov = data.ProvenanceAuto
	}
	return data.ExportEntry{
		ID: id, Kind: kind, Title: title, ContentType: contentType, Licence: licence,
		Meta: data.Meta{Version: 1, AsOf: asOf.UTC(), Provenance: prov, ContentClass: class},
	}
}

func (r *run) skipf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	r.rep.Skipped = append(r.rep.Skipped, msg)
	r.Log.Warn("export skipped", "reason", msg)
}

const jsonType = "application/json"

// trackedBill exports every stored version of one tracked bill and records
// its feed events.
func (r *run) trackedBill(ctx context.Context, c data.CatalogEntry) error {
	r.rep.Bills++
	vs, err := r.Ports.BillText.ListVersions(ctx, c.Bill)
	if err != nil {
		if errors.Is(err, data.ErrNotFound) {
			r.skipf("%s: no stored versions", c.Bill.ID())
			return nil
		}
		return fmt.Errorf("list versions %s: %w", c.Bill.ID(), err)
	}
	for i, vs := range vs {
		if err := r.version(ctx, i+1, vs.Ref); err != nil {
			return err
		}
	}
	return nil
}

func (r *run) version(ctx context.Context, n int, ref data.VersionRef) error {
	r.rep.Versions++
	v, err := r.Ports.BillText.GetVersion(ctx, ref)
	if err != nil {
		return fmt.Errorf("get version %s: %w", ref, err)
	}
	src, err := r.Ports.BillText.GetSource(ctx, ref)
	if err != nil {
		return fmt.Errorf("get source %s: %w", ref, err)
	}
	title := ref.Bill.ID() + " " + ref.Version
	srcID, textID := billSourceID(ref), billTextID(ref)
	if err := r.putBytes(ctx, entryOf(srcID, data.ExportBillText, "Original document of "+title,
		"application/xml", data.LicencePublicDomain, v.Meta.AsOf, data.ContentOfficial), src, false); err != nil {
		return err
	}
	if err := r.put(ctx, entryOf(textID, data.ExportBillText, "Bill text of "+title, jsonType,
		data.LicencePublicDomain, v.Meta.AsOf, data.ContentOfficial), billTextFile(textID, srcID, n, v), false); err != nil {
		return err
	}
	if err := r.lawExport(ctx, ref, title, v.Meta.AsOf); err != nil {
		return err
	}
	if err := r.explanationExport(ctx, n, ref, title, v.Meta.AsOf); err != nil {
		return err
	}
	r.versionEvent(n, ref)
	return nil
}

// lawExport exports the cited provisions of a version under the current
// release. A version with no stored citation set has no such export.
func (r *run) lawExport(ctx context.Context, ref data.VersionRef, title string, _ time.Time) error {
	if r.Ports.Citations == nil || r.Ports.Law == nil {
		return nil
	}
	rel, err := r.Ports.Law.CurrentRelease(ctx)
	if errors.Is(err, data.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("current release: %w", err)
	}
	set, err := r.Ports.Citations.GetCitations(ctx, data.CitationRef{
		Version: ref, Method: citations.Method(billxml.ParserVersion, rel.ID),
	})
	if errors.Is(err, data.ErrNotFound) {
		r.skipf("%s: no citation set under release %s", ref, rel.ID)
		return nil
	}
	if err != nil {
		return fmt.Errorf("get citations %s: %w", ref, err)
	}
	id := existingLawID(ref)
	return r.put(ctx, entryOf(id, data.ExportExistingLaw, "Cited existing law of "+title, jsonType,
		data.LicencePublicDomain, set.Meta.AsOf, data.ContentOfficial), existingLawFile(id, ref, set), true)
}

func (r *run) explanationExport(
	ctx context.Context, n int, ref data.VersionRef, title string, fallback time.Time,
) error {
	xs, err := r.Ports.PublicDocs.GetExplanations(ctx, ref)
	if err != nil && !errors.Is(err, data.ErrNotFound) {
		return fmt.Errorf("get explanations %s: %w", ref, err)
	}
	id := explanationsID(ref)
	f := explanationsFile(id, ref, fallback, xs)
	if err := r.put(ctx, entryOf(id, data.ExportChangeExplanations, "Change explanations of "+title, jsonType,
		data.LicenceCC0, f.AsOf, data.ContentPlatform), f, true); err != nil {
		return err
	}
	for _, x := range xs {
		r.explanationEvent(n, ref, x)
	}
	return nil
}

func (r *run) catalogExport(ctx context.Context, catalog []data.CatalogEntry) error {
	ts := make([]time.Time, len(catalog))
	for i, e := range catalog {
		ts[i] = e.Meta.AsOf
		r.catalogEvents(e)
	}
	if len(catalog) == 0 {
		r.skipf("catalog: no entries, no catalog export")
		return nil
	}
	asOf := latestAsOf(r.Now(), ts)
	f := CatalogFile{Header: newHeader(catalogID, data.ExportOther, asOf, data.ContentPlatform), Entries: catalog}
	return r.put(ctx, entryOf(catalogID, data.ExportOther, "Catalog of all bills", jsonType,
		data.LicenceCC0, asOf, data.ContentPlatform), f, true)
}

// trackedSetExport re-publishes the tracked-set breakdown that M1-06 writes
// on every pull, with an export header so it states its as-of time and licence.
func (r *run) trackedSetExport(ctx context.Context) error {
	raw, err := r.Ports.Publisher.GetFile(ctx, selection.BreakdownPath)
	if errors.Is(err, data.ErrNotFound) {
		r.skipf("tracked set: no breakdown stored yet")
		return nil
	}
	if err != nil {
		return fmt.Errorf("read breakdown: %w", err)
	}
	asOf := breakdownAsOf(raw)
	if asOf.IsZero() {
		r.skipf("tracked set: breakdown has no as_of")
		return nil
	}
	f := TrackedSetFile{
		Header:    newHeader(trackedSetID, data.ExportOther, asOf, data.ContentPlatform),
		Breakdown: json.RawMessage(raw),
	}
	return r.put(ctx, entryOf(trackedSetID, data.ExportOther, "Tracked set breakdown", jsonType,
		data.LicenceCC0, asOf, data.ContentPlatform), f, true)
}

// breakdownAsOf reads the as-of time of a stored breakdown; zero when it has none.
func breakdownAsOf(raw []byte) time.Time {
	var head struct {
		AsOf time.Time `json:"as_of"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return time.Time{}
	}
	return head.AsOf
}
