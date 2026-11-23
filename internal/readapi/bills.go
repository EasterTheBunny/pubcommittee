package readapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	apiv1 "github.com/easterthebunny/pubcommittee/api/v1"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

// scanBatch is the page size used when scanning the catalog for filtered
// results.
const scanBatch = 100

// catalogCursor resumes a filtered catalog scan: the port cursor of the page
// being scanned and how many of its items were already consumed.
type catalogCursor struct {
	Port string `json:"p"`
	Skip int    `json:"s"`
}

// catalogFilter holds the ListBills filters; the zero value matches all.
type catalogFilter struct {
	tracked   bool
	status    string
	chamber   string
	committee *bool
	subject   string
	terms     []string
}

func newFilter(p apiv1.ListBillsParams) catalogFilter {
	f := catalogFilter{tracked: p.Scope == nil || *p.Scope != apiv1.ListBillsParamsScope("catalog")}
	if p.Status != nil {
		f.status = strings.ToLower(*p.Status)
	}
	if p.Chamber != nil {
		f.chamber = string(*p.Chamber)
	}
	f.committee = p.CommitteeActivity
	if p.Subject != nil {
		f.subject = *p.Subject
	}
	if p.Q != nil {
		f.terms = strings.Fields(strings.ToLower(*p.Q))
	}
	return f
}

func (f catalogFilter) match(e data.CatalogEntry) bool {
	return f.matchFields(e) && f.matchTerms(e)
}

func (f catalogFilter) matchFields(e data.CatalogEntry) bool {
	switch {
	case f.tracked && !e.Tracked:
		return false
	case f.status != "" && strings.ToLower(e.Status) != f.status:
		return false
	case f.chamber != "" && e.Chamber != f.chamber:
		return false
	}
	return f.matchTopics(e)
}

func (f catalogFilter) matchTopics(e data.CatalogEntry) bool {
	if f.committee != nil && e.CommitteeActivity != *f.committee {
		return false
	}
	return f.subject == "" || slices.Contains(e.Subjects, f.subject)
}

// matchTerms requires every search word to appear in the title, the bill
// type and number, or the bill id.
func (f catalogFilter) matchTerms(e data.CatalogEntry) bool {
	if len(f.terms) == 0 {
		return true
	}
	hay := strings.ToLower(strings.Join([]string{
		e.Title, e.Bill.Type, strconv.Itoa(e.Bill.Number), e.Bill.Type + strconv.Itoa(e.Bill.Number), billID(e.Bill),
	}, " "))
	for _, t := range f.terms {
		if !strings.Contains(hay, t) {
			return false
		}
	}
	return true
}

// startCursor decodes the cursor of a filtered scan; none starts at the top.
func startCursor(cursor *string) (catalogCursor, error) {
	var cur catalogCursor
	if cursor == nil || *cursor == "" {
		return cur, nil
	}
	if err := decodeCursor(*cursor, &cur); err != nil || cur.Skip < 0 {
		return catalogCursor{}, fmt.Errorf("cursor: %w", data.ErrInvalid)
	}
	return cur, nil
}

// scanCatalog returns up to limit matching entries and the cursor of the
// rest. It reads the port page by page and never holds more than one page.
func (s *Server) scanCatalog(
	ctx context.Context, f catalogFilter, cursor *string, limit int,
) ([]data.CatalogEntry, *string, error) {
	cur, err := startCursor(cursor)
	if err != nil {
		return nil, nil, err
	}
	var out []data.CatalogEntry
	for {
		pg, err := s.ports.PublicDocs.ListCatalog(ctx, data.Page{Cursor: cur.Port, Limit: scanBatch})
		if err != nil {
			return nil, nil, fmt.Errorf("list catalog: %w", err)
		}
		for i := cur.Skip; i < len(pg.Items); i++ {
			if !f.match(pg.Items[i]) {
				continue
			}
			if len(out) == limit {
				tok, err := encodeCursor(catalogCursor{Port: cur.Port, Skip: i})
				if err != nil {
					return nil, nil, err
				}
				return out, &tok, nil
			}
			out = append(out, pg.Items[i])
		}
		if pg.Next == "" {
			return out, nil, nil
		}
		cur = catalogCursor{Port: pg.Next}
	}
}

