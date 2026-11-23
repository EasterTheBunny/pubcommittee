package versiondiff_test

import (
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/billxml"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/versiondiff"
)

func parseFixture(t *testing.T, bill, code string) *billxml.Document {
	t.Helper()
	src, err := fs.ReadFile(os.DirFS("../../fixtures/bills"), bill+"/v/"+code+"/source.xml")
	if err != nil {
		t.Fatal(err)
	}
	d, err := billxml.Parse(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func text(d *billxml.Document) map[int]string {
	out := map[int]string{}
	for _, s := range d.Sections {
		for _, l := range s.Lines {
			out[l.N] = strings.Join(strings.Fields(l.Text), " ")
		}
	}
	return out
}

// TestExitBills compares every consecutive pair of the exit bills: every old
// anchor has a class, unchanged lines carry the same words, removed lines have
// no identical line in the new version, and the mapping is one to one.
func TestExitBills(t *testing.T) {
	t.Parallel()
	pairs := [][2]string{{"ih", "rh"}, {"rh", "eh"}, {"eh", "rfs"}}
	for _, bill := range []string{"us-119-hr-4313", "us-119-hr-3062", "us-119-hr-3427"} {
		for _, p := range pairs {
			t.Run(bill+"/"+p[0]+"-"+p[1], func(t *testing.T) {
				t.Parallel()
				prev, next := parseFixture(t, bill, p[0]), parseFixture(t, bill, p[1])
				counts := checkMapping(t, prev, next)
				t.Logf("%v", counts)
				if p == [2]string{"ih", "rh"} && bill == "us-119-hr-4313" && counts[data.LineUnchanged] == len(text(prev)) {
					t.Error("ih to rh of hr-4313 has text differences but everything is unchanged")
				}
			})
		}
	}
}

func checkMapping(t *testing.T, prev, next *billxml.Document) map[data.LineClass]int {
	t.Helper()
	oldText, newText := text(prev), text(next)
	newSet := map[string]bool{}
	for _, s := range newText {
		newSet[s] = true
	}
	seen, used := 0, map[int]bool{}
	counts := map[data.LineClass]int{}
	for _, sc := range versiondiff.CompareDocuments(prev, next) {
		for _, lm := range sc.Lines {
			seen++
			counts[lm.Class]++
			checkLine(t, lm, oldText, newText, newSet)
			if lm.New != nil {
				if used[lm.New.N] {
					t.Errorf("new l%d is the counterpart of two old lines", lm.New.N)
				}
				used[lm.New.N] = true
			}
		}
	}
	if seen != len(oldText) {
		t.Errorf("classified %d old lines, want %d", seen, len(oldText))
	}
	return counts
}

func checkLine(t *testing.T, lm data.LineMapping, oldText, newText map[int]string, newSet map[string]bool) {
	t.Helper()
	switch lm.Class {
	case data.LineUnchanged:
		if oldText[lm.N] != newText[lm.New.N] {
			t.Errorf("unchanged l%d differs from new l%d", lm.N, lm.New.N)
		}
	case data.LineMoved, data.LineChanged:
		if lm.New == nil {
			t.Errorf("%s l%d has no new location", lm.Class, lm.N)
		}
	case data.LineRemoved:
		if newSet[oldText[lm.N]] {
			t.Errorf("removed l%d still appears verbatim in the new version", lm.N)
		}
	}
}

func TestIdenticalVersionsAreUnchanged(t *testing.T) {
	t.Parallel()
	d := parseFixture(t, "us-119-hr-4313", "ih")
	for _, sc := range versiondiff.CompareDocuments(d, d) {
		if sc.Status != data.SectionUnchanged || len(sc.Changes) != 0 {
			t.Fatalf("section %v: status %s, %d changes", sc.Path, sc.Status, len(sc.Changes))
		}
	}
}
