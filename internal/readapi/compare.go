package readapi

import (
	"fmt"
	"net/http"
	"strings"

	apiv1 "github.com/easterthebunny/pubcommittee/api/v1"
	"github.com/easterthebunny/pubcommittee/internal/billxml"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/versiondiff"
)

// CompareVersions serves the stored comparison of version n (base) with
// version m. Both versions must exist and a comparison must be stored for the
// pair, oldest first; otherwise the answer is not found.
func (s *Server) CompareVersions(
	w http.ResponseWriter, r *http.Request, bill apiv1.BillPath, n apiv1.VersionPath, m int,
) {
	ctx := r.Context()
	res, err := s.resolve(ctx, bill, n, false)
	if err != nil {
		fail(w, r, err)
		return
	}
	if m < 1 || m > res.latest {
		fail(w, r, fmt.Errorf("version %d of %s: %w", m, res.id, data.ErrNotFound))
		return
	}
	if s.ports.Comparisons == nil {
		notFound(w, detailNoCompare)
		return
	}
	vs, err := s.listVersions(ctx, res.ref)
	if err != nil {
		fail(w, r, err)
		return
	}
	ref := data.ComparisonRef{
		Bill: res.ref, From: vs[res.n-1].Ref.Version, To: vs[m-1].Ref.Version,
		Method: versiondiff.Method(billxml.ParserVersion),
	}
	// Equal or reversed versions have no stored comparison; skip the lookup
	// so a bad pair costs no storage read.
	if res.n >= m {
		notFound(w, detailNoCompare)
		return
	}
	c, err := s.ports.Comparisons.GetComparison(ctx, ref)
	if err != nil {
		fail(w, r, fmt.Errorf("get comparison: %w", err))
		return
	}
	from, err := s.ports.BillText.GetVersion(ctx, vs[res.n-1].Ref)
	if err != nil {
		fail(w, r, fmt.Errorf("get version: %w", err))
		return
	}
	to, err := s.ports.BillText.GetVersion(ctx, vs[m-1].Ref)
	if err != nil {
		fail(w, r, fmt.Errorf("get version: %w", err))
		return
	}
	out := apiv1.VersionComparison{
		Bill: res.id, FromN: res.n, ToN: m, Sections: toComparisonSections(c, from, to),
	}
	out.Version, out.AsOf, out.Provenance, out.ContentClass = meta(c.Meta)
	// The stored text is immutable and both versions are explicit.
	sendJSON(w, r, cacheImmutable, out)
}

func toComparisonSections(c data.Comparison, from, to data.OfficialVersion) []apiv1.ComparisonSection {
	out := make([]apiv1.ComparisonSection, 0, len(c.Sections))
	for _, sc := range c.Sections {
		cs := apiv1.ComparisonSection{
			SectionId: sectionID(sc.Path), Status: apiv1.ComparisonSectionStatus(sc.Status),
			Changes: make([]apiv1.ComparisonChange, 0, len(sc.Changes)),
		}
		for _, ch := range sc.Changes {
			cs.Changes = append(cs.Changes, toComparisonChange(ch, from, to))
		}
		out = append(out, cs)
	}
	return out
}

func toComparisonChange(ch data.ComparisonChange, from, to data.OfficialVersion) apiv1.ComparisonChange {
	out := apiv1.ComparisonChange{Kind: apiv1.ComparisonChangeKind(ch.Kind)}
	if ch.BeforeFrom > 0 {
		out.BeforeSpan = &apiv1.LineSpan{SectionId: sectionID(ch.BeforeSection), From: ch.BeforeFrom, To: ch.BeforeTo}
		out.Before = officialText(from, ch.BeforeSection, ch.BeforeFrom, ch.BeforeTo)
	}
	if ch.AfterFrom > 0 {
		out.AfterSpan = &apiv1.LineSpan{SectionId: sectionID(ch.AfterSection), From: ch.AfterFrom, To: ch.AfterTo}
		out.After = officialText(to, ch.AfterSection, ch.AfterFrom, ch.AfterTo)
	}
	return out
}

// officialText joins the lines from..to of a section of v with newlines.
func officialText(v data.OfficialVersion, path []string, from, to int) *apiv1.ClassedText {
	sec, err := findSection(v, path)
	if err != nil {
		return nil
	}
	var parts []string
	for _, l := range sec.Lines {
		if l.N >= from && l.N <= to {
			parts = append(parts, l.Text)
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return &apiv1.ClassedText{
		Text: strings.Join(parts, "\n"), ContentClass: apiv1.TrustedContentClass(data.ContentOfficial),
	}
}