// checkListParams validates the ListBills parameters; a non-empty detail
// names the first problem.
func checkListParams(p apiv1.ListBillsParams) (limit int, detail string) {
	if p.Scope != nil && *p.Scope != "tracked" && *p.Scope != "catalog" {
		return 0, "scope must be tracked or catalog"
	}
	if p.Chamber != nil && !slices.Contains([]string{"house", "senate", "joint"}, string(*p.Chamber)) {
		return 0, "chamber must be house, senate, or joint"
	}
	limit, err := limitOf(p.Limit)
	if err != nil {
		return 0, "limit must be from 1 to 200"
	}
	return limit, ""
}

// ListBills serves tracked bills or catalog entries.
func (s *Server) ListBills(w http.ResponseWriter, r *http.Request, p apiv1.ListBillsParams) {
	limit, detail := checkListParams(p)
	if detail != "" {
		invalid(w, detail)
		return
	}
	f := newFilter(p)
	entries, next, err := s.scanCatalog(r.Context(), f, p.Cursor, limit)
	if err != nil {
		if errors.Is(err, data.ErrInvalid) {
			invalid(w, "cursor is not valid")
			return
		}
		fail(w, r, err)
		return
	}
	if !f.tracked {
		items := make([]apiv1.CatalogEntry, len(entries))
		for i, e := range entries {
			items[i] = toCatalogEntry(e)
		}
		sendJSON(w, r, cacheShort, apiv1.CatalogEntryPage{Items: items, Next: next})
		return
	}
	items := make([]apiv1.BillSummary, len(entries))
	for i, e := range entries {
		items[i], err = s.summaryOf(r.Context(), e)
		if err != nil {
			fail(w, r, err)
			return
		}
	}
	sendJSON(w, r, cacheShort, apiv1.BillSummaryPage{Items: items, Next: next})
}

func toLinks(in []data.Link) []apiv1.Link {
	out := make([]apiv1.Link, len(in))
	for i, l := range in {
		out[i] = apiv1.Link{Label: l.Label, Url: l.URL}
		if l.Kind != "" {
			out[i].Kind = new(apiv1.LinkKind(l.Kind))
		}
	}
	return out
}

func strsPtr(in []string) *[]string {
	if len(in) == 0 {
		return nil
	}
	return new(slices.Clone(in))
}

func toRef(b data.BillRef) apiv1.BillRef {
	return apiv1.BillRef{Jurisdiction: b.Jurisdiction, Session: b.Session, Type: b.Type, Number: b.Number}
}

// markupStatus returns the entry's markup status; an entry that never had
// one is in review when tracked and otherwise not a candidate, so no
// request to mark it up is offered.
func markupStatus(e data.CatalogEntry) apiv1.CatalogEntryMarkupStatus {
	switch {
	case e.MarkupStatus != "":
		return apiv1.CatalogEntryMarkupStatus(e.MarkupStatus)
	case e.Tracked:
		return apiv1.CatalogEntryMarkupStatus(data.MarkupInReview)
	}
	return apiv1.CatalogEntryMarkupStatus(data.MarkupNotACandidate)
}

