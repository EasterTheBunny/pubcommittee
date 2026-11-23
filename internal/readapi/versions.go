package readapi

import (
	"fmt"
	"net/http"
	"slices"

	apiv1 "github.com/easterthebunny/pubcommittee/api/v1"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

const licencePublicDomain = "public-domain"

// toLines converts a section's lines, computing each line's block_to in one
// pass: the last line of the run nested deeper than it.
func toLines(path []string, lines []data.Line) []apiv1.Line {
	end := make([]int, len(lines)) // index of the last line of each block
	for i := len(lines) - 1; i >= 0; i-- {
		end[i] = i
		for j := i + 1; j < len(lines) && lines[j].Depth > lines[i].Depth; j = end[j] + 1 {
			end[i] = end[j]
		}
	}
	out := make([]apiv1.Line, len(lines))
	for i, l := range lines {
		out[i] = apiv1.Line{
			N: l.N, Depth: l.Depth, Text: l.Text,
			Anchor: anchorOf(path, l.N), BlockTo: lines[end[i]].N,
		}
		if l.SourceID != "" {
			out[i].SourceId = new(l.SourceID)
		}
	}
	return out
}

func toSection(s data.Section) apiv1.Section {
	out := apiv1.Section{
		Id: sectionID(s.Path), Path: slices.Clone(s.Path), Label: s.Label,
		Heading: s.Heading, Lines: toLines(s.Path, s.Lines),
	}
	if s.SourceID != "" {
		out.SourceId = new(s.SourceID)
	}
	return out
}

// findSection returns the section with the given path.
func findSection(v data.OfficialVersion, path []string) (data.Section, error) {
	for _, s := range v.Sections {
		if slices.Equal(s.Path, path) {
			return s, nil
		}
	}
	return data.Section{}, fmt.Errorf("section %s: %w", sectionID(path), data.ErrNotFound)
}

// GetVersion serves one official version.
func (s *Server) GetVersion(w http.ResponseWriter, r *http.Request, bill apiv1.BillPath, n apiv1.VersionPath) {
	res, err := s.resolve(r.Context(), bill, n, true)
	if err != nil {
		fail(w, r, err)
		return
	}
	canonical := versionPath(res.id, res.n)
	v := res.version
	out := apiv1.Version{
		Bill: res.id, N: res.n, Code: v.Ref.Version, Type: v.Type, PublishedAt: v.PublishedAt.UTC(),
		SourceSha256: v.SourceSHA256, Licence: licencePublicDomain, Address: res.address(canonical, ""),
		Sections: make([]apiv1.Section, len(v.Sections)),
	}
	out.Version, out.AsOf, out.Provenance, out.ContentClass = meta(v.Meta)
	if v.StewardEntered {
		out.StewardEntered = new(true)
	}
	for i, sec := range v.Sections {
		out.Sections[i] = toSection(sec)
	}
	sendJSONAt(w, r, res.class(), res.location(canonical), out)
}

// GetSection serves one section of a pinned or latest version.
func (s *Server) GetSection(
	w http.ResponseWriter, r *http.Request, bill apiv1.BillPath, n apiv1.VersionPath, section apiv1.SectionPath,
) {
	s.section(w, r, bill, n, section)
}

// GetSectionLatest serves a section of the latest version.
func (s *Server) GetSectionLatest(
	w http.ResponseWriter, r *http.Request, bill apiv1.BillPath, section apiv1.SectionPath,
) {
	s.section(w, r, bill, latestWord, section)
}

func (s *Server) section(w http.ResponseWriter, r *http.Request, bill, n, section string) {
	path, err := parseSection(section)
	if err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.resolve(r.Context(), bill, n, true)
	if err != nil {
		fail(w, r, err)
		return
	}
	sec, err := findSection(res.version, path)
	if err != nil {
		fail(w, r, err)
		return
	}
	canonical := versionPath(res.id, res.n) + "/s/" + sectionID(path)
	out := apiv1.SectionResource{
		Bill: res.id, N: res.n, Licence: licencePublicDomain,
		Address: res.address(canonical, ""), Section: toSection(sec),
	}
	out.Version, out.AsOf, out.Provenance, out.ContentClass = meta(res.version.Meta)
	sendJSONAt(w, r, res.class(), res.location(canonical), out)
}

// GetLineRange serves a line or range of a pinned or latest version.
func (s *Server) GetLineRange(
	w http.ResponseWriter, r *http.Request, bill apiv1.BillPath, n apiv1.VersionPath,
	section apiv1.SectionPath, pRange apiv1.RangePath,
) {
	s.lineRange(w, r, bill, n, section, pRange)
}

// GetLineRangeLatest serves a line range of the latest version.
func (s *Server) GetLineRangeLatest(
	w http.ResponseWriter, r *http.Request, bill apiv1.BillPath, section apiv1.SectionPath, pRange apiv1.RangePath,
) {
	s.lineRange(w, r, bill, latestWord, section, pRange)
}

func (s *Server) lineRange(w http.ResponseWriter, r *http.Request, bill, n, section, rng string) {
	path, err := parseSection(section)
	if err != nil {
		fail(w, r, err)
		return
	}
	from, to, err := parseRange(rng)
	if err != nil {
		invalid(w, "range must be a line number or an inclusive range such as 12-13")
		return
	}
	res, err := s.resolve(r.Context(), bill, n, true)
	if err != nil {
		fail(w, r, err)
		return
	}
	sec, err := findSection(res.version, path)
	if err != nil {
		fail(w, r, err)
		return
	}
	i := slices.IndexFunc(sec.Lines, func(l data.Line) bool { return l.N == from })
	j := slices.IndexFunc(sec.Lines, func(l data.Line) bool { return l.N == to })
	if i < 0 || j < i || j-i != to-from {
		fail(w, r, fmt.Errorf("lines %d-%d of %s: %w", from, to, sectionID(path), data.ErrNotFound))
		return
	}
	all := toLines(path, sec.Lines)
	canonical := versionPath(res.id, res.n) + "/s/" + sectionID(path) + "/l/" + rng
	out := apiv1.LineRangeResource{
		Bill: res.id, N: res.n, SectionId: sectionID(path), SectionPath: slices.Clone(path),
		From: from, To: to, Licence: licencePublicDomain,
		Address: res.address(canonical, fragmentOf(path, from, to)), Lines: all[i : j+1],
	}
	out.Version, out.AsOf, out.Provenance, out.ContentClass = meta(res.version.Meta)
	sendJSONAt(w, r, res.class(), res.location(canonical), out)
}
