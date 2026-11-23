package citations

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// Raw is one reference found in a bill version, before resolution. Which
// fields are set depends on Kind.
type Raw struct {
	Kind    string   // data.CiteUSC, CiteUSCChapter, CitePublicLaw, CiteActSection, or "other"
	Source  string   // data.SourceMarkup or data.SourceText
	Section []string // bill section path holding the line
	Line    int      // line number N
	Text    string   // as written in the bill

	Title   int      // usc, usc-chapter
	USCSect string   // usc: normalised section ("1395cc-7")
	Path    []string // usc, act-section: subdivision path as cited, e.g. ["a", "1"]
	EtSeq   bool     // usc: the cite says "et seq."
	Chapter string   // usc-chapter

	PublicLaw string // public-law: "111-163"

	Act        string // act-section: the Act as written, e.g. "Social Security Act"
	ActSection string // act-section: the section number as written, e.g. "1866G"
	// PairedWith is, for an act-section, the index in the result of the
	// markup citation that follows it in parentheses: that citation says what
	// the section is.
	PairedWith int
}

const uscPrefix = "usc" // first element of a provision id and of a parsable-cite

// NormalizeSection returns a section number as stored: the dashes of the
// sources (en dash, em dash, non-breaking hyphen, figure dash) become hyphens.
func NormalizeSection(s string) string {
	return strings.NewReplacer("\u2013", "-", "\u2014", "-", "\u2011", "-", "\u2012", "-").Replace(strings.TrimSpace(s))
}

// entities are the character entities bill XML uses without a DTD.
func entities() map[string]string {
	return map[string]string{
		"nbsp": " ", "mdash": "\u2014", "ndash": "\u2013", "sect": "\u00a7", "para": "\u00b6",
		"ldquo": "\u201c", "rdquo": "\u201d", "lsquo": "\u2018", "rsquo": "\u2019", "hellip": "\u2026",
	}
}

// xref is one external-xref element found in the source XML.
type xref struct {
	legalDoc string
	cite     string
	text     string
	ids      []string // ids of the enclosing elements, innermost last
}

// xrefScan is the state of scanXrefs.
type xrefScan struct {
	out   []xref
	stack []string // id of each open element ("" when none)
	open  int      // depth of the open xref, or -1
	cur   xref
	text  strings.Builder
}

func (x *xrefScan) start(t xml.StartElement) {
	var id, legal, cite string
	for _, a := range t.Attr {
		switch a.Name.Local {
		case "id":
			id = a.Value
		case "legal-doc":
			legal = a.Value
		case "parsable-cite":
			cite = a.Value
		}
	}
	x.stack = append(x.stack, id)
	if t.Name.Local != "external-xref" || x.open >= 0 {
		return
	}
	x.open = len(x.stack)
	x.cur = xref{legalDoc: legal, cite: cite}
	for _, s := range x.stack[:len(x.stack)-1] {
		if s != "" {
			x.cur.ids = append(x.cur.ids, s)
		}
	}
	x.text.Reset()
}

func (x *xrefScan) end() {
	if x.open == len(x.stack) {
		x.cur.text = collapse(x.text.String())
		x.out = append(x.out, x.cur)
		x.open = -1
	}
	x.stack = x.stack[:len(x.stack)-1]
}

// scanXrefs reads the source XML and returns its external-xref elements in
// document order.
func scanXrefs(src []byte) ([]xref, error) {
	dec := xml.NewDecoder(bytes.NewReader(src))
	dec.Strict = false
	dec.Entity = entities()
	dec.CharsetReader = func(_ string, in io.Reader) (io.Reader, error) { return in, nil }
	x := &xrefScan{open: -1}
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return x.out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("citations: scan source: %w: %w", data.ErrInvalid, err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			x.start(t)
		case xml.EndElement:
			x.end()
		case xml.CharData:
			if x.open >= 0 {
				x.text.Write(t)
			}
		}
	}
}

var spaces = regexp.MustCompile(`\s+`)

func collapse(s string) string { return strings.TrimSpace(spaces.ReplaceAllString(s, " ")) }

// lineRef is a line with the path of its bill section.
type lineRef struct {
	sec  []string
	line data.Line
}

// index holds the lines of a version for locating citations.
type index struct {
	all  []lineRef
	byID map[string][]lineRef
}

func indexLines(v data.OfficialVersion) index {
	ix := index{byID: map[string][]lineRef{}}
	for _, s := range v.Sections {
		for _, l := range s.Lines {
			lr := lineRef{s.Path, l}
			ix.all = append(ix.all, lr)
			if l.SourceID != "" {
				ix.byID[l.SourceID] = append(ix.byID[l.SourceID], lr)
			}
		}
	}
	return ix
}

