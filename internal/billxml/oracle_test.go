package billxml_test

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/billxml"
)

// oracle independently flattens the text of the bill body (legis-body,
// resolution-body, preamble) so tests can check that the parser dropped
// nothing. Whitespace is removed before comparing; <quote> gets curly quotes
// as in the parser; amendment XML (out of scope) is skipped.
type oracle struct {
	b    strings.Builder
	in   int // depth inside a body, 0 outside
	skip int // depth inside skipped amendment XML
}

func (o *oracle) start(name string) {
	switch {
	case o.skip > 0 || (o.in == 0 && (name == "amendment-doc" || name == "engrossed-amendment-form")):
		o.skip++
	case o.in > 0:
		o.in++
		if name == "quote" {
			o.b.WriteString("“")
		}
	case name == "legis-body" || name == "resolution-body" || name == "preamble":
		o.in = 1
	}
}

func (o *oracle) end(name string) {
	switch {
	case o.skip > 0:
		o.skip--
	case o.in > 0:
		if name == "quote" {
			o.b.WriteString("”")
		}
		o.in--
	}
}

func bodyText(t *testing.T, src []byte) string {
	t.Helper()
	//nolint:gosec // G709: the test oracle reads committed public-domain fixtures
	dec := xml.NewDecoder(bytes.NewReader(src))
	dec.Entity = map[string]string{"nbsp": " ", "mdash": "—", "ndash": "–", "sect": "§"}
	var o oracle
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("oracle: %v", err)
		}
		switch e := tok.(type) {
		case xml.StartElement:
			o.start(e.Name.Local)
		case xml.EndElement:
			o.end(e.Name.Local)
		case xml.CharData:
			if o.in > 0 && o.skip == 0 {
				o.b.Write(e)
			}
		}
	}
	return strings.Join(strings.Fields(o.b.String()), "")
}

// parsedText concatenates everything the parser kept, in document order.
func parsedText(d *billxml.Document) string {
	var b strings.Builder
	for _, s := range d.Sections {
		// Group headings (title, division, ...) sit at the start of the next
		// section's lines but precede that section's own enum and heading.
		i := 0
		for ; i < len(s.Lines) && s.Lines[i].Kind == billxml.KindGroup; i++ {
			b.WriteString(s.Lines[i].Text)
		}
		b.WriteString(s.Enum + s.Heading)
		for _, l := range s.Lines[i:] {
			b.WriteString(l.Text)
		}
	}
	return strings.Join(strings.Fields(b.String()), "")
}

// firstDiff describes where two strings first differ.
func firstDiff(got, want string) string {
	r, w := []rune(got), []rune(want)
	i := 0
	for i < len(r) && i < len(w) && r[i] == w[i] {
		i++
	}
	lo := max(i-40, 0)
	return string(r[lo:min(i+60, len(r))]) + " | source " + string(w[lo:min(i+60, len(w))])
}