// toCatalogEntry converts an entry. It carries metadata and links only,
// never bill text, so an untracked entry exposes none.
func toCatalogEntry(e data.CatalogEntry) apiv1.CatalogEntry {
	id := billID(e.Bill)
	out := apiv1.CatalogEntry{
		Id: id, Ref: toRef(e.Bill), Title: e.Title, Tracked: e.Tracked,
		MarkupStatus: markupStatus(e), OfficialLinks: toLinks(e.OfficialLinks),
		Reasons: strsPtr(e.Reasons), Subjects: strsPtr(e.Subjects),
	}
	out.Version, out.AsOf, out.Provenance, out.ContentClass = meta(e.Meta)
	if e.Status != "" {
		out.Status = new(e.Status)
	}
	if e.Chamber != "" {
		out.Chamber = new(apiv1.Chamber(e.Chamber))
	}
	if e.CommitteeActivity {
		out.CommitteeActivity = new(true)
	}
	if e.MarkupStatus == data.MarkupNotACandidate && e.ExclusionReason != "" {
		out.ExclusionReason = new(e.ExclusionReason)
	}
	if e.Tracked {
		out.Address = new(billPath(id))
	}
	addTracking(&out, e)
	return out
}

// summaryOf builds the summary of a tracked bill from its bill document,
// falling back to the catalog entry when the document is absent.
func (s *Server) summaryOf(ctx context.Context, e data.CatalogEntry) (apiv1.BillSummary, error) {
	id := billID(e.Bill)
	out := apiv1.BillSummary{Id: id, Ref: toRef(e.Bill), Title: e.Title, Status: e.Status, Address: billPath(id)}
	out.Version, out.AsOf, out.Provenance, out.ContentClass = meta(e.Meta)
	if e.Chamber != "" {
		out.Chamber = new(apiv1.Chamber(e.Chamber))
	}
	b, err := s.ports.PublicDocs.GetBill(ctx, e.Bill)
	switch {
	case errors.Is(err, data.ErrNotFound):
		return out, nil
	case err != nil:
		return apiv1.BillSummary{}, fmt.Errorf("get bill: %w", err)
	}
	out.Title, out.Status = b.Title, b.Status
	out.Version, out.AsOf, out.Provenance, out.ContentClass = meta(b.Meta)
	if b.Chamber != "" {
		out.Chamber = new(apiv1.Chamber(b.Chamber))
	}
	if n := len(b.Versions); n > 0 {
		out.LatestVersion = new(n)
	}
	return out, nil
}

// GetBill serves one bill with its version list.
func (s *Server) GetBill(w http.ResponseWriter, r *http.Request, bill apiv1.BillPath) {
	ref, err := parseBillID(bill)
	if err != nil {
		fail(w, r, err)
		return
	}
	b, err := s.ports.PublicDocs.GetBill(r.Context(), ref)
	if err != nil {
		fail(w, r, fmt.Errorf("get bill: %w", err))
		return
	}
	vs, err := s.ports.BillText.ListVersions(r.Context(), ref)
	if err != nil {
		fail(w, r, fmt.Errorf("list versions: %w", err))
		return
	}
	id := billID(ref)
	out := apiv1.Bill{
		Id: id, Ref: toRef(ref), Title: b.Title, Status: b.Status, Address: billPath(id),
		Sponsor: b.Sponsor, ReviewState: apiv1.BillReviewState(reviewState(b)),
		OfficialLinks: toLinks(b.OfficialLinks), TrackedReasons: strsPtr(b.TrackedReasons),
		Versions: make([]apiv1.VersionSummary, len(vs)),
	}
	out.Version, out.AsOf, out.Provenance, out.ContentClass = meta(b.Meta)
	if b.Chamber != "" {
		out.Chamber = new(apiv1.Chamber(b.Chamber))
	}
	if b.PolicyVersion > 0 {
		out.PolicyVersion = new(b.PolicyVersion)
	}
	if len(vs) > 0 {
		out.LatestVersion = new(len(vs))
	}
	for i, v := range vs {
		out.Versions[i] = apiv1.VersionSummary{
			N: i + 1, Code: v.Ref.Version, Type: v.Type, PublishedAt: v.PublishedAt.UTC(),
			Address: versionPath(id, i+1),
		}
	}
	sendJSON(w, r, cacheShort, out)
}

func reviewState(b data.Bill) string {
	if b.ReviewState == "" {
		return data.ReviewOpen
	}
	return b.ReviewState
}
