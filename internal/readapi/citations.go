package readapi

import (
	"errors"
	"fmt"
	"net/http"

	apiv1 "github.com/easterthebunny/pubcommittee/api/v1"
	"github.com/easterthebunny/pubcommittee/internal/billxml"
	"github.com/easterthebunny/pubcommittee/internal/citations"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

// detailNoCitations names the gap: no citation set is stored for the version
// under the current release.
const detailNoCitations = "no citations are published for this version"

// ListCitations serves the stored citations of one version, in stored order,
// optionally filtered by resolution state.
func (s *Server) ListCitations(
	w http.ResponseWriter, r *http.Request, bill apiv1.BillPath, n apiv1.VersionPath, p apiv1.ListCitationsParams,
) {
	res, set, ok := s.storedCitations(w, r, bill, n)
	if !ok {
		return
	}
	keep := make([]data.CitationEntry, 0, len(set.Links))
	for _, e := range set.Links {
		if p.State != nil && apiv1.CitationState(e.State) != *p.State {
			continue
		}
		keep = append(keep, e)
	}
	s.sendCitations(w, r, res, set, keep, p.Cursor, p.Limit)
}

// ListUnresolvedCitations serves the unresolved and ambiguous citations of one
// version, in stored order.
func (s *Server) ListUnresolvedCitations(
	w http.ResponseWriter, r *http.Request, bill apiv1.BillPath, n apiv1.VersionPath,
	p apiv1.ListUnresolvedCitationsParams,
) {
	res, set, ok := s.storedCitations(w, r, bill, n)
	if !ok {
		return
	}
	s.sendCitations(w, r, res, set, set.Unresolved(), p.Cursor, p.Limit)
}

// storedCitations resolves the version and loads its stored citation set under
// the current release. When it answers, it writes the response and ok is
// false.
func (s *Server) storedCitations(
	w http.ResponseWriter, r *http.Request, bill, n string,
) (res resolved, set data.CitationSet, ok bool) {
	ctx := r.Context()
	res, err := s.resolve(ctx, bill, n, false)
	if err != nil {
		fail(w, r, err)
		return resolved{}, data.CitationSet{}, false
	}
	if s.ports.Citations == nil || s.ports.Law == nil {
		notFound(w, detailNoCitations)
		return resolved{}, data.CitationSet{}, false
	}
	rel, err := s.ports.Law.CurrentRelease(ctx)
	if err != nil {
		citationsError(w, r, err)
		return resolved{}, data.CitationSet{}, false
	}
	vs, err := s.listVersions(ctx, res.ref)
	if err != nil {
		fail(w, r, err)
		return resolved{}, data.CitationSet{}, false
	}
	ref := data.CitationRef{
		Version: vs[res.n-1].Ref, Method: citations.Method(billxml.ParserVersion, rel.ID),
	}
	set, err = s.ports.Citations.GetCitations(ctx, ref)
	if err != nil {
		citationsError(w, r, err)
		return resolved{}, data.CitationSet{}, false
	}
	return res, set, true
}

// citationsError answers not found when the store holds nothing for the
// request, and fails on any other error.
func citationsError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, data.ErrNotFound) {
		notFound(w, detailNoCitations)
		return
	}
	fail(w, r, fmt.Errorf("get citations: %w", err))
}

// sendCitations pages the entries and writes them as citations of res.
func (s *Server) sendCitations(
	w http.ResponseWriter, r *http.Request, res resolved, set data.CitationSet,
	entries []data.CitationEntry, cursor *string, limit *int,
) {
	page, next, err := slicePage(entries, cursor, limit)
	if err != nil {
		invalid(w, detailInvalidCur)
		return
	}
	items := make([]apiv1.Citation, len(page))
	for i, e := range page {
		items[i] = toCitation(res, set, e)
	}
	sendJSON(w, r, cacheShort, apiv1.CitationPage{Items: items, Next: next})
}

func toCitation(res resolved, set data.CitationSet, e data.CitationEntry) apiv1.Citation {
	ids := e.ProvisionIDs
	if ids == nil {
		ids = []string{}
	}
	out := apiv1.Citation{
		Bill: res.id, Code: res.code, SectionId: sectionID(e.Section), Text: e.Text,
		State: apiv1.CitationState(e.State), ProvisionIds: ids,
	}
	if e.Line > 0 {
		out.Line = new(e.Line)
	}
	out.Version, out.AsOf, out.Provenance, out.ContentClass = meta(set.Meta)
	return out
}
