package uslm

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// rootPattern matches the identifier of a title file. Appendix titles such as
// "/us/usc/t5a" do not match and are rejected.
var rootPattern = regexp.MustCompile(`^/us/usc/t([1-9][0-9]*)$`)

// idPattern is the shape of a normalised section id.
var idPattern = regexp.MustCompile(`^[0-9A-Za-z]+(-[0-9A-Za-z]+)*$`)

// ParseTitle streams a US Code title file and calls fn once per top-level section, in document order.
// release supplies the release date. An error from fn stops the parse and is returned unchanged.
func ParseTitle(src []byte, release data.Release, fn func(data.LawSection) error) error {
	dec := xml.NewDecoder(bytes.NewReader(src))
	dec.Strict = true
	title, err := readRoot(dec)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(src)
	doc := document{title: title, release: release, sha: hex.EncodeToString(sum[:])}
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return malformed(err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "section" {
			continue
		}
		id, ok := doc.sectionID(attr(start, "identifier"))
		if !ok {
			continue
		}
		root, err := buildNode(dec, start)
		if err != nil {
			return err
		}
		if err := fn(doc.lawSection(root, id)); err != nil {
			return err
		}
	}
}

// document holds what every section of one title file shares.
type document struct {
	title   int
	release data.Release
	sha     string // SHA-256 of the whole source, lowercase hex
}

// sectionID returns the normalised id of a top-level section with this raw
// identifier, and false for anything that is not a top-level section.
func (d document) sectionID(identifier string) (string, bool) {
	rest, ok := strings.CutPrefix(identifier, "/us/usc/t"+strconv.Itoa(d.title)+"/s")
	if !ok || strings.Contains(rest, "/") {
		return "", false
	}
	id := normaliseDashes(rest)
	return id, idPattern.MatchString(id)
}

// lawSection builds the record for one section tree.
func (d document) lawSection(root *node, id string) data.LawSection {
	meta := data.Meta{
		Version:      1,
		AsOf:         d.release.Date,
		Provenance:   data.ProvenanceSource,
		ContentClass: data.ContentOfficial,
	}
	b := sectionBuilder{title: d.title, id: id, prefix: root.identifier + "/", release: d.release, meta: meta}
	provisions := []data.Provision{{
		ID:          data.ProvisionID(d.title, id),
		Citation:    b.citation(nil),
		Text:        root.text(),
		CurrentAsOf: d.release.Date,
		Meta:        meta,
	}}
	return data.LawSection{
		Ref:          data.ProvisionRef{Release: d.release.ID, Title: d.title, Section: id},
		Heading:      root.heading(),
		Status:       root.status,
		SourceSHA256: d.sha,
		Provisions:   b.collect(root, provisions),
		Meta:         meta,
	}
}

// sectionBuilder collects the subdivisions of one section.
type sectionBuilder struct {
	title   int
	id      string
	prefix  string // raw identifier of the section plus "/"
	release data.Release
	meta    data.Meta
}

// collect appends every subdivision below n in document order: a parent before its children.
func (b sectionBuilder) collect(n *node, out []data.Provision) []data.Provision {
	for _, p := range n.parts {
		c := p.el
		if c == nil {
			continue
		}
		if isSubdivision(c.name) {
			if path, ok := b.path(c.identifier); ok {
				out = append(out, data.Provision{
					ID:          data.ProvisionID(b.title, b.id, path...),
					Citation:    b.citation(path),
					Text:        c.text(),
					CurrentAsOf: b.release.Date,
					Meta:        b.meta,
				})
			}
		}
		out = b.collect(c, out)
	}
	return out
}

// path returns the normalised segments after the section identifier, and false
// when the identifier is not below this section.
func (b sectionBuilder) path(identifier string) ([]string, bool) {
	rest, ok := strings.CutPrefix(identifier, b.prefix)
	if !ok {
		return nil, false
	}
	segs := strings.Split(rest, "/")
	for i, seg := range segs {
		if seg == "" {
			return nil, false
		}
		segs[i] = normaliseDashes(seg)
	}
	return segs, true
}

