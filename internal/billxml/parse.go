package billxml

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// Element names used in more than one place.
const (
	elEnum   = "enum"
	elHeader = "header"
	elQuote  = "quote"
	elText   = "text"
	elQBlock = "quoted-block"
	elQCont  = "quoted-block-continuation-text"
)

// class says how the walker treats a child element of a level.
type class int

const (
	clsUnknown class = iota
	clsLevel
	clsQuotedBlock
	clsContinuation
	clsAfterQuoted
	clsList
	clsUnit
	clsText
	clsSkip // page breaks and inline elements already part of the head line
)

//nolint:gochecknoglobals // read-only element tables
var (
	// levelNames are structural elements that carry their own head line.
	levelNames = set("section", "subsection", "paragraph", "subparagraph", "clause", "subclause",
		"item", "subitem", "subsubitem", "appropriations-major", "appropriations-intermediate",
		"appropriations-small", "list-item", "division", "title", "subtitle", "chapter", "subchapter",
		"part", "subpart")
	groupNames = set("division", "title", "subtitle", "chapter", "subchapter", "part", "subpart")
	unitKinds  = map[string]string{
		"table": KindTable, "toc": KindToc, "multi-column-toc": KindToc,
		"formula": KindFormula, "graphic": KindGraphic,
	}
	// inlineNames are elements that may appear inside running text.
	inlineNames = set("term", "short-title", "external-xref", "header-in-text", "italic", "bold",
		"smallcap", "superscript", "subscript", "underline", "strike", "fraction", "inline-equation",
		elEnum, elHeader, "sponsor", "cosponsor", "committee-name", "act-name", "bill-name",
		elQuote, "pagebreak", "linebreak", "nobreak", "em-dash-in-text", "toc-entry", "toc-enum",
		"toc-header", "toc-level", "toc-item", "internal-xref", "enum-in-header", "level-header",
		"editorial", "target", "proviso", "added-phrase", "ttitle", "toc-quoted-entry",
		"multi-column-toc-entry")
)

