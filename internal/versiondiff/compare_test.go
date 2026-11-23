package versiondiff_test

import (
	"reflect"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/versiondiff"
)

// sec builds a section whose lines are numbered from first, depth 0.
func sec(ord int, label, heading string, first int, texts ...string) data.Section {
	s := data.Section{Path: []string{"s" + itoa(ord)}, Label: label, Heading: heading}
	for i, t := range texts {
		s.Lines = append(s.Lines, data.Line{N: first + i, Text: t})
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

func classes(t *testing.T, c []data.SectionComparison, path string) []data.LineClass {
	t.Helper()
	for _, s := range c {
		if s.Path[0] == path && s.Status != data.SectionAdded {
			out := make([]data.LineClass, len(s.Lines))
			for i, l := range s.Lines {
				out[i] = l.Class
			}
			return out
		}
	}
	t.Fatalf("no section %s", path)
	return nil
}

func want(t *testing.T, got []data.LineClass, w ...data.LineClass) {
	t.Helper()
	if !reflect.DeepEqual(got, w) {
		t.Errorf("classes = %v, want %v", got, w)
	}
}

const (
	un = data.LineUnchanged
	mv = data.LineMoved
	ch = data.LineChanged
	rm = data.LineRemoved
)

func TestWhitespaceIsNotAChange(t *testing.T) {
	t.Parallel()
	a := []data.Section{sec(1, "SEC. 1.", "Short title", 1, "This Act may be cited as the Example Act.")}
	b := []data.Section{sec(1, "SEC. 1.", "Short title", 1, "  This Act may   be cited as the Example Act.\n")}
	want(t, classes(t, versiondiff.CompareSections(a, b), "s1"), un)
}

func TestPunctuationAndWordingAreChanges(t *testing.T) {
	t.Parallel()
	a := []data.Section{sec(1, "SEC. 1.", "X", 1,
		"The Secretary shall issue rules within 90 days.",
		"The Secretary may waive fees for small entities.")}
	b := []data.Section{sec(1, "SEC. 1.", "X", 1,
		"The Secretary shall issue rules within 180 days.",
		"The Secretary may waive fees, for small entities.")}
	want(t, classes(t, versiondiff.CompareSections(a, b), "s1"), ch, ch)
}

func TestRenumberedIdenticalSubsectionIsMoved(t *testing.T) {
	t.Parallel()
	a := []data.Section{sec(1, "SEC. 1.", "Rules", 1,
		"(a) First duty text stays the same.",
		"(b) Second duty text stays the same.",
		"(c) Third duty text stays the same.")}
	// A new subsection (a) is inserted, so old (a)..(c) become (b)..(d).
	b := []data.Section{sec(1, "SEC. 1.", "Rules", 1,
		"(a) A brand new opening duty that has nothing in common.",
		"(b) First duty text stays the same.",
		"(c) Second duty text stays the same.",
		"(d) Third duty text stays the same.")}
	got := versiondiff.CompareSections(a, b)
	want(t, classes(t, got, "s1"), mv, mv, mv)
	if got[0].Status != data.SectionChanged {
		t.Errorf("status = %s, want changed", got[0].Status)
	}
	if loc := got[0].Lines[0].New; loc == nil || loc.N != 2 {
		t.Errorf("new location of old line 1 = %+v, want line 2", loc)
	}
}

func TestRenumberedSectionIsMoved(t *testing.T) {
	t.Parallel()
	a := []data.Section{
		sec(1, "SEC. 1.", "One", 1, "Alpha alpha alpha."),
		sec(2, "SEC. 2.", "Two", 2, "Beta beta beta."),
	}
	b := []data.Section{
		sec(1, "SEC. 1.", "One", 1, "Alpha alpha alpha."),
		sec(2, "SEC. 2.", "Inserted", 2, "Gamma gamma gamma."),
		sec(3, "SEC. 3.", "Two", 3, "Beta beta beta."),
	}
	got := versiondiff.CompareSections(a, b)
	want(t, classes(t, got, "s1"), un)
	want(t, classes(t, got, "s2"), mv)
	if got[0].Status != data.SectionUnchanged {
		t.Errorf("s1 status = %s", got[0].Status)
	}
	if got[len(got)-1].Status != data.SectionAdded {
		t.Errorf("last status = %s, want added", got[len(got)-1].Status)
	}
}

func TestDeletedSectionAndChangedVersusRemoved(t *testing.T) {
	t.Parallel()
	a := []data.Section{
		sec(1, "SEC. 1.", "Keep", 1, "The Secretary shall report to Congress each year."),
		sec(2, "SEC. 2.", "Drop", 2, "Funds are authorized for fiscal year 2026 only."),
	}
	b := []data.Section{
		sec(1, "SEC. 1.", "Keep", 1, "The Secretary shall report to Congress each quarter."),
	}
	got := versiondiff.CompareSections(a, b)
	want(t, classes(t, got, "s1"), ch)
	want(t, classes(t, got, "s2"), rm)
	if got[1].Status != data.SectionRemoved {
		t.Errorf("s2 status = %s", got[1].Status)
	}
}

func TestSplitSectionAndMergedParagraphs(t *testing.T) {
	t.Parallel()
	a := []data.Section{
		sec(1, "SEC. 1.", "Big", 1, "Line one is here.", "Line two is here.", "Line three is here."),
	}
	b := []data.Section{
		sec(1, "SEC. 1.", "Big", 1, "Line one is here."),
		sec(2, "SEC. 2.", "Split off", 2, "Line two is here.", "Line three is here."),
	}
	got := versiondiff.CompareSections(a, b)
	want(t, classes(t, got, "s1"), un, mv, mv)
	if n := got[0].Lines[1].New; n == nil || n.Section[0] != "s2" || n.N != 2 {
		t.Errorf("split line new = %+v", n)
	}
	// Merged paragraphs: two old lines become one longer new line.
	c := []data.Section{sec(1, "SEC. 1.", "Big", 1, "Line one is here. Line two is here.")}
	got = versiondiff.CompareSections([]data.Section{sec(1, "SEC. 1.", "Big", 1, "Line one is here.", "Line two is here.")}, c)
	cl := classes(t, got, "s1")
	if cl[0] == un || cl[1] == un {
		t.Errorf("merged classes = %v, want no unchanged", cl)
	}
}

func TestDeterministic(t *testing.T) {
	t.Parallel()
	a := []data.Section{sec(1, "SEC. 1.", "X", 1, "a b c d", "e f g h", "a b c e", "q r s t")}
	b := []data.Section{sec(1, "SEC. 1.", "X", 1, "a b c e", "a b c d", "e f g x", "z")}
	first := versiondiff.CompareSections(a, b)
	for range 20 {
		if got := versiondiff.CompareSections(a, b); !reflect.DeepEqual(got, first) {
			t.Fatal("comparison is not deterministic")
		}
	}
}

func TestLookupRanges(t *testing.T) {
	t.Parallel()
	a := []data.Section{sec(1, "SEC. 1.", "X", 1,
		"(a) Same words here.", "(b) Other words there.", "(c) Gone entirely forever.")}
	b := []data.Section{sec(1, "SEC. 1.", "X", 1,
		"(a) Same words here.", "(b) Other words there today.")}
	c := data.Comparison{Sections: versiondiff.CompareSections(a, b)}
	cases := []struct {
		from, to int
		class    data.LineClass
		newFrom  int
	}{
		{1, 1, un, 1},
		{2, 2, ch, 2},
		{3, 3, rm, 0},
		{1, 2, ch, 1},
		{1, 3, ch, 1},
	}
	for _, tc := range cases {
		r, err := c.Lookup([]string{"s1"}, tc.from, tc.to)
		if err != nil {
			t.Fatal(err)
		}
		if r.Class != tc.class {
			t.Errorf("%d-%d: class = %s, want %s", tc.from, tc.to, r.Class, tc.class)
		}
		if (r.New == nil) != (tc.newFrom == 0) || (r.New != nil && r.New.From != tc.newFrom) {
			t.Errorf("%d-%d: new = %+v", tc.from, tc.to, r.New)
		}
	}
	if _, err := c.Lookup([]string{"s9"}, 1, 1); err == nil {
		t.Error("unknown section: want error")
	}
	if _, err := c.Lookup([]string{"s1"}, 1, 9); err == nil {
		t.Error("unknown line: want error")
	}
}

func TestMovedRange(t *testing.T) {
	t.Parallel()
	a := []data.Section{
		sec(1, "SEC. 1.", "A", 1, "Para alpha text.", "Para beta text."),
		sec(2, "SEC. 2.", "B", 3, "Para gamma text."),
	}
	b := []data.Section{
		sec(1, "SEC. 1.", "A", 1, "Para gamma text."),
		sec(2, "SEC. 2.", "B", 2, "Para alpha text.", "Para beta text."),
	}
	c := data.Comparison{Sections: versiondiff.CompareSections(a, b)}
	r, err := c.Lookup([]string{"s1"}, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if r.Class != mv || r.New == nil || r.New.Section[0] != "s2" || r.New.From != 2 || r.New.To != 3 {
		t.Errorf("moved range = %+v", r)
	}
}

func TestPairs(t *testing.T) {
	t.Parallel()
	got := versiondiff.Pairs(4)
	w := [][2]int{{0, 1}, {1, 2}, {2, 3}, {0, 3}, {1, 3}}
	if !reflect.DeepEqual(got, w) {
		t.Errorf("Pairs(4) = %v, want %v", got, w)
	}
	if len(versiondiff.Pairs(1)) != 0 {
		t.Error("Pairs(1) not empty")
	}
}