// citation returns "42 U.S.C. 1395cc-7(a)(2)(A)" for path ["a", "2", "A"].
func (b sectionBuilder) citation(path []string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d U.S.C. %s", b.title, b.id)
	for _, seg := range path {
		sb.WriteString("(" + seg + ")")
	}
	return sb.String()
}

// node is one element of the section tree, kept with its character data in document order.
type node struct {
	name       string
	identifier string
	status     string
	parts      []part
}

// part is either character data (el is nil) or a child element.
type part struct {
	text string
	el   *node
}

// buildNode reads the element that starts with start, and its content, into a node.
// Notes, sourceCredit and toc are read past and not kept.
func buildNode(dec *xml.Decoder, start xml.StartElement) (*node, error) {
	n := &node{name: start.Name.Local, identifier: attr(start, "identifier"), status: attr(start, "status")}
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, malformed(err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if isExcluded(t.Name.Local) {
				if err := dec.Skip(); err != nil {
					return nil, malformed(err)
				}
				continue
			}
			child, err := buildNode(dec, t)
			if err != nil {
				return nil, err
			}
			n.parts = append(n.parts, part{el: child})
		case xml.CharData:
			n.parts = append(n.parts, part{text: string(t)})
		case xml.EndElement:
			return n, nil
		}
	}
}

// text returns the element's text with block elements separated by spaces and
// white space collapsed.
func (n *node) text() string {
	var b strings.Builder
	n.writeText(&b)
	return strings.Join(strings.Fields(b.String()), " ")
}

func (n *node) writeText(b *strings.Builder) {
	block := isBlock(n.name)
	if block {
		b.WriteByte(' ')
	}
	for _, p := range n.parts {
		if p.el != nil {
			p.el.writeText(b)
		} else {
			b.WriteString(p.text)
		}
	}
	if block {
		b.WriteByte(' ')
	}
}

// heading returns the text of the direct child heading element.
func (n *node) heading() string {
	for _, p := range n.parts {
		if p.el != nil && p.el.name == "heading" {
			return p.el.text()
		}
	}
	return ""
}

// readRoot reads up to the root element and returns the title number from its identifier.
func readRoot(dec *xml.Decoder) (int, error) {
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return 0, fmt.Errorf("uslm: no root element: %w", data.ErrInvalid)
		}
		if err != nil {
			return 0, malformed(err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != "uscDoc" {
			return 0, fmt.Errorf("uslm: root element <%s> is not uscDoc: %w", start.Name.Local, data.ErrInvalid)
		}
		identifier := attr(start, "identifier")
		m := rootPattern.FindStringSubmatch(identifier)
		if m == nil {
			return 0, fmt.Errorf("uslm: root identifier %q is not /us/usc/t<N>: %w", identifier, data.ErrInvalid)
		}
		title, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, fmt.Errorf("uslm: root identifier %q: %w: %w", identifier, data.ErrInvalid, err)
		}
		return title, nil
	}
}

func malformed(err error) error {
	return fmt.Errorf("uslm: malformed XML: %w: %w", data.ErrInvalid, err)
}

// attr returns the value of the attribute with this local name, or "".
func attr(start xml.StartElement, name string) string {
	for _, a := range start.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

// normaliseDashes replaces en dash, em dash and non-breaking hyphen with hyphen-minus.
func normaliseDashes(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '–', '—', '‑':
			return '-'
		}
		return r
	}, s)
}

// isExcluded reports whether an element's content is never part of any provision's text.
func isExcluded(name string) bool {
	switch name {
	case "notes", "note", "sourceCredit", "toc":
		return true
	}
	return false
}

// isSubdivision reports whether an element can be a provision below a section.
func isSubdivision(name string) bool {
	switch name {
	case "subsection", "paragraph", "subparagraph", "clause", "subclause", "item", "subitem", "subsubitem":
		return true
	}
	return false
}

// isBlock reports whether an element separates its text from its neighbours.
func isBlock(name string) bool {
	switch name {
	case "section", "subsection", "paragraph", "subparagraph", "clause", "subclause", "item",
		"subitem", "subsubitem", "num", "heading", "content", "chapeau", "continuation", "p",
		"proviso", "quotedContent", "tr", "th", "td", "br", "layout", "column":
		return true
	}
	return false
}
