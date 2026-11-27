package data_test

import (
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

func lines(texts ...string) []data.Line {
	out := make([]data.Line, len(texts))
	for i, t := range texts {
		out[i] = data.Line{N: i + 3, Text: t}
	}
	return out
}

func TestNeedsExplanation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		sec  data.Section
		want bool
	}{
		{"short title", data.Section{Lines: lines("This Act may be cited as the “Example Act”.")}, false},
		{"short title with heading line", data.Section{Lines: lines("(a) Short title This Act may be cited as the “Example Act of 2025”.")}, false},
		{"short title and table of contents", data.Section{Lines: lines(
			"(a) Short title This Act may be cited as the “Example Act”.",
			"(b) Table of contents The table of contents for this Act is as follows:",
		)}, true},
		{"other provision", data.Section{Lines: lines("The Secretary shall publish a report.")}, true},
		{"no lines", data.Section{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := c.sec.NeedsExplanation(); got != c.want {
				t.Fatalf("NeedsExplanation = %v, want %v", got, c.want)
			}
		})
	}
}

func TestProvisionCitation(t *testing.T) {
	t.Parallel()
	for id, want := range map[string]string{
		"usc/16/824a/f":   "16 U.S.C. 824a(f)",
		"usc/42/1395":     "42 U.S.C. 1395",
		"usc/15/717b/c/1": "15 U.S.C. 717b(c)(1)",
		"usc/3":           "usc/3",
		"other/1/2":       "other/1/2",
		"":                "",
	} {
		if got := data.ProvisionCitation(id); got != want {
			t.Errorf("ProvisionCitation(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestExplanationSpan(t *testing.T) {
	t.Parallel()
	sec := data.Section{Path: []string{"s2"}, Lines: lines("a", "b", "c", "d", "e")} // lines 3..7
	links := []data.CitationLink{
		{Section: []string{"s2"}, Line: 5, ProvisionIDs: []string{"usc/16/824a/f"}},
		{Section: []string{"s2"}, Line: 6, ProvisionIDs: []string{"usc/16/824a/f"}},
		{Section: []string{"s9"}, Line: 4, ProvisionIDs: []string{"usc/16/824a/f"}},
		{Section: []string{"s2"}, Line: 99, ProvisionIDs: []string{"usc/16/824a/f"}},
	}
	cases := []struct {
		name     string
		anchors  []string
		prov     string
		wantFrom int
		wantTo   int
	}{
		{"citation lines win over anchors", []string{"s2-l4", "s2-l6-7"}, "usc/16/824a/f", 5, 6},
		{"stored anchors when no line cites the provision", []string{"s2-l4", "s2-l6-7"}, "usc/1/1", 4, 7},
		{"anchors of another section ignored", []string{"s9-l4"}, "usc/16/824a/f", 5, 6},
		{"citation lines", nil, "usc/16/824a/f", 5, 6},
		{"no citing line: whole section", nil, "usc/1/1", 3, 7},
		{"no provision: whole section", nil, "", 3, 7},
		{"anchor beyond the section ignored", []string{"s2-l40"}, "", 3, 7},
	}
	for _, c := range cases {
		from, to := sec.ExplanationSpan(c.anchors, c.prov, links)
		if from != c.wantFrom || to != c.wantTo {
			t.Errorf("%s: got %d-%d, want %d-%d", c.name, from, to, c.wantFrom, c.wantTo)
		}
	}
	if f, to := (data.Section{}).ExplanationSpan(nil, "", nil); f != 0 || to != 0 {
		t.Errorf("empty section: got %d-%d", f, to)
	}
}

func TestExplanationsToShow(t *testing.T) {
	t.Parallel()
	v := data.OfficialVersion{Sections: []data.Section{
		{Path: []string{"s1"}, Lines: []data.Line{{Text: "This Act may be cited as the “Example Act”."}}},
		{Path: []string{"s2"}, Lines: []data.Line{{Text: "The Secretary shall publish a report."}}},
	}}
	xs := []data.ChangeExplanation{{Section: []string{"s1"}}, {Section: []string{"s2"}}, {Section: []string{"s9"}}}
	got := data.ExplanationsToShow(v, xs)
	if len(got) != 2 || got[0].Section[0] != "s2" || got[1].Section[0] != "s9" {
		t.Fatalf("ExplanationsToShow kept %v, want s2 and s9", got)
	}
}
