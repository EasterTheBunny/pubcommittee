package billxml_test

import (
	"errors"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/billxml"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

// wantLine is one expected line. SourceID is checked only when non-empty.
// Raw is checked only when non-empty; every non-prose line must carry Raw
// anyway, which checkSectionWant verifies.
type wantLine struct {
	Text         string
	Depth        int
	Kind         string
	Quoted       bool
	Continuation bool
	SourceID     string
	Raw          string
}

// sectionWant is one expected section. The path is always checked; the
// enum, label and heading are checked only when non-empty.
type sectionWant struct {
	path    []string
	enum    string
	label   string
	heading string
	lines   []wantLine
}

// parseCase is one synthetic document and what the parser must return for it.
// kind is checked only when non-empty. warnings holds one substring per
// expected warning; nil means the parser must emit no warnings.
type parseCase struct {
	name     string
	src      string
	kind     string
	sections []sectionWant
	warnings []string
}

// wrapBody wraps a fragment as the legis-body of a minimal bill.
func wrapBody(fragment string) string {
	return "<bill><legis-body>" + fragment + "</legis-body></bill>"
}

// runParseCase parses the case source and checks the document against it.
func runParseCase(t *testing.T, tc parseCase) {
	t.Helper()
	doc, err := billxml.Parse([]byte(tc.src), nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if tc.kind != "" && doc.Kind != tc.kind {
		t.Errorf("Kind = %q, want %q", doc.Kind, tc.kind)
	}
	if len(doc.Sections) != len(tc.sections) {
		t.Fatalf("got %d sections, want %d", len(doc.Sections), len(tc.sections))
	}
	first := 0
	for i, want := range tc.sections {
		checkSectionWant(t, doc.Sections[i], want, first)
		first += len(want.lines)
	}
	checkWarningList(t, doc.Warnings, tc.warnings)
}

// checkSectionWant compares one section. first is the number of lines in
// earlier sections, so N and anchors can be checked against document order.
func checkSectionWant(t *testing.T, got billxml.Section, want sectionWant, first int) {
	t.Helper()
	if !slices.Equal(got.Path, want.path) {
		t.Errorf("Path = %q, want %q", got.Path, want.path)
	}
	if want.enum != "" && got.Enum != want.enum {
		t.Errorf("Enum = %q, want %q", got.Enum, want.enum)
	}
	if want.label != "" && got.Label != want.label {
		t.Errorf("Label = %q, want %q", got.Label, want.label)
	}
	if want.heading != "" && got.Heading != want.heading {
		t.Errorf("Heading = %q, want %q", got.Heading, want.heading)
	}
	if len(got.Lines) != len(want.lines) {
		t.Fatalf("got %d lines, want %d: %+v", len(got.Lines), len(want.lines), got.Lines)
	}
	for i, w := range want.lines {
		checkLine(t, got.Lines[i], w, first+i+1, want.path[0])
	}
}

// checkLine compares one line. n is its document-order number and prefix the
// section path element that begins its anchor.
func checkLine(t *testing.T, g billxml.Line, w wantLine, n int, prefix string) {
	t.Helper()
	checkLineFields(t, g, w)
	checkLineIdentity(t, g, w, n, prefix)
	checkLineRaw(t, g, w)
}

// checkLineFields compares the text, nesting and flags of one line.
func checkLineFields(t *testing.T, g billxml.Line, w wantLine) {
	t.Helper()
	if g.Text != w.Text || g.Depth != w.Depth || g.Kind != w.Kind ||
		g.Quoted != w.Quoted || g.Continuation != w.Continuation {
		t.Errorf("line %d: got {Text:%q Depth:%d Kind:%q Quoted:%t Continuation:%t}, want %+v",
			g.N, g.Text, g.Depth, g.Kind, g.Quoted, g.Continuation, w)
	}
}

// checkLineIdentity compares the number, anchor and source id of one line.
func checkLineIdentity(t *testing.T, g billxml.Line, w wantLine, n int, prefix string) {
	t.Helper()
	if g.N != n {
		t.Errorf("line %q: N = %d, want %d", g.Text, g.N, n)
	}
	if anchor := prefix + "-l" + strconv.Itoa(n); g.Anchor != anchor {
		t.Errorf("line %q: Anchor = %q, want %q", g.Text, g.Anchor, anchor)
	}
	if w.SourceID != "" && g.SourceID != w.SourceID {
		t.Errorf("line %q: SourceID = %q, want %q", g.Text, g.SourceID, w.SourceID)
	}
}

// checkLineRaw compares the raw markup of one line. Every non-prose line
// must carry it.
func checkLineRaw(t *testing.T, g billxml.Line, w wantLine) {
	t.Helper()
	if w.Raw != "" && g.Raw != w.Raw {
		t.Errorf("line %q: Raw = %q, want %q", g.Text, g.Raw, w.Raw)
	}
	nonProse := g.Kind == billxml.KindTable || g.Kind == billxml.KindFormula ||
		g.Kind == billxml.KindGraphic || g.Kind == billxml.KindToc
	if nonProse && g.Raw == "" {
		t.Errorf("line %q: non-prose kind %q has empty Raw", g.Text, g.Kind)
	}
}

// checkWarningList compares the warnings with one expected substring each.
func checkWarningList(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d warnings %q, want %d", len(got), got, len(want))
	}
	for i, sub := range want {
		if !strings.Contains(got[i], sub) {
			t.Errorf("warning %q does not contain %q", got[i], sub)
		}
	}
}

func TestParseSections(t *testing.T) {
	t.Parallel()
	tests := []parseCase{
		{
			name: "nesting with continuation",
			src: wrapBody(`<section id="sa"><enum>1.</enum><header>H</header>` +
				`<text>Intro—</text><paragraph id="p1"><enum>(1)</enum><text>one;</text>` +
				`<subparagraph><enum>(A)</enum><text>deep</text></subparagraph></paragraph>` +
				`<continuation-text>flush end.</continuation-text></section>`),
			sections: []sectionWant{{
				path: []string{"s1"}, enum: "1.", label: "Sec. 1", heading: "H",
				lines: []wantLine{
					{Text: "Intro—", Depth: 0, Kind: billxml.KindText},
					{Text: "(1) one;", Depth: 1, Kind: billxml.KindText, SourceID: "p1"},
					{Text: "(A) deep", Depth: 2, Kind: billxml.KindText},
					{Text: "flush end.", Depth: 0, Kind: billxml.KindText, Continuation: true},
				},
			}},
		},
		{
			name: "quoted block with after-quoted-block punctuation",
			src: wrapBody(`<section><enum>2.</enum><text>Amend—</text><paragraph><enum>(1)</enum>` +
				`<text>by adding:</text><quoted-block><subsection><enum>(c)</enum><text>New</text>` +
				`</subsection><after-quoted-block>.</after-quoted-block></quoted-block></paragraph></section>`),
			sections: []sectionWant{{
				path: []string{"s1"}, enum: "2.", label: "Sec. 2",
				lines: []wantLine{
					{Text: "Amend—", Depth: 0, Kind: billxml.KindText},
					{Text: "(1) by adding:", Depth: 1, Kind: billxml.KindText},
					{Text: "(c) New.", Depth: 2, Kind: billxml.KindText, Quoted: true},
				},
			}},
		},
		{
			name: "list item with quote and fraction",
			src: wrapBody(`<section><enum>1.</enum><text>a</text><list><list-item>x <quote>q</quote>` +
				` 1<fraction>1/2</fraction> y</list-item></list></section>`),
			sections: []sectionWant{{
				path: []string{"s1"}, enum: "1.", label: "Sec. 1",
				lines: []wantLine{
					{Text: "a", Depth: 0, Kind: billxml.KindText},
					{Text: "x “q” 11/2 y", Depth: 1, Kind: billxml.KindText},
				},
			}},
		},
		{
			name: "whitespace in running text is collapsed",
			src:  wrapBody("<section><enum>1.</enum><text>a   b\n c</text></section>"),
			sections: []sectionWant{{
				path: []string{"s1"}, enum: "1.", label: "Sec. 1",
				lines: []wantLine{{Text: "a b c", Depth: 0, Kind: billxml.KindText}},
			}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runParseCase(t, tc)
		})
	}
}

func TestParseUnits(t *testing.T) {
	t.Parallel()
	tests := []parseCase{
		{
			name: "non-prose units keep raw markup",
			src: wrapBody(`<section><enum>3.</enum><text>x</text>` +
				`<table><tgroup><tbody><row><entry>a</entry><entry>b</entry></row></tbody></tgroup></table>` +
				`<formula>A = B</formula><graphic>g</graphic><toc><toc-entry>t1</toc-entry></toc></section>`),
			sections: []sectionWant{{
				path: []string{"s1"}, enum: "3.", label: "Sec. 3",
				lines: []wantLine{
					{Text: "x", Depth: 0, Kind: billxml.KindText},
					{
						Text: "a b", Depth: 1, Kind: billxml.KindTable,
						Raw: `<table><tgroup><tbody><row><entry>a</entry><entry>b</entry></row></tbody></tgroup></table>`,
					},
					{Text: "A = B", Depth: 1, Kind: billxml.KindFormula, Raw: `<formula>A = B</formula>`},
					{Text: "g", Depth: 1, Kind: billxml.KindGraphic, Raw: `<graphic>g</graphic>`},
					{Text: "t1", Depth: 1, Kind: billxml.KindToc, Raw: `<toc><toc-entry>t1</toc-entry></toc>`},
				},
			}},
		},
		{
			name: "division heading becomes a group line",
			src: wrapBody(`<title id="t1"><enum>I</enum><header>Title one</header>` +
				`<section><enum>1.</enum><text>a</text></section></title>`),
			sections: []sectionWant{{
				path: []string{"s1"}, enum: "1.", label: "Sec. 1",
				lines: []wantLine{
					{Text: "I Title one", Depth: 0, Kind: billxml.KindGroup},
					{Text: "a", Depth: 0, Kind: billxml.KindText},
				},
			}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runParseCase(t, tc)
		})
	}
}

func TestParseDocuments(t *testing.T) {
	t.Parallel()
	tests := []parseCase{
		{
			name: "resolution preamble and body",
			src: `<resolution><preamble><text>Whereas x;</text></preamble>` +
				`<resolution-body><text>Resolved, That y.</text></resolution-body></resolution>`,
			kind: "resolution",
			sections: []sectionWant{
				{
					path: []string{"s0"}, heading: "Preamble",
					lines: []wantLine{{Text: "Whereas x;", Depth: 0, Kind: billxml.KindText}},
				},
				{
					path:  []string{"s1"},
					lines: []wantLine{{Text: "Resolved, That y.", Depth: 0, Kind: billxml.KindText}},
				},
			},
		},
		{
			name: "unknown element is kept as text and warned once",
			src:  wrapBody("<section><enum>1.</enum><text>a</text><weird>kept</weird></section>"),
			sections: []sectionWant{{
				path: []string{"s1"}, enum: "1.", label: "Sec. 1",
				lines: []wantLine{
					{Text: "a", Depth: 0, Kind: billxml.KindText},
					{Text: "kept", Depth: 1, Kind: billxml.KindText},
				},
			}},
			warnings: []string{"weird"},
		},
		{
			name: "two legis-body elements number sections in order",
			src: `<bill><legis-body><section><enum>1.</enum><text>a</text></section></legis-body>` +
				`<legis-body><section><enum>1.</enum><text>b</text></section></legis-body></bill>`,
			sections: []sectionWant{
				{
					path: []string{"s1"}, enum: "1.", label: "Sec. 1",
					lines: []wantLine{{Text: "a", Depth: 0, Kind: billxml.KindText}},
				},
				{
					path: []string{"s2"}, enum: "1.", label: "Sec. 1",
					lines: []wantLine{{Text: "b", Depth: 0, Kind: billxml.KindText}},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runParseCase(t, tc)
		})
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()
	deep := strings.Repeat("<a>", 300) + strings.Repeat("</a>", 300)
	tests := []struct {
		name string
		src  string
	}{
		{name: "not xml", src: "not xml"},
		{name: "wrong root element", src: "<other><legis-body/></other>"},
		{name: "no body", src: "<bill></bill>"},
		{name: "unclosed", src: "<bill><legis-body>"},
		{name: "nesting too deep", src: wrapBody(deep)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := billxml.Parse([]byte(tc.src), nil)
			if !errors.Is(err, data.ErrInvalid) {
				t.Errorf("Parse error = %v, want it to wrap data.ErrInvalid", err)
			}
		})
	}
}

func TestBlockOfFixtureSection(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("../../fixtures/bills/us-119-hr-4313/v/rh/source.xml")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	doc, err := billxml.Parse(src, nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	secs := doc.DataSections()
	if len(secs) < 3 {
		t.Fatalf("got %d sections, want at least 3", len(secs))
	}
	sec := secs[2]
	tests := []struct {
		name     string
		n        int
		from, to int
	}{
		{name: "block with nested lines", n: 10, from: 10, to: 22},
		{name: "leaf line", n: 5, from: 5, to: 5},
		{name: "block with one nested line", n: 6, from: 6, to: 7},
		{name: "section heading line", n: 3, from: 3, to: 22},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			from, to, ok := data.Block(sec, tc.n)
			if !ok || from != tc.from || to != tc.to {
				t.Errorf("Block(s3, %d) = (%d, %d, %t), want (%d, %d, true)",
					tc.n, from, to, ok, tc.from, tc.to)
			}
		})
	}
}