func set(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

//nolint:gochecknoglobals // read-only element table
var namedClass = map[string]class{
	elQBlock: clsQuotedBlock, "continuation-text": clsContinuation, "after-quoted-block": clsAfterQuoted,
	"list": clsList, elText: clsText, elQCont: clsText, "pagebreak": clsSkip,
}

func classify(name string) class {
	if c, ok := namedClass[name]; ok {
		return c
	}
	switch {
	case levelNames[name]:
		return clsLevel
	case unitKinds[name] != "":
		return clsUnit
	case inlineNames[name]:
		return clsSkip
	}
	return clsUnknown
}

// Parse parses the original XML bytes of one official version. log may be
// nil. Errors wrap data.ErrInvalid.
func Parse(src []byte, log *slog.Logger) (*Document, error) {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	root, err := buildTree(src)
	if err != nil {
		return nil, fmt.Errorf("billxml: %w: %w", err, data.ErrInvalid)
	}
	top, bodies, err := findBodies(root)
	if err != nil {
		return nil, err
	}
	w := &walker{src: src, log: log, warned: map[string]bool{}}
	doc := w.header(top, src)
	if pre := top.child("preamble"); pre != nil {
		w.openSection([]string{"s0"}, "", "", "Preamble", pre.attr("id"))
		w.children(pre, 0, false)
		w.cur = nil // text after the preamble starts its own section
	}
	for _, b := range bodies {
		w.body(b)
	}
	for _, k := range top.elements() {
		if k.name == "amendment-doc" || k.name == "engrossed-amendment-form" {
			w.warn("<" + k.name + "> (amendment XML) not parsed")
		}
	}
	w.flushPending()
	w.finish()
	doc.Sections = w.sections
	doc.Warnings = w.warnings
	if doc.Warnings == nil {
		doc.Warnings = []string{}
	}
	return doc, nil
}

// findBodies returns the bill or resolution element and its body elements.
func findBodies(root *node) (*node, []*node, error) {
	els := root.elements()
	if len(els) == 0 || (els[0].name != "bill" && els[0].name != "resolution") {
		return nil, nil, fmt.Errorf("billxml: document element is not bill or resolution: %w", data.ErrInvalid)
	}
	var bodies []*node
	for _, k := range els[0].elements() {
		if k.name == "legis-body" || k.name == "resolution-body" {
			bodies = append(bodies, k)
		}
	}
	if len(bodies) == 0 {
		return nil, nil, fmt.Errorf("billxml: no legis-body or resolution-body: %w", data.ErrInvalid)
	}
	return els[0], bodies, nil
}

// header fills the document fields that come from the bill element and form.
func (w *walker) header(top *node, src []byte) *Document {
	sum := sha256.Sum256(src)
	doc := &Document{
		ParserVersion: ParserVersion,
		SourceSHA256:  hex.EncodeToString(sum[:]),
		Kind:          top.name,
		Stage:         top.attr("bill-stage"),
		SourceID:      top.attr("dms-id"),
	}
	if form := top.child("form"); form != nil {
		if n := form.child("legis-num"); n != nil {
			doc.LegisNum = w.inline(n)
		}
		if n := form.child("official-title"); n != nil {
			doc.OfficialTitle = w.inline(n)
		}
	}
	return doc
}

type walker struct {
	src      []byte
	log      *slog.Logger
	sections []Section
	cur      *Section // section receiving lines; nil before the first
	pending  []Line   // group headings and loose text waiting for the next section
	ordinal  int
	warnings []string
	warned   map[string]bool
}

func (w *walker) warn(msg string) {
	if w.warned[msg] {
		return
	}
	w.warned[msg] = true
	w.warnings = append(w.warnings, msg)
	w.log.Warn("billxml", slog.String("warning", msg))
}

func (w *walker) openSection(path []string, enum, label, heading, id string) {
	w.sections = append(w.sections, Section{
		Path: path, Enum: enum, Label: label, Heading: heading, SourceID: id, Lines: []Line{},
	})
	w.cur = &w.sections[len(w.sections)-1]
	w.cur.Lines = append(w.cur.Lines, w.pending...)
	w.pending = nil
}

// flushPending gives group headings or loose text that follow the last
// section a section of their own, so no text is dropped.
func (w *walker) flushPending() {
	if len(w.pending) == 0 {
		return
	}
	w.ordinal++
	w.openSection([]string{"s" + strconv.Itoa(w.ordinal)}, "", "", "", "")
}

// emit adds a line to the current section, or to pending while group
// headings or loose text wait for the next section.
func (w *walker) emit(l Line) {
	if l.Text == "" && l.Raw == "" {
		return // an element with no text of its own, for example only a page break
	}
	if w.cur == nil || len(w.pending) > 0 {
		w.pending = append(w.pending, l)
		return
	}
	w.cur.Lines = append(w.cur.Lines, l)
}

func (w *walker) text(k *node, depth int, quoted, continuation bool) {
	w.emit(Line{
		Depth: depth, Text: w.inline(k), SourceID: k.attr("id"), Kind: KindText,
		Quoted: quoted, Continuation: continuation,
	})
}

// finish numbers the lines in document order and derives anchors.
func (w *walker) finish() {
	n := 0
	for i := range w.sections {
		s := &w.sections[i]
		for j := range s.Lines {
			n++
			s.Lines[j].N = n
			s.Lines[j].Anchor = s.Path[0] + "-l" + strconv.Itoa(n)
		}
	}
}

func (w *walker) body(body *node) {
	for _, e := range body.elements() {
		switch {
		case e.name == "section":
			w.section(e)
		case groupNames[e.name]:
			w.group(e)
		default:
			w.child(e, 0, 0, false, false)
		}
	}
}

// group emits the heading of a division, title, ... and walks its contents.
func (w *walker) group(e *node) {
	if head := w.head(e); head != "" {
		w.pending = append(w.pending, Line{Depth: 0, Text: head, SourceID: e.attr("id"), Kind: KindGroup})
	}
	for _, k := range e.elements() {
		switch {
		case k.name == "section":
			w.section(k)
		case groupNames[k.name]:
			w.group(k)
		case k.name != elEnum && k.name != elHeader:
			w.child(k, 0, 0, false, false)
		}
	}
}

// head joins the enum and header of a level element.
func (w *walker) head(e *node) string {
	var parts []string
	for _, name := range []string{elEnum, elHeader} {
		if k := e.child(name); k != nil {
			if t := w.inline(k); t != "" {
				parts = append(parts, t)
			}
		}
	}
	return strings.Join(parts, " ")
}

// section opens a Section for a top-level section element.
func (w *walker) section(e *node) {
	for _, l := range w.pending {
		if l.Kind != KindGroup { // loose text before this section keeps its place in a section of its own
			w.flushPending()
			break
		}
	}
	w.ordinal++
	enum, label, heading := "", "", ""
	if k := e.child(elEnum); k != nil {
		enum = w.inline(k)
		label = sectionLabel(enum)
	}
	if k := e.child(elHeader); k != nil {
		heading = w.inline(k)
	}
	w.openSection([]string{"s" + strconv.Itoa(w.ordinal)}, enum, label, heading, e.attr("id"))
	w.levelBody(e, 0, false, true)
}

func sectionLabel(enum string) string {
	t := strings.TrimSpace(enum)
	if len(t) >= 4 && strings.EqualFold(t[:4], "sec.") {
		t = strings.TrimSpace(t[4:])
	}
	t = strings.TrimRight(t, ".")
	if t == "" {
		return ""
	}
	return "Sec. " + t
}

// levelHead builds the head line text of a level element. A section's enum
// and header are its label and heading, not line text.
func (w *walker) levelHead(e, first *node, isSection bool) string {
	var head string
	switch {
	case isSection && first != nil:
		head = w.inline(first)
	case !isSection:
		head = w.head(e)
		if first != nil {
			head = join(head, w.inline(first))
		}
	}
	return join(head, w.directText(e))
}

// levelBody emits the head line of a level element and walks its children.
// Children nest one level deeper when the element has a head line.
func (w *walker) levelBody(e *node, depth int, quoted, isSection bool) {
	first := e.child(elText)
	head := w.levelHead(e, first, isSection)
	childDepth := depth
	if head != "" {
		w.emit(Line{Depth: depth, Text: head, SourceID: e.attr("id"), Kind: KindText, Quoted: quoted})
		childDepth = depth + 1
	}
	for _, k := range e.elements() {
		switch k.name {
		case elEnum, elHeader:
		case elText:
			if k != first {
				w.text(k, depth, quoted, false)
			}
		default:
			w.child(k, depth, childDepth, quoted, head != "")
		}
	}
}

// directText returns the running text that sits directly in an element:
// character data and inline elements, without structural children (list
// items carry their text without a text child).
func (w *walker) directText(e *node) string {
	var b strings.Builder
	for _, k := range e.kids {
		switch {
		case k.name == "":
			b.WriteString(k.text)
		case k.name == elEnum || k.name == elHeader:
		case inlineNames[k.name]:
			w.flatten(&node{kids: []*node{k}}, &b, false)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// children walks the element children of a container that has no head line.
func (w *walker) children(e *node, depth int, quoted bool) {
	for _, k := range e.elements() {
		w.child(k, depth, depth, quoted, false)
	}
}

// child handles one element inside a level. depth is the parent's own depth;
// childDepth is where nested content goes.
func (w *walker) child(k *node, depth, childDepth int, quoted, hasHead bool) {
	switch c := classify(k.name); c {
	case clsLevel:
		w.levelBody(k, childDepth, quoted, false)
	case clsQuotedBlock:
		w.quotedBlock(k, childDepth)
	case clsList:
		for _, li := range k.elements() {
			w.child(li, depth, childDepth, quoted, hasHead)
		}
	case clsUnit:
		w.unit(k, childDepth, quoted)
	case clsContinuation, clsAfterQuoted, clsText, clsUnknown, clsSkip:
		w.childText(c, k, depth, childDepth, quoted, hasHead)
	}
}

// childText handles the text-like classes of child.
func (w *walker) childText(c class, k *node, depth, childDepth int, quoted, hasHead bool) {
	switch c {
	case clsContinuation:
		w.text(k, depth, quoted, hasHead)
	case clsAfterQuoted:
		w.appendToLast(w.inline(k))
	case clsText:
		w.text(k, childDepth, quoted, false)
	case clsUnknown:
		w.warn("unknown element <" + k.name + "> kept as text")
		w.text(k, childDepth, quoted, false)
	case clsLevel, clsQuotedBlock, clsList, clsUnit, clsSkip:
		// Level, quoted-block, list and unit are handled by child; skip is
		// a page break or an inline element already part of the head line.
	}
}

// quotedBlock walks text inserted into existing law: same line rules, with
// every line flagged Quoted, nested under the line that introduced the block.
func (w *walker) quotedBlock(e *node, depth int) {
	for _, k := range e.elements() {
		w.child(k, depth, depth, true, false)
	}
}

// unit emits a table, formula, graphic or table of contents as one line.
func (w *walker) unit(k *node, depth int, quoted bool) {
	w.emit(Line{
		Depth: depth, Text: w.spaced(k), SourceID: k.attr("id"), Kind: unitKinds[k.name],
		Quoted: quoted, Raw: string(w.src[k.start:k.end]),
	})
}

// appendToLast adds closing punctuation (after-quoted-block) to the last line.
func (w *walker) appendToLast(t string) {
	if t == "" {
		return
	}
	var ls *[]Line
	switch {
	case len(w.pending) > 0:
		ls = &w.pending
	case w.cur != nil && len(w.cur.Lines) > 0:
		ls = &w.cur.Lines
	default:
		w.warn("after-quoted-block with no preceding line")
		return
	}
	last := &(*ls)[len(*ls)-1]
	last.Text += t
}

func join(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + " " + b
}

// inline flattens an element to text: character data in document order,
// whitespace collapsed, <quote> wrapped in curly quotes, page breaks dropped.
func (w *walker) inline(n *node) string {
	var b strings.Builder
	w.flatten(n, &b, false)
	return strings.Join(strings.Fields(b.String()), " ")
}

// spaced flattens a table-like unit, separating nested elements with spaces
// so adjacent cells do not run together.
func (w *walker) spaced(n *node) string {
	var b strings.Builder
	w.flatten(n, &b, true)
	return strings.Join(strings.Fields(b.String()), " ")
}

func (w *walker) flatten(n *node, b *strings.Builder, spaced bool) {
	for _, k := range n.kids {
		switch k.name {
		case "":
			b.WriteString(k.text)
		case "pagebreak":
		case "linebreak", "nobreak":
			b.WriteByte(' ')
		case elQuote:
			b.WriteString("“")
			w.flatten(k, b, spaced)
			b.WriteString("”")
		default:
			w.flattenElement(k, b, spaced)
		}
	}
}

func (w *walker) flattenElement(k *node, b *strings.Builder, spaced bool) {
	if !spaced && !inlineNames[k.name] && unitKinds[k.name] == "" {
		w.warn("unknown inline element <" + k.name + "> kept as text")
	}
	if spaced {
		b.WriteByte(' ')
	}
	w.flatten(k, b, spaced)
	if spaced {
		b.WriteByte(' ')
	}
}