// locate finds the line holding the xref: the line of the innermost
// enclosing element whose text holds the xref text, else any line that does.
func (ix index) locate(x xref) (lineRef, bool) {
	for _, id := range slices.Backward(x.ids) {
		for _, lr := range ix.byID[id] {
			if strings.Contains(collapse(lr.line.Text), x.text) {
				return lr, true
			}
		}
	}
	for _, lr := range ix.all {
		if strings.Contains(collapse(lr.line.Text), x.text) {
			return lr, true
		}
	}
	return lineRef{}, false
}

// found is a raw citation with its offset in the collapsed line text.
type found struct {
	r     Raw
	start int
}

// markupByLine places every xref on its line, with offsets. Repeated texts
// on one line take successive occurrences.
func markupByLine(xs []xref, ix index) map[int][]found {
	out := map[int][]found{}
	used := map[int]int{} // line N -> search offset for repeated texts
	for _, x := range xs {
		lr, ok := ix.locate(x)
		if !ok {
			continue
		}
		txt := collapse(lr.line.Text)
		from := min(used[lr.line.N], len(txt))
		at := strings.Index(txt[from:], x.text)
		if at < 0 {
			at = strings.Index(txt, x.text)
		} else {
			at += from
		}
		used[lr.line.N] = at + len(x.text)
		r := rawFromXref(x)
		r.Section, r.Line = lr.sec, lr.line.N
		out[lr.line.N] = append(out[lr.line.N], found{r, at})
	}
	return out
}

// Extract finds the references to existing law in one version. Markup
// (external-xref elements of the source XML) comes first; text patterns find
// what the markup does not cover. The result is in document order.
func Extract(v data.OfficialVersion, src []byte) ([]Raw, error) {
	xs, err := scanXrefs(src)
	if err != nil {
		return nil, err
	}
	ix := indexLines(v)
	marked := markupByLine(xs, ix)
	var out []Raw
	for _, lr := range ix.all {
		out = appendLine(out, lr, marked[lr.line.N])
	}
	return out, nil
}

// appendLine adds the citations of one line to out in text order, and pairs
// an Act-section phrase with the markup citation that follows it in
// parentheses: that citation says what the section is.
func appendLine(out []Raw, lr lineRef, marked []found) []Raw {
	txt := collapse(lr.line.Text)
	items := slices.Clone(marked)
	cov := make([][2]int, 0, len(marked))
	for _, m := range marked {
		cov = append(cov, [2]int{m.start, m.start + len(m.r.Text)})
	}
	for _, m := range textMatches(txt, cov) {
		m.r.Section, m.r.Line = lr.sec, lr.line.N
		items = append(items, m)
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].start < items[j].start })
	base := len(out)
	for i, it := range items {
		it.r.PairedWith = -1
		if i+1 < len(items) && pairs(txt, it, items[i+1]) {
			it.r.PairedWith = base + i + 1
		}
		out = append(out, it.r)
	}
	return out
}

// pairs reports whether a is an Act-section phrase directly followed by "("
// and the markup US Code citation b.
func pairs(txt string, a, b found) bool {
	end := a.start + len(a.r.Text)
	return a.r.Kind == data.CiteActSection && a.r.Source == data.SourceText &&
		b.r.Source == data.SourceMarkup && b.r.Kind == data.CiteUSC &&
		end <= b.start && b.start <= len(txt) && strings.TrimSpace(txt[end:b.start]) == "("
}

var (
	pathPattern    = regexp.MustCompile(`^((?:\([A-Za-z0-9]+\))+)`)
	pathElemRe     = regexp.MustCompile(`\(([A-Za-z0-9]+)\)`)
	etSeqPattern   = regexp.MustCompile(`^\s*,?\s*et seq\.?`)
	uscTextPattern = regexp.MustCompile(
		`\b(\d{1,2}) U\.S\.C\. (?:\x{a7}\s*)?(\d+[A-Za-z]*(?:[-\x{2013}\x{2014}]\d+[A-Za-z]*)*)((?:\([A-Za-z0-9]+\))*)`)
	plPattern  = regexp.MustCompile(`\b(?:Public Law|Pub\. L\.) (\d+)[-\x{2013}\x{2014}](\d+)`)
	titleSecRe = regexp.MustCompile(
		`\b[Ss]ections? (\d+[A-Za-z]*(?:[-\x{2013}\x{2014}]\d+[A-Za-z]*)*)((?:\([A-Za-z0-9]+\))*) ` +
			`of title (\d+),? United States Code`)
	actSecRe = regexp.MustCompile(
		`\b[Ss]ections? (\d+[A-Za-z]*(?:[-\x{2013}\x{2014}]\d+[A-Za-z]*)*)((?:\([A-Za-z0-9]+\))*) of the ` +
			`((?:[A-Z][A-Za-z.,'\x{2019}&-]*)(?: (?:[A-Z][A-Za-z.,'\x{2019}&-]*|of|and|for|the))* ` +
			`(?:Act|Code)(?: of \d{4})?)`)
)

