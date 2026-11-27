package data

import "slices"

// ExplanationSpan returns the lines of s an explanation is pinned to, from the
// first to the last of ExplanationSpans. Both are 0 for a section with no lines.
func (s Section) ExplanationSpan(anchors []string, provisionID string, links []CitationLink) (from, to int) {
	runs := s.ExplanationSpans(anchors, provisionID, links)
	if len(runs) == 0 {
		return 0, 0
	}
	return runs[0][0], runs[len(runs)-1][1]
}

// ExplanationSpans returns the line runs ([from, to] pairs, ascending, apart
// from each other) an explanation is pinned to. In order of preference:
//   - the lines of s whose citation links name provisionID. The explanation is
//     about that provision, so the lines that cite it say best what it covers;
//     the model's anchors do not, because explanations of one section cite the
//     same shared lines (M1-10b measurement). Each line stands alone: a cited
//     line is not extended to the lines nested under it, which would take in
//     its siblings' text and, for a cited lead-in, the whole subsection;
//   - the lines named by anchors (as stored with a new explanation), with the
//     section's lead-in line (its first line, when lines nest under it) left
//     out when the anchors name other lines;
//   - the whole section.
//
// Touching runs are merged. A link or anchor naming a line the section does
// not hold is ignored. A section with no lines has no runs.
func (s Section) ExplanationSpans(anchors []string, provisionID string, links []CitationLink) [][2]int {
	if len(s.Lines) == 0 {
		return nil
	}
	runs := s.citationRuns(provisionID, links)
	if len(runs) == 0 {
		runs = s.anchorRuns(anchors)
	}
	if len(runs) == 0 {
		return [][2]int{{s.Lines[0].N, s.Lines[len(s.Lines)-1].N}}
	}
	return mergeRuns(runs)
}

// anchorRuns lists the runs named by the anchors that belong to s.
func (s Section) anchorRuns(anchors []string) [][2]int {
	var spans [][2]int
	for _, a := range anchors {
		if sec, lo, hi, ok := ParseAnchorID(a); ok && slices.Equal(sec, s.Path) && s.hasLine(lo) && s.hasLine(hi) {
			spans = append(spans, [2]int{lo, hi})
		}
	}
	return s.withoutLeadIn(spans)
}

// withoutLeadIn drops the lead-in line from spans, when spans name other lines.
func (s Section) withoutLeadIn(spans [][2]int) [][2]int {
	lead := s.leadIn()
	if lead == 0 {
		return spans
	}
	var rest [][2]int
	for _, sp := range spans {
		switch {
		case sp[0] == lead && sp[1] == lead:
			// A bare citation of the lead-in.
		case sp[0] == lead:
			rest = append(rest, [2]int{s.nextLine(lead), sp[1]})
		default:
			rest = append(rest, sp)
		}
	}
	if len(rest) == 0 {
		return spans
	}
	return rest
}

// citationRuns lists the lines of s that cite provisionID.
func (s Section) citationRuns(provisionID string, links []CitationLink) [][2]int {
	if provisionID == "" {
		return nil
	}
	var spans [][2]int
	for _, l := range links {
		if slices.Equal(l.Section, s.Path) && slices.Contains(l.ProvisionIDs, provisionID) && s.hasLine(l.Line) {
			spans = append(spans, [2]int{l.Line, l.Line})
		}
	}
	return spans
}

// leadIn is the number of the first line when other lines nest under it, else 0.
func (s Section) leadIn() int {
	if len(s.Lines) > 1 && s.Lines[1].Depth > s.Lines[0].Depth {
		return s.Lines[0].N
	}
	return 0
}

// nextLine is the number of the line after line n, or n when it is the last.
func (s Section) nextLine(n int) int {
	for i, l := range s.Lines {
		if l.N == n && i+1 < len(s.Lines) {
			return s.Lines[i+1].N
		}
	}
	return n
}

// mergeRuns sorts runs and joins those that overlap or touch.
func mergeRuns(runs [][2]int) [][2]int {
	slices.SortFunc(runs, func(a, b [2]int) int { return a[0] - b[0] })
	out := [][2]int{runs[0]}
	for _, r := range runs[1:] {
		last := &out[len(out)-1]
		if r[0] <= last[1]+1 {
			last[1] = max(last[1], r[1])
			continue
		}
		out = append(out, r)
	}
	return out
}
