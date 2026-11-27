package readapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	apiv1 "github.com/easterthebunny/pubcommittee/api/v1"
	"github.com/easterthebunny/pubcommittee/internal/billxml"
	"github.com/easterthebunny/pubcommittee/internal/citations"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/versiondiff"
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

// toExplanation builds the resource. The lines it is pinned to come from the stored anchors, else from the
// citations of its provision, else the whole section (data.Section.ExplanationSpan), so explanations stored
// before anchors were kept are pinned too.
func toExplanation(
	res resolved, e data.ChangeExplanation, sec data.Section, links []data.CitationLink,
) apiv1.Explanation {
	out := apiv1.Explanation{
		Bill: res.id, Code: res.code, SectionId: sectionID(e.Section),
		WhatChanges: e.WhatChanges, WhyItMatters: e.WhyMatters, Flags: []apiv1.ExplanationFlags{},
	}
	out.Version, out.AsOf, out.Provenance, out.ContentClass = meta(e.Meta)
	if e.ProvisionID != "" {
		out.ProvisionId = new(e.ProvisionID)
		out.ProvisionCitation = new(data.ProvisionCitation(e.ProvisionID))
	}
	if from, to := sec.ExplanationSpan(e.Anchors, e.ProvisionID, links); from > 0 {
		out.LineFrom, out.LineTo = &from, &to
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
	res, err := s.resolve(r.Context(), bill, n, true)
	if err != nil {
		fail(w, r, err)
		return
	}
	all, err := s.ports.PublicDocs.GetExplanations(r.Context(), res.version.Ref)
	if err != nil {
		fail(w, r, fmt.Errorf("get explanations: %w", err))
		return
	}
	// A section that only names the Act needs no explanation; one stored before planning skipped it is hidden.
	all = data.ExplanationsToShow(res.version, all)
	links := s.citationLinks(r.Context(), res.version.Ref)
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
		items[i] = toExplanation(res, e, sectionOf(res.version, e.Section), links)
	}
	class := cacheShort
	sendJSON(w, r, class, apiv1.ExplanationPage{Items: items, Next: next})
}

