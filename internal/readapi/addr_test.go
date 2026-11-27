package readapi

import (
	"errors"
	"slices"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

func TestParseBillID(t *testing.T) {
	t.Parallel()

	valid := []struct {
		name string
		in   string
		want data.BillRef
	}{
		{
			name: "house bill",
			in:   "us-119-hr-4127",
			want: data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: 4127},
		},
		{
			name: "senate bill of an earlier session",
			in:   "us-118-s-1",
			want: data.BillRef{Jurisdiction: "us", Session: "118", Type: "s", Number: 1},
		},
	}
	for _, tc := range valid {
		t.Run("valid "+tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseBillID(tc.in)
			if err != nil {
				t.Fatalf("parseBillID(%q): unexpected error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("parseBillID(%q) = %+v, want %+v", tc.in, got, tc.want)
			}
			if id := billID(got); id != tc.in {
				t.Fatalf("billID(parseBillID(%q)) = %q, want the input back", tc.in, id)
			}
		})
	}

	bad := []struct {
		name string
		in   string
	}{
		{name: "empty", in: ""},
		{name: "missing number", in: "us-119-hr"},
		{name: "upper case jurisdiction", in: "US-119-hr-1"},
		{name: "zero number", in: "us-119-hr-0"},
		{name: "non-numeric number", in: "us-119-hr-x"},
		{name: "extra segment", in: "us-119-hr-1-2"},
	}
	for _, tc := range bad {
		t.Run("bad "+tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseBillID(tc.in)
			if !errors.Is(err, data.ErrNotFound) {
				t.Fatalf("parseBillID(%q): error %v, want ErrNotFound", tc.in, err)
			}
		})
	}
}

func TestParseSection(t *testing.T) {
	t.Parallel()

	valid := []struct {
		name string
		in   string
		want []string
	}{
		{name: "top level", in: "s4", want: []string{"s4"}},
		{name: "nested", in: "s4.b", want: []string{"s4", "b"}},
	}
	for _, tc := range valid {
		t.Run("valid "+tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseSection(tc.in)
			if err != nil {
				t.Fatalf("parseSection(%q): unexpected error: %v", tc.in, err)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("parseSection(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}

	t.Run("section id is dotted path", func(t *testing.T) {
		t.Parallel()
		if got := sectionID([]string{"s4", "b"}); got != "s4.b" {
			t.Fatalf("sectionID = %q, want %q", got, "s4.b")
		}
	})

	bad := []struct {
		name string
		in   string
	}{
		{name: "empty", in: ""},
		{name: "leading dot", in: ".s4"},
		{name: "slash", in: "s4/b"},
		{name: "space", in: "s 4"},
	}
	for _, tc := range bad {
		t.Run("bad "+tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseSection(tc.in)
			if !errors.Is(err, data.ErrNotFound) {
				t.Fatalf("parseSection(%q): error %v, want ErrNotFound", tc.in, err)
			}
		})
	}
}

func TestParseRange(t *testing.T) {
	t.Parallel()

	valid := []struct {
		name     string
		in       string
		wantFrom int
		wantTo   int
	}{
		{name: "single line", in: "12", wantFrom: 12, wantTo: 12},
		{name: "inclusive range", in: "12-13", wantFrom: 12, wantTo: 13},
	}
	for _, tc := range valid {
		t.Run("valid "+tc.name, func(t *testing.T) {
			t.Parallel()
			from, to, err := parseRange(tc.in)
			if err != nil {
				t.Fatalf("parseRange(%q): unexpected error: %v", tc.in, err)
			}
			if from != tc.wantFrom || to != tc.wantTo {
				t.Fatalf("parseRange(%q) = %d, %d; want %d, %d", tc.in, from, to, tc.wantFrom, tc.wantTo)
			}
		})
	}

	bad := []struct {
		name string
		in   string
	}{
		{name: "zero line", in: "0"},
		{name: "reversed range", in: "13-12"},
		{name: "letters", in: "a"},
		{name: "open end", in: "1-"},
		{name: "open start", in: "-1"},
		{name: "three parts", in: "1-2-3"},
		{name: "empty", in: ""},
	}
	for _, tc := range bad {
		t.Run("bad "+tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := parseRange(tc.in)
			if !errors.Is(err, data.ErrInvalid) {
				t.Fatalf("parseRange(%q): error %v, want ErrInvalid", tc.in, err)
			}
		})
	}
}

func TestAnchors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "anchor of nested section", got: anchorOf([]string{"s4", "b"}, 12), want: "s4.b-l12"},
		{name: "fragment of one line", got: fragmentOf([]string{"s4"}, 12, 12), want: "#s4-l12"},
		{name: "fragment of a range", got: fragmentOf([]string{"s4"}, 12, 13), want: "#s4-l12-13"},
		{name: "bill path", got: billPath("us-119-hr-4127"), want: "/v1/bills/us-119-hr-4127"},
		{name: "version path", got: versionPath("us-119-hr-4127", "eh"), want: "/v1/bills/us-119-hr-4127/v/eh"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.got != tc.want {
				t.Fatalf("got %q, want %q", tc.got, tc.want)
			}
		})
	}
}

