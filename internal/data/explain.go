package data

import (
	"slices"
	"strconv"
	"strings"
)

// NeedsExplanation reports whether an explanation of s would add anything. A
// section whose every line only names the Act ("This Act may be cited as ...")
// says all there is to say, so it gets none; a short title that sits with a
// table of contents or other provisions is still explained. Planning and the
// read side both use this rule (operator feedback, design/tasks/m1/FEEDBACK.md
// item 2 and the follow-up that hides explanations stored earlier).
func (s Section) NeedsExplanation() bool {
	if len(s.Lines) == 0 {
		return false
	}
	for _, l := range s.Lines {
		if !strings.Contains(strings.ToLower(strings.TrimSpace(l.Text)), " may be cited as ") {
			return true
		}
	}
	return false
}

// ProvisionCitation writes a provision id the way people cite it:
// "usc/16/824a/f" is "16 U.S.C. 824a(f)". An id of another form is returned as it is.
func ProvisionCitation(id string) string {
	title, section, path, err := ParseProvisionID(id)
	if err != nil {
		return id
	}
	var b strings.Builder
	b.WriteString(strconv.Itoa(title) + " U.S.C. " + section)
	for _, p := range path {
		b.WriteString("(" + p + ")")
	}
	return b.String()
}

// ExplanationSpan returns the lines of s an explanation is pinned to. In order:
// the lines named by anchors (as stored with a new explanation); the lines of s
// that cite provisionID, from links; the whole section. Both are 0 for a
// section with no lines.
func (s Section) ExplanationSpan(anchors []string, provisionID string, links []CitationLink) (from, to int) {
	if len(s.Lines) == 0 {
		return 0, 0
	}
	for _, sp := range s.candidateSpans(anchors, provisionID, links) {
		if !s.hasLine(sp[0]) || !s.hasLine(sp[1]) {
			continue
		}
		if from == 0 || sp[0] < from {
			from = sp[0]
		}
		to = max(to, sp[1])
	}
	if from == 0 {
		return s.Lines[0].N, s.Lines[len(s.Lines)-1].N
	}
	return from, to
}

// candidateSpans lists the line spans named by the anchors that belong to s; with none, the lines of s that
// cite provisionID.
func (s Section) candidateSpans(anchors []string, provisionID string, links []CitationLink) [][2]int {
	var spans [][2]int
	for _, a := range anchors {
		if sec, lo, hi, ok := ParseAnchorID(a); ok && slices.Equal(sec, s.Path) {
			spans = append(spans, [2]int{lo, hi})
		}
	}
	if len(spans) > 0 || provisionID == "" {
		return spans
	}
	for _, l := range links {
		if slices.Equal(l.Section, s.Path) && slices.Contains(l.ProvisionIDs, provisionID) {
			spans = append(spans, [2]int{l.Line, l.Line})
		}
	}
	return spans
}

func (s Section) hasLine(n int) bool {
	return slices.ContainsFunc(s.Lines, func(l Line) bool { return l.N == n })
}
