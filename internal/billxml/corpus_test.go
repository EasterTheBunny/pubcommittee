package billxml_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/billxml"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

// BILLXML_UPDATE=1 rewrites the golden files. BILLXML_BULK_DIR optionally
// names a directory of extra govinfo XML files (never committed) to check
// for dropped text.

func fixtureFS(t *testing.T) fs.FS {
	t.Helper()
	return os.DirFS("../../fixtures/bills")
}

// sources lists "<bill>/v/<version>/source.xml" under the fixtures.
func sources(t *testing.T) []string {
	t.Helper()
	paths, err := fs.Glob(fixtureFS(t), "*/v/*/source.xml")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no fixtures: %v", err)
	}
	return paths
}

func parseFile(t *testing.T, fsys fs.FS, name string) (*billxml.Document, []byte) {
	t.Helper()
	src, err := fs.ReadFile(fsys, name)
	if err != nil {
		t.Fatal(err)
	}
	d, err := billxml.Parse(src, nil)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return d, src
}

func checkNoDropped(t *testing.T, name string, d *billxml.Document, src []byte) {
	t.Helper()
	if got, want := parsedText(d), bodyText(t, src); got != want {
		t.Errorf("%s: dropped or reordered text at %s", name, firstDiff(got, want))
	}
	n := 0
	for _, s := range d.Sections {
		for _, l := range s.Lines {
			n++
			if l.N != n || l.Anchor != s.Path[0]+"-l"+strconv.Itoa(n) || l.Depth < 0 || l.Kind == "" || l.Text == "" {
				t.Errorf("%s: bad line %+v at position %d", name, l, n)
			}
		}
	}
}

func TestCorpusNoDroppedText(t *testing.T) {
	t.Parallel()
	fsys := fixtureFS(t)
	for _, p := range sources(t) {
		d, src := parseFile(t, fsys, p)
		checkNoDropped(t, p, d, src)
	}
}

// TestBulkNoDroppedText runs the same check over a larger local pull.
func TestBulkNoDroppedText(t *testing.T) {
	t.Parallel()
	dir := os.Getenv("BILLXML_BULK_DIR")
	if dir == "" {
		t.Skip("BILLXML_BULK_DIR not set")
	}
	fsys := os.DirFS(dir)
	names, err := fs.Glob(fsys, "*.xml")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		src, err := fs.ReadFile(fsys, name)
		if err != nil {
			t.Fatal(err)
		}
		d, err := billxml.Parse(src, nil)
		if errors.Is(err, data.ErrInvalid) && strings.Contains(err.Error(), "not bill or resolution") {
			continue // amendment documents are out of scope (M1-04 Out)
		}
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		checkNoDropped(t, name, d, src)
	}
}

func TestDeterministic(t *testing.T) {
	t.Parallel()
	fsys := fixtureFS(t)
	for _, p := range sources(t) {
		a, src := parseFile(t, fsys, p)
		b, err := billxml.Parse(src, nil)
		if err != nil {
			t.Fatal(err)
		}
		ja, errA := billxml.Marshal(a)
		jb, errB := billxml.Marshal(b)
		if errA != nil || errB != nil || !bytes.Equal(ja, jb) {
			t.Errorf("%s: output differs between runs", p)
		}
	}
}

func TestGolden(t *testing.T) {
	t.Parallel()
	fsys := fixtureFS(t)
	root, err := os.OpenRoot("testdata")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	for _, p := range sources(t) {
		d, _ := parseFile(t, fsys, p)
		got, err := billxml.Marshal(d)
		if err != nil {
			t.Fatal(err)
		}
		// us-119-hr-4313/v/ih/source.xml -> us-119-hr-4313.ih.text.golden.json
		parts := strings.Split(p, "/")
		golden := parts[0] + "." + parts[2] + ".text.golden.json"
		if os.Getenv("BILLXML_UPDATE") != "" {
			if err := root.WriteFile(golden, got, 0o600); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := fs.ReadFile(root.FS(), path.Clean(golden))
		if err != nil {
			t.Fatalf("%v (run with BILLXML_UPDATE=1)", err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s: output differs from testdata/%s", p, golden)
		}
	}
}