func pathOf(parens string) []string {
	ms := pathElemRe.FindAllStringSubmatch(parens, -1)
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m[1])
	}
	return out
}

// typicalCites is the capacity a line starts with for the citations found in its text.
const typicalCites = 4

// minParts is the number of parts of a parsable-cite that name a section,
// such as "usc/42/1395cc-7".
const minParts = 3

func rawFromXref(x xref) Raw {
	r := Raw{Source: data.SourceMarkup, Text: x.text}
	parts := strings.Split(x.cite, "/")
	switch x.legalDoc {
	case "usc":
		r.Kind = data.CiteUSC
		fillUSC(&r, parts)
	case "usc-chapter":
		r.Kind = data.CiteUSCChapter
		if len(parts) >= minParts {
			r.Title, _ = strconv.Atoi(parts[1])
			r.Chapter = parts[2]
		}
	case "public-law":
		r.Kind = data.CitePublicLaw
		if len(parts) >= minParts {
			r.PublicLaw = parts[1] + "-" + parts[2]
		}
	default:
		r.Kind = "other"
	}
	return r
}

// fillUSC reads the title, section and path of a "usc/<title>/<section>[/path]"
// cite; a cite without a path takes it from the text of the xref.
func fillUSC(r *Raw, parts []string) {
	if len(parts) >= minParts && parts[0] == uscPrefix {
		r.Title, _ = strconv.Atoi(parts[1])
		r.USCSect = NormalizeSection(parts[2])
		r.Path = parts[minParts:]
	}
	if r.Title > 0 && len(r.Path) == 0 {
		r.Path, r.EtSeq = pathFromText(r.Text, r.USCSect)
	}
}

// pathFromText reads the subdivision path that follows the section number in
// the text of a cite, and whether "et seq." follows.
func pathFromText(text, sec string) (path []string, etSeq bool) {
	txt := NormalizeSection(text)
	_, after, ok := strings.Cut(txt, sec)
	if !ok {
		return nil, false
	}
	rest := after
	if m := pathPattern.FindString(rest); m != "" {
		path = pathOf(m)
		rest = rest[len(m):]
	}
	return path, etSeqPattern.MatchString(rest)
}

func overlaps(cov [][2]int, a, b int) bool {
	for _, c := range cov {
		if a < c[1] && c[0] < b {
			return true
		}
	}
	return false
}

// textMatches finds references in text that no markup citation covers.
func textMatches(txt string, cov [][2]int) []found {
	out := make([]found, 0, typicalCites)
	add := func(loc []int, r Raw) {
		if overlaps(cov, loc[0], loc[1]) {
			return
		}
		r.Source, r.Text = data.SourceText, txt[loc[0]:loc[1]]
		out = append(out, found{r, loc[0]})
	}
	for _, loc := range uscTextPattern.FindAllStringSubmatchIndex(txt, -1) {
		title, _ := strconv.Atoi(txt[loc[2]:loc[3]])
		r := Raw{
			Kind: data.CiteUSC, Title: title, USCSect: NormalizeSection(txt[loc[4]:loc[5]]),
			Path: pathOf(txt[loc[6]:loc[7]]), EtSeq: etSeqPattern.MatchString(txt[loc[1]:]),
		}
		add(loc[:2], r)
	}
	for _, loc := range plPattern.FindAllStringSubmatchIndex(txt, -1) {
		add(loc[:2], Raw{Kind: data.CitePublicLaw, PublicLaw: txt[loc[2]:loc[3]] + "-" + txt[loc[4]:loc[5]]})
	}
	for _, loc := range titleSecRe.FindAllStringSubmatchIndex(txt, -1) {
		title, _ := strconv.Atoi(txt[loc[6]:loc[7]])
		add(loc[:2], Raw{
			Kind: data.CiteUSC, Title: title, USCSect: NormalizeSection(txt[loc[2]:loc[3]]), Path: pathOf(txt[loc[4]:loc[5]]),
		})
	}
	for _, loc := range actSecRe.FindAllStringSubmatchIndex(txt, -1) {
		add(loc[:2], Raw{
			Kind: data.CiteActSection, Act: txt[loc[6]:loc[7]],
			ActSection: NormalizeSection(txt[loc[2]:loc[3]]), Path: pathOf(txt[loc[4]:loc[5]]),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].start < out[j].start })
	return out
}
