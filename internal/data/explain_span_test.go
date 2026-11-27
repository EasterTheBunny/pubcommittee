package data_test

import (
	"slices"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// hr3062Section has the depths of section 2 of H.R. 3062, lines 2 to 45.
func hr3062Section() data.Section {
	depths := []int{
		0, 1, 1, 2, 2, 3, 3, 2, 3, 3, 1, 2, 2, 2, 3, 3, 1, 2, 2, 2, // 2..21: (a)
		0, 0, 1, 1, 2, 2, 0, 0, 1, 1, 1, 0, 0, 1, 1, 2, 2, 0, 1, 1, 1, 1, 1, 1, // 22..45: (b)..(h)
	}
	sec := data.Section{Path: []string{"s2"}}
	for i, d := range depths {
		sec.Lines = append(sec.Lines, data.Line{N: i + 2, Depth: d, Text: "line"})
	}
	return sec
}

// hr3062Links are lines of section 2 that cite one provision each (as measured on the rfs version).
func hr3062Links() []data.CitationLink {
	cite := map[string]int{
		"usc/15/717a": 42, "usc/15/717b/c": 22, "usc/16/796": 45, "usc/16/824a/e": 24,
		"usc/16/824a/f": 26, "usc/49/60502": 21,
	}
	out := make([]data.CitationLink, 0, len(cite)+2)
	for id, n := range cite {
		out = append(out, data.CitationLink{Section: []string{"s2"}, Line: n, ProvisionIDs: []string{id}})
	}
	// One line that cites two provisions, and a line of another section.
	out = append(out,
		data.CitationLink{Section: []string{"s2"}, Line: 33, ProvisionIDs: []string{"usc/3/301", "usc/15/717b"}},
		data.CitationLink{Section: []string{"s9"}, Line: 5, ProvisionIDs: []string{"usc/16/796"}})
	return out
}

// hr3062SharedAnchors are the lines every stored explanation of the section cites (staging measurement).
func hr3062SharedAnchors() []string {
	return []string{"s2-l3", "s2-l5-8", "s2-l22", "s2-l24", "s2-l28", "s2-l33", "s2-l35"}
}

func TestExplanationSpansHR3062(t *testing.T) {
	t.Parallel()
	sec, links := hr3062Section(), hr3062Links()
	cases := []struct {
		prov    string
		anchors []string
		want    [][2]int
	}{
		{"usc/16/824a/f", append([]string{"s2-l23-26"}, hr3062SharedAnchors()...), [][2]int{{26, 26}}},
		{"usc/16/796", append([]string{"s2-l9-11", "s2-l45"}, hr3062SharedAnchors()...), [][2]int{{45, 45}}},
		{"usc/49/60502", append([]string{"s2-l21", "s2-l40"}, hr3062SharedAnchors()...), [][2]int{{21, 21}}},
		{"usc/15/717b", hr3062SharedAnchors(), [][2]int{{33, 33}}},
		{"usc/3/301", hr3062SharedAnchors(), [][2]int{{33, 33}}},
		// No line cites the provision: the anchors, with the lead-in (line 2) not among them.
		{"usc/42/1", []string{"s2-l2", "s2-l33", "s2-l45"}, [][2]int{{33, 33}, {45, 45}}},
		{"usc/42/1", []string{"s2-l22", "s2-l23"}, [][2]int{{22, 23}}},
		{"usc/42/1", []string{"s2-l2"}, [][2]int{{2, 2}}},
		{"usc/42/1", []string{"s2-l2-4"}, [][2]int{{3, 4}}},
		{"usc/42/1", []string{"s9-l5", "s2-l99"}, [][2]int{{2, 45}}},
		{"", nil, [][2]int{{2, 45}}},
	}
	seen := map[int]string{}
	for _, c := range cases {
		got := sec.ExplanationSpans(c.anchors, c.prov, links)
		if !slices.Equal(got, c.want) {
			t.Errorf("%s %v: got %v, want %v", c.prov, c.anchors, got, c.want)
		}
		from, to := sec.ExplanationSpan(c.anchors, c.prov, links)
		if from != c.want[0][0] || to != c.want[len(c.want)-1][1] {
			t.Errorf("%s: span %d-%d", c.prov, from, to)
		}
		if from == to && c.prov != "usc/42/1" && c.prov != "usc/3/301" {
			if other, dup := seen[from]; dup {
				t.Errorf("%s and %s share line %d", c.prov, other, from)
			}
			seen[from] = c.prov
		}
	}
	if got := (data.Section{}).ExplanationSpans([]string{"s2-l2"}, "usc/16/796", links); got != nil {
		t.Errorf("empty section: got %v", got)
	}
}

func TestExplanationSpansScatteredCitations(t *testing.T) {
	t.Parallel()
	sec := hr3062Section()
	links := []data.CitationLink{
		{Section: []string{"s2"}, Line: 40, ProvisionIDs: []string{"usc/1/1"}},
		{Section: []string{"s2"}, Line: 5, ProvisionIDs: []string{"usc/1/1"}},
		{Section: []string{"s2"}, Line: 6, ProvisionIDs: []string{"usc/1/1"}},
		{Section: []string{"s2"}, Line: 99, ProvisionIDs: []string{"usc/1/1"}},
	}
	want := [][2]int{{5, 6}, {40, 40}}
	if got := sec.ExplanationSpans(nil, "usc/1/1", links); !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if from, to := sec.ExplanationSpan(nil, "usc/1/1", links); from != 5 || to != 40 {
		t.Errorf("span %d-%d", from, to)
	}
}
