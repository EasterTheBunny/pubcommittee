package readapi

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	apiv1 "github.com/easterthebunny/pubcommittee/api/v1"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

// Details for derived documents whose data other tasks produce. They name
// the gap without inventing content.
const (
	detailNoSummary  = "no summary is published for this version"
	detailNoChanges  = "no change summary is published for this version"
	detailNoCompare  = "no comparison is published for these versions"
	detailInvalidCur = "cursor or limit is not valid"
)

// isExplanationFlag reports whether the contract allows p as an explanation flag.
func isExplanationFlag(p data.Provenance) bool {
	return p == data.ProvenanceAuto || p == data.ProvenanceStewardChecked || p == data.ProvenanceBillOnly
}

func toExplanation(res resolved, e data.ChangeExplanation) apiv1.Explanation {
	out := apiv1.Explanation{
		Bill: res.id, N: res.n, SectionId: sectionID(e.Section),
		WhatChanges: e.WhatChanges, WhyItMatters: e.WhyMatters, Flags: []apiv1.ExplanationFlags{},
	}
	out.Version, out.AsOf, out.Provenance, out.ContentClass = meta(e.Meta)
	if e.ProvisionID != "" {
		out.ProvisionId = new(e.ProvisionID)
	}
	if e.CurrentText != "" {
		out.CurrentText = &apiv1.ClassedText{
			Text: e.CurrentText, ContentClass: apiv1.TrustedContentClass(data.ContentOfficial),
		}
	}
	for _, f := range e.Flags {
		if isExplanationFlag(f) {
			out.Flags = append(out.Flags, apiv1.ExplanationFlags(f))
		}
	}
	if e.AsAmended != "" {
		out.AsAmended = &apiv1.AsAmendedPreview{
			Text: e.AsAmended, ContentClass: "platform", Unofficial: true,
		}
	}
	return out
}

// ListExplanations serves the change explanations of one version.
func (s *Server) ListExplanations(
	w http.ResponseWriter, r *http.Request, bill apiv1.BillPath, n apiv1.VersionPath, p apiv1.ListExplanationsParams,
) {
	res, err := s.resolve(r.Context(), bill, n, false)
	if err != nil {
		fail(w, r, err)
		return
	}
	vs, err := s.listVersions(r.Context(), res.ref)
	if err != nil {
		fail(w, r, err)
		return
	}
	all, err := s.ports.PublicDocs.GetExplanations(r.Context(), vs[res.n-1].Ref)
	if err != nil {
		fail(w, r, fmt.Errorf("get explanations: %w", err))
		return
	}
	slices.SortFunc(all, func(a, b data.ChangeExplanation) int {
		if c := strings.Compare(sectionID(a.Section), sectionID(b.Section)); c != 0 {
			return c
		}
		return strings.Compare(a.ProvisionID, b.ProvisionID)
	})
	page, next, err := slicePage(all, p.Cursor, p.Limit)
	if err != nil {
		invalid(w, detailInvalidCur)
		return
	}
	items := make([]apiv1.Explanation, len(page))
	for i, e := range page {
		items[i] = toExplanation(res, e)
	}
	class := cacheShort
	sendJSON(w, r, class, apiv1.ExplanationPage{Items: items, Next: next})
}

// GetBillSummary answers not found until M1-09 stores summaries.
func (s *Server) GetBillSummary(w http.ResponseWriter, r *http.Request, bill apiv1.BillPath, n apiv1.VersionPath) {
	if _, err := s.resolve(r.Context(), bill, n, false); err != nil {
		fail(w, r, err)
		return
	}
	notFound(w, detailNoSummary)
}

// GetVersionChangeSummary answers not found until M1-09 stores change
// summaries; the first version never has one.
func (s *Server) GetVersionChangeSummary(
	w http.ResponseWriter, r *http.Request, bill apiv1.BillPath, n apiv1.VersionPath,
) {
	if _, err := s.resolve(r.Context(), bill, n, false); err != nil {
		fail(w, r, err)
		return
	}
	notFound(w, detailNoChanges)
}

// ListCitations serves the stored citations of one version. No port holds
// citations until M1-07, so the page is empty for a version that exists.
func (s *Server) ListCitations(
	w http.ResponseWriter, r *http.Request, bill apiv1.BillPath, n apiv1.VersionPath, p apiv1.ListCitationsParams,
) {
	s.citations(w, r, bill, n, p.Cursor, p.Limit)
}

// ListUnresolvedCitations serves the unresolved and ambiguous citations.
func (s *Server) ListUnresolvedCitations(
	w http.ResponseWriter, r *http.Request, bill apiv1.BillPath, n apiv1.VersionPath,
	p apiv1.ListUnresolvedCitationsParams,
) {
	s.citations(w, r, bill, n, p.Cursor, p.Limit)
}

func (s *Server) citations(w http.ResponseWriter, r *http.Request, bill, n string, cursor *string, limit *int) {
	if _, err := s.resolve(r.Context(), bill, n, false); err != nil {
		fail(w, r, err)
		return
	}
	page, next, err := slicePage([]apiv1.Citation{}, cursor, limit)
	if err != nil {
		invalid(w, detailInvalidCur)
		return
	}
	sendJSON(w, r, cacheShort, apiv1.CitationPage{Items: page, Next: next})
}
