package billxml_test

import (
	"strings"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/billxml"
)

// TestCorpusTableUnit checks that a real table parses to one addressable
// unit that keeps its raw markup, inside a quoted block.
func TestCorpusTableUnit(t *testing.T) {
	t.Parallel()
	d, src := parseFile(t, fixtureFS(t), "us-119-hr-319/v/ih/source.xml")
	var tables int
	for _, s := range d.Sections {
		for _, l := range s.Lines {
			if l.Kind != billxml.KindTable {
				continue
			}
			tables++
			if !l.Quoted || !strings.HasPrefix(l.Raw, "<table") || !strings.HasSuffix(l.Raw, "</table>") ||
				!strings.Contains(string(src), l.Raw) || l.Text != "Idaho 3." {
				t.Errorf("table line %+v", l)
			}
		}
	}
	if tables != 1 {
		t.Errorf("got %d table lines, want 1", tables)
	}
}

// TestCorpusAmendatoryAndRepeal checks the exit bills' coverage in parsed form.
func TestCorpusAmendatoryAndRepeal(t *testing.T) {
	t.Parallel()
	fsys := fixtureFS(t)
	for name, want := range map[string]struct {
		quoted   bool
		contains string
	}{
		"us-119-hr-4313/v/ih/source.xml": {true, "striking"},
		"us-119-hr-3062/v/ih/source.xml": {false, "is repealed"},
		"us-119-hr-3427/v/ih/source.xml": {false, ""},
	} {
		d, _ := parseFile(t, fsys, name)
		var quoted bool
		var all strings.Builder
		for _, s := range d.Sections {
			for _, l := range s.Lines {
				quoted = quoted || l.Quoted
				all.WriteString(l.Text + "\n")
			}
		}
		if quoted != want.quoted || !strings.Contains(all.String(), want.contains) {
			t.Errorf("%s: quoted=%v contains(%q)=%v", name, quoted, want.contains, strings.Contains(all.String(), want.contains))
		}
	}
}