// blockFixture is a section whose depths are 0,1,1,2,0,1: line 1 owns 2-4,
// line 2 owns none, line 3 owns 4, line 4 owns none, line 5 owns 6, line 6
// owns none.
func blockFixture() []data.Line {
	return []data.Line{
		{N: 1, Depth: 0, Text: "a"},
		{N: 2, Depth: 1, Text: "b"},
		{N: 3, Depth: 1, Text: "c"},
		{N: 4, Depth: 2, Text: "d"},
		{N: 5, Depth: 0, Text: "e"},
		{N: 6, Depth: 1, Text: "f"},
	}
}

func TestToLinesBlockTo(t *testing.T) {
	t.Parallel()

	path := []string{"s1"}
	lines := blockFixture()
	sec := data.Section{Path: path, Lines: lines}
	wantBlockTo := []int{4, 2, 4, 4, 6, 6}

	got := toLines(path, lines)
	if len(got) != len(lines) {
		t.Fatalf("toLines returned %d lines, want %d", len(got), len(lines))
	}
	for i, l := range got {
		if l.BlockTo != wantBlockTo[i] {
			t.Errorf("line %d: BlockTo = %d, want %d", l.N, l.BlockTo, wantBlockTo[i])
		}
		from, to, ok := data.Block(sec, l.N)
		if !ok || from != l.N || to != l.BlockTo {
			t.Errorf("line %d: data.Block = %d, %d, %t; toLines BlockTo %d", l.N, from, to, ok, l.BlockTo)
		}
	}
	if got[2].Anchor != "s1-l3" {
		t.Fatalf("line 3 Anchor = %q, want %q", got[2].Anchor, "s1-l3")
	}
}

func TestToLinesSourceID(t *testing.T) {
	t.Parallel()

	path := []string{"s1"}
	tests := []struct {
		name     string
		sourceID string
		wantSet  bool
	}{
		{name: "unset stays nil", sourceID: "", wantSet: false},
		{name: "set is copied", sourceID: "x", wantSet: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			src := blockFixture()
			src[2].SourceID = tc.sourceID
			got := toLines(path, src)
			if !tc.wantSet {
				if got[2].SourceId != nil {
					t.Fatalf("SourceId = %q, want nil", *got[2].SourceId)
				}
				return
			}
			if got[2].SourceId == nil || *got[2].SourceId != tc.sourceID {
				t.Fatalf("SourceId = %v, want %q", got[2].SourceId, tc.sourceID)
			}
			if got[0].SourceId != nil {
				t.Fatalf("line 1 SourceId = %v, want nil", *got[0].SourceId)
			}
		})
	}
}

func TestSlicePage(t *testing.T) {
	t.Parallel()

	items := []int{1, 2, 3, 4, 5}

	t.Run("walks pages with next cursors", func(t *testing.T) {
		t.Parallel()
		want := [][]int{{1, 2}, {3, 4}, {5}}
		var cursor *string
		for i, wantPage := range want {
			page, next, err := slicePage(items, cursor, new(2))
			if err != nil {
				t.Fatalf("page %d: unexpected error: %v", i, err)
			}
			if !slices.Equal(page, wantPage) {
				t.Fatalf("page %d = %v, want %v", i, page, wantPage)
			}
			last := i == len(want)-1
			if last && next != nil {
				t.Fatalf("last page: next cursor set, want nil")
			}
			if !last && next == nil {
				t.Fatalf("page %d: next cursor nil, want a cursor", i)
			}
			cursor = next
		}
	})
}

func TestSlicePageDefaultLimit(t *testing.T) {
	t.Parallel()

	items := []int{1, 2, 3, 4, 5}
	page, next, err := slicePage(items, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.Equal(page, items) || next != nil {
		t.Fatalf("got %v, next %v; want all items and no next", page, next)
	}
}

func TestSlicePageRejects(t *testing.T) {
	t.Parallel()

	items := []int{1, 2, 3, 4, 5}

	t.Run("offset past the end is invalid", func(t *testing.T) {
		t.Parallel()
		tok, err := encodeCursor(pageCursor{Offset: len(items) + 1})
		if err != nil {
			t.Fatalf("encodeCursor: %v", err)
		}
		_, _, err = slicePage(items, &tok, new(2))
		if !errors.Is(err, data.ErrInvalid) {
			t.Fatalf("error %v, want ErrInvalid", err)
		}
	})

	bad := []struct {
		name   string
		cursor *string
		limit  *int
	}{
		{name: "limit zero", cursor: nil, limit: new(0)},
		{name: "limit above maximum", cursor: nil, limit: new(201)},
		{name: "malformed cursor", cursor: new("!!"), limit: new(2)},
	}
	for _, tc := range bad {
		t.Run("bad "+tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := slicePage(items, tc.cursor, tc.limit)
			if !errors.Is(err, data.ErrInvalid) {
				t.Fatalf("error %v, want ErrInvalid", err)
			}
		})
	}
}
