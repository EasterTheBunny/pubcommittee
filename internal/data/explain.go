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

func (s Section) hasLine(n int) bool {
	return slices.ContainsFunc(s.Lines, func(l Line) bool { return l.N == n })
}

// ExplanationsToShow drops the explanations of sections of v that need none
// (NeedsExplanation), such as one stored before planning skipped short titles.
// An explanation whose section v does not hold is kept. read-api and the
// change-explanations export both use it, so they serve the same set.
func ExplanationsToShow(v OfficialVersion, xs []ChangeExplanation) []ChangeExplanation {
	return slices.DeleteFunc(xs, func(e ChangeExplanation) bool {
		for _, sec := range v.Sections {
			if slices.Equal(sec.Path, e.Section) {
				return !sec.NeedsExplanation()
			}
		}
		return false
	})
}