// GetBillSummary serves the published summary of one version; not found
// until job-summarize has stored one (M1-09).
func (s *Server) GetBillSummary(w http.ResponseWriter, r *http.Request, bill apiv1.BillPath, n apiv1.VersionPath) {
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
	out, err := s.published(r.Context(), data.TaskBillSummary, vs[res.n-1].Ref, "")
	if errors.Is(err, data.ErrNotFound) {
		notFound(w, detailNoSummary)
		return
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	doc := apiv1.BillSummaryDoc{
		Address:    res.address(versionPath(res.id, res.code)+"/summary", ""),
		Bill:       res.id,
		Code:       res.code,
		Headline:   out.Headline,
		Statements: toStatements(out.Statements),
		Flags:      new([]apiv1.BillSummaryDocFlags{apiv1.BillSummaryDocFlagsAuto}),
	}
	doc.Version, doc.AsOf, doc.Provenance, doc.ContentClass = meta(out.Meta)
	// Summaries are short-lived even for a pinned version: stewards may correct them.
	sendJSON(w, r, cacheShort, doc)
}

// GetVersionChangeSummary serves the published change summary of one version
// against the previous one; not found until job-summarize has stored one
// (M1-09), and never for the first version.
func (s *Server) GetVersionChangeSummary(
	w http.ResponseWriter, r *http.Request, bill apiv1.BillPath, n apiv1.VersionPath,
) {
	res, err := s.resolve(r.Context(), bill, n, false)
	if err != nil {
		fail(w, r, err)
		return
	}
	if res.n == 1 {
		notFound(w, detailNoChanges)
		return
	}
	vs, err := s.listVersions(r.Context(), res.ref)
	if err != nil {
		fail(w, r, err)
		return
	}
	// The target of a change summary is the code of the previous version.
	out, err := s.published(r.Context(), data.TaskVersionChanges, vs[res.n-1].Ref, vs[res.n-2].Ref.Version)
	if errors.Is(err, data.ErrNotFound) {
		s.serveNoDifferences(w, r, res, vs)
		return
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	doc := apiv1.VersionChangeSummary{
		Bill:       res.id,
		FromCode:   res.vs[res.n-2].Ref.Version,
		ToCode:     res.code,
		Headline:   out.Headline,
		Statements: toStatements(out.Statements),
		Flags:      new([]apiv1.VersionChangeSummaryFlags{apiv1.VersionChangeSummaryFlagsAuto}),
	}
	doc.Version, doc.AsOf, doc.Provenance, doc.ContentClass = meta(out.Meta)
	sendJSON(w, r, cacheShort, doc)
}

// serveNoDifferences answers a version whose text equals the previous one
// with a fixed change summary built from the stored comparison (no model
// call). Any other version without a published summary is not found.
func (s *Server) serveNoDifferences(w http.ResponseWriter, r *http.Request, res resolved, vs []data.VersionSummary) {
	if s.ports.Comparisons == nil {
		notFound(w, detailNoChanges)
		return
	}
	c, err := s.ports.Comparisons.GetComparison(r.Context(), data.ComparisonRef{
		Bill: res.ref, From: vs[res.n-2].Ref.Version, To: vs[res.n-1].Ref.Version,
		Method: versiondiff.Method(billxml.ParserVersion),
	})
	if errors.Is(err, data.ErrNotFound) || (err == nil && hasChanges(c)) {
		notFound(w, detailNoChanges)
		return
	}
	if err != nil {
		fail(w, r, fmt.Errorf("get comparison: %w", err))
		return
	}
	doc := apiv1.VersionChangeSummary{
		Bill:       res.id,
		FromCode:   res.vs[res.n-2].Ref.Version,
		ToCode:     res.code,
		Headline:   fmt.Sprintf("No text changes from version %s.", res.vs[res.n-2].Ref.Version),
		Statements: []apiv1.Statement{},
		Flags:      new([]apiv1.VersionChangeSummaryFlags{apiv1.VersionChangeSummaryFlagsNoDifferences}),
	}
	doc.Version, doc.AsOf, doc.Provenance, _ = meta(c.Meta)
	// The headline is platform text, whatever the comparison's class.
	doc.ContentClass = apiv1.TrustedContentClass(data.ContentPlatform)
	sendJSON(w, r, cacheShort, doc)
}

// hasChanges reports whether a comparison holds any change.
func hasChanges(c data.Comparison) bool {
	for _, sc := range c.Sections {
		if len(sc.Changes) > 0 {
			return true
		}
	}
	return false
}

// published reads the current output of a task for a version and target. A
// server with no generated store, and a version with nothing published, both
// answer data.ErrNotFound.
func (s *Server) published(
	ctx context.Context, task string, v data.VersionRef, target string,
) (data.GeneratedOutput, error) {
	if s.ports.Generated == nil {
		return data.GeneratedOutput{}, fmt.Errorf("%s: %w", task, data.ErrNotFound)
	}
	out, err := s.ports.Generated.GetCurrent(ctx, task, v, target)
	if err != nil {
		return data.GeneratedOutput{}, fmt.Errorf("get %s: %w", task, err)
	}
	// An output the grounding grader marked contradicted is withheld.
	if g, gerr := s.ports.Generated.GetGrade(ctx, out.Key); gerr == nil && g.Contradicted() {
		return data.GeneratedOutput{}, fmt.Errorf("%s withheld after grading: %w", task, data.ErrNotFound)
	}
	return out, nil
}

// toStatements maps stored statements to the contract. Every statement is
// platform content; anchors and statements are never null on the wire.
func toStatements(in []data.Statement) []apiv1.Statement {
	out := make([]apiv1.Statement, len(in))
	for i, s := range in {
		st := apiv1.Statement{
			Text:         s.Text,
			ContentClass: apiv1.StatementContentClassPlatform,
			Anchors:      append([]string{}, s.Anchors...),
		}
		if len(s.ProvisionIDs) > 0 {
			st.ProvisionIds = new(slices.Clone(s.ProvisionIDs))
		}
		out[i] = st
	}
	return out
}

// sectionOf returns the section of v with this path; the zero Section when there is none.
func sectionOf(v data.OfficialVersion, path []string) data.Section {
	for _, sec := range v.Sections {
		if slices.Equal(sec.Path, path) {
			return sec
		}
	}
	return data.Section{}
}

// citationLinks returns the citation links stored for a version under the current release; none when the
// citations are not stored (they only refine where an explanation is pinned).
func (s *Server) citationLinks(ctx context.Context, ref data.VersionRef) []data.CitationLink {
	if s.ports.Citations == nil || s.ports.Law == nil {
		return nil
	}
	rel, err := s.ports.Law.CurrentRelease(ctx)
	if err != nil {
		return nil
	}
	set, err := s.ports.Citations.GetCitations(ctx, data.CitationRef{
		Version: ref, Method: citations.Method(billxml.ParserVersion, rel.ID),
	})
	if err != nil {
		return nil
	}
	links := make([]data.CitationLink, len(set.Links))
	for i, l := range set.Links {
		links[i] = l.CitationLink
	}
	return links
}
