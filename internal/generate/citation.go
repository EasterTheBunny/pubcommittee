package generate

import (
	"regexp"
	"strings"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// citationRE matches a US Code citation as written in prose or in a provision id the model composed from the
// text: "42 U.S.C. 1395cc–7(a)(1)". The groups are the title, the section and the subsection suffix.
var citationRE = regexp.MustCompile(
	`(\d+)\s+U\.S\.C\.\s+` + // title
		`(\d+[A-Za-z0-9]*(?:[-‐‑‒–—−][A-Za-z0-9]+)*)` + // section, with dash look-alikes inside
		`((?:\([A-Za-z0-9]+\))*)`) // subsection suffix

// hyphenated maps the dash look-alikes of the printed Code to the hyphen of the stored ids.
func hyphenated(s string) string {
	return strings.NewReplacer("‐", "-", "‑", "-", "‒", "-", "–", "-", "—", "-", "−", "-").Replace(s)
}

// citationID converts a citation ("42 U.S.C. 1395cc–7(a)(1)") to the stored provision id form
// ("usc/42/1395cc-7/a/1"). ok is false when s is not exactly one citation.
func citationID(s string) (string, bool) {
	s = strings.TrimSpace(s)
	m := citationRE.FindStringSubmatchIndex(s)
	if m == nil || m[0] != 0 || m[1] != len(s) {
		return "", false
	}
	return idOf(s[m[2]:m[3]], s[m[4]:m[5]], s[m[6]:m[7]]), true
}

// idOf builds the stored id from the parts of a citation; each "(x)" of the suffix becomes a path element.
func idOf(title, section, suffix string) string {
	var id strings.Builder
	id.WriteString("usc/" + title + "/" + hyphenated(section))
	for p := range strings.SplitSeq(suffix, ")") {
		if p = strings.TrimPrefix(p, "("); p != "" {
			id.WriteString("/" + p)
		}
	}
	return id.String()
}

// lookupProvision finds the provision a statement cites. id is a stored provision id or a citation of it in
// printed form. A citation naming a subsection that the input holds only as a whole section ("1395cc-7(a)(1)" with
// the stored "usc/42/1395cc-7") resolves to that section: the cited text is part of what the input holds. A
// citation broader than the stored provision does not resolve.
func lookupProvision(gc data.GenContext, id string) (text string, found bool) {
	if t, ok := gc.Provisions[id]; ok {
		return t, true
	}
	norm, ok := citationID(id)
	if !ok {
		norm = hyphenated(id)
	}
	for ; strings.Count(norm, "/") >= 2; norm = norm[:strings.LastIndex(norm, "/")] {
		if t, ok := gc.Provisions[norm]; ok {
			return t, true
		}
	}
	return "", false
}

// withoutCitations returns text with every US Code citation that names a provision of the input removed, so
// that the title and section numbers a statement writes for a provision it describes are not read as facts.
func withoutCitations(text string, gc data.GenContext) string {
	return citationRE.ReplaceAllStringFunc(text, func(c string) string {
		if _, ok := lookupProvision(gc, c); ok {
			return " "
		}
		return c
	})
}
