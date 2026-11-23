package billxml_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"strconv"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/billxml"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

// maxSeedBytes keeps fixture seeds small so the fuzzer mutates quickly.
const maxSeedBytes = 20000

// seedInputs returns the fuzz seeds: the small fixture sources, then literal
// snippets that cover the main element classes and edge cases.
func seedInputs(tb testing.TB) [][]byte {
	tb.Helper()
	fsys := os.DirFS("../../fixtures/bills")
	names, err := fs.Glob(fsys, "*/v/*/source.xml")
	if err != nil || len(names) == 0 {
		tb.Fatalf("no fixture seeds: %v", err)
	}
	var seeds [][]byte
	for _, name := range names {
		src, err := fs.ReadFile(fsys, name)
		if err != nil {
			tb.Fatal(err)
		}
		if len(src) < maxSeedBytes {
			seeds = append(seeds, src)
		}
	}
	literals := []string{
		`<bill><legis-body><section><enum>1.</enum><text>a</text></section></legis-body></bill>`,
		`<bill><legis-body><quoted-block><table><row/></table></quoted-block></legis-body></bill>`,
		`<resolution><preamble/><resolution-body/></resolution>`,
		``,
		`<`,
		`<bill>`,
	}
	for _, s := range literals {
		seeds = append(seeds, []byte(s))
	}
	return seeds
}

// checkParse runs the invariants every input must satisfy. Malformed input
// must fail with data.ErrInvalid. Accepted input must marshal, parse
// deterministically, and number its lines in order with consistent anchors.
// Panics are not recovered: they fail the test.
func checkParse(t *testing.T, src []byte) {
	t.Helper()
	doc := parseOrInvalid(t, src)
	if doc == nil {
		return
	}
	checkDeterministic(t, src, doc)
	checkLines(t, doc)
}

// parseOrInvalid parses src. It returns nil when Parse rejects src with an
// error that wraps data.ErrInvalid, and fails the test for any other error.
func parseOrInvalid(t *testing.T, src []byte) *billxml.Document {
	t.Helper()
	doc, err := billxml.Parse(src, nil)
	if err != nil {
		if !errors.Is(err, data.ErrInvalid) {
			t.Fatalf("Parse error does not wrap data.ErrInvalid: %v", err)
		}
		return nil
	}
	return doc
}

// checkDeterministic marshals doc, parses src again, and requires the second
// marshal output to match byte for byte.
func checkDeterministic(t *testing.T, src []byte, doc *billxml.Document) {
	t.Helper()
	out, err := billxml.Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal of accepted input failed: %v", err)
	}
	again, err := billxml.Parse(src, nil)
	if err != nil {
		t.Fatalf("second Parse of same bytes failed: %v", err)
	}
	out2, err := billxml.Marshal(again)
	if err != nil {
		t.Fatalf("second Marshal failed: %v", err)
	}
	if !bytes.Equal(out, out2) {
		t.Fatalf("Parse is not deterministic: marshal output differs between runs")
	}
}

// checkLines requires lines numbered 1..n in document order, anchors of the
// form section-path[0] + "-l" + n, non-negative depth, and some content.
func checkLines(t *testing.T, doc *billxml.Document) {
	t.Helper()
	n := 0
	for _, s := range doc.Sections {
		if len(s.Path) == 0 {
			t.Fatalf("section has empty Path")
		}
		for _, l := range s.Lines {
			n++
			if l.N != n {
				t.Fatalf("line numbered %s, want %s", strconv.Itoa(l.N), strconv.Itoa(n))
			}
			want := s.Path[0] + "-l" + strconv.Itoa(n)
			if l.Anchor != want {
				t.Fatalf("line %s has Anchor %q, want %q", strconv.Itoa(n), l.Anchor, want)
			}
			if l.Depth < 0 {
				t.Fatalf("line %s has negative Depth %s", strconv.Itoa(n), strconv.Itoa(l.Depth))
			}
			if l.Text == "" && l.Raw == "" {
				t.Fatalf("line %s has neither Text nor Raw", strconv.Itoa(n))
			}
		}
	}
}

// FuzzParse feeds arbitrary bytes to Parse and checks the invariants in
// checkParse. Run: go test ./internal/billxml/ -run xxx -fuzz FuzzParse.
func FuzzParse(f *testing.F) {
	for _, seed := range seedInputs(f) {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, src []byte) {
		checkParse(t, src)
	})
}

// TestFuzzSeedsParse runs the same checks over the seeds without fuzzing.
func TestFuzzSeedsParse(t *testing.T) {
	t.Parallel()
	for i, src := range seedInputs(t) {
		t.Run("seed-"+strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			checkParse(t, src)
		})
	}
}
