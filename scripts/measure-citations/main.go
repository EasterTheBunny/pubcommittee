// Command measure-citations resolves the citations of fixture bills against
// US Code title files and compares each with a hand-labelled sample. It
// prints a Markdown report: coverage (how many labelled citations were found
// and linked) and the precision of `resolved` are reported separately.
//
//	go run -tags memory ./scripts/measure-citations \
//	    -law .local-data/uscode/119-142 -release 119-142 \
//	    -bills fixtures/bills -labels test/acceptance/M1-07/labels.json
//
// -law reads the downloaded xml_usc<TT>@<release>.zip files; with -fixtures
// it reads the cited sections committed under fixtures/uscode instead.
package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/billxml"
	"github.com/easterthebunny/pubcommittee/internal/citations"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/uslm"
)

// label is one hand-labelled citation: the text as written, where it occurs
// (bill versions with the number of occurrences), and the expected outcome.
type label struct {
	Bill       string         `json:"bill"`
	Text       string         `json:"text"`
	Kind       string         `json:"kind"`
	Versions   map[string]int `json:"versions"`
	State      string         `json:"state"`
	Provisions []string       `json:"provisions"`
	Note       string         `json:"note,omitempty"`
}

var errInput = errors.New("bad input")

func main() {
	lawDir := flag.String("law", "", "directory with xml_usc<TT>@<release>.zip files")
	fixtures := flag.String("fixtures", "", "fixtures/uscode directory (cited sections only) instead of -law")
	release := flag.String("release", "119-142", "release point id")
	releaseDate := flag.String("release-date", "2026-11-10", "date the release point is current through")
	bills := flag.String("bills", "fixtures/bills", "fixture bills directory")
	labelsFile := flag.String("labels", "", "labels JSON file")
	only := flag.String("only", "", "comma-separated bill ids to measure (default all labelled bills)")
	flag.Parse()
	if err := run(*lawDir, *fixtures, *release, *releaseDate, *bills, *labelsFile, *only); err != nil {
		fmt.Fprintln(os.Stderr, "measure-citations:", err)
		os.Exit(1)
	}
}

func run(lawDir, fixtures, release, releaseDate, bills, labelsFile, only string) error {
	date, err := time.Parse(time.DateOnly, releaseDate)
	if err != nil {
		return fmt.Errorf("release date: %w", err)
	}
	raw, err := os.ReadFile(labelsFile)
	if err != nil {
		return fmt.Errorf("labels: %w", err)
	}
	var labels []label
	if err := json.Unmarshal(raw, &labels); err != nil {
		return fmt.Errorf("labels: %w", err)
	}
	if only != "" {
		keep := strings.Split(only, ",")
		labels = slices.DeleteFunc(labels, func(l label) bool { return !slices.Contains(keep, l.Bill) })
	}
	ctx := context.Background()
	store := memory.New()
	rel := data.Release{
		ID: release, PublicLaw: release, Date: date,
		Meta: data.Meta{Version: 1, AsOf: date, Provenance: data.ProvenanceSource, ContentClass: data.ContentOfficial},
	}
	if err := loadLaw(ctx, store, rel, lawDir, fixtures); err != nil {
		return err
	}
	sets, err := buildSets(ctx, store, rel, bills, labels)
	if err != nil {
		return err
	}
	report(os.Stdout, rel, labels, sets)
	return nil
}

func loadLaw(ctx context.Context, store *memory.Store, rel data.Release, lawDir, fixtures string) error {
	put := func(s data.LawSection) error { return store.PutLawSection(ctx, s) }
	switch {
	case lawDir != "":
		zips, err := filepath.Glob(filepath.Join(lawDir, "xml_usc*@"+rel.ID+".zip"))
		if err != nil || len(zips) == 0 {
			return fmt.Errorf("no title zips in %s: %w", lawDir, errInput)
		}
		for _, z := range zips {
			b, err := readZipXML(z)
			if err != nil {
				return err
			}
			if err := uslm.ParseTitle(b, rel, put); err != nil {
				return fmt.Errorf("%s: %w", z, err)
			}
		}
	case fixtures != "":
		files, err := filepath.Glob(filepath.Join(fixtures, rel.ID, "t*-s*.xml"))
		if err != nil || len(files) == 0 {
			return fmt.Errorf("no sections in %s: %w", fixtures, errInput)
		}
		for _, f := range files {
			var title int
			if _, err := fmt.Sscanf(filepath.Base(f), "t%d-", &title); err != nil {
				return fmt.Errorf("%s: %w", f, err)
			}
			frag, err := os.ReadFile(f) //nolint:gosec // developer tool reading a path given on the command line
			if err != nil {
				return fmt.Errorf("read %s: %w", f, err)
			}
			doc := fmt.Sprintf(`<uscDoc identifier="/us/usc/t%d"><main>%s</main></uscDoc>`, title, frag)
			if err := uslm.ParseTitle([]byte(doc), rel, put); err != nil {
				return fmt.Errorf("%s: %w", f, err)
			}
		}
	default:
		return fmt.Errorf("give -law or -fixtures: %w", errInput)
	}
	if err := store.PutRelease(ctx, rel); err != nil {
		return fmt.Errorf("store release: %w", err)
	}
	return nil
}

func readZipXML(p string) ([]byte, error) {
	zr, err := zip.OpenReader(p)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", p, err)
	}
	defer func() { _ = zr.Close() }()
	for _, f := range zr.File {
		if !strings.HasSuffix(strings.ToLower(f.Name), ".xml") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open member: %w", err)
		}
		defer func() { _ = rc.Close() }()
		b, err := io.ReadAll(rc)
		if err != nil {
			return nil, fmt.Errorf("read member: %w", err)
		}
		return b, nil
	}
	return nil, fmt.Errorf("%s: no xml member: %w", p, errInput)
}

// buildSets resolves every version of every labelled bill. The key is
// "<bill>/<version>".
func buildSets(
	ctx context.Context, store *memory.Store, rel data.Release, bills string, labels []label,
) (map[string]data.CitationSet, error) {
	sets := map[string]data.CitationSet{}
	seen := map[string]bool{}
	for _, l := range labels {
		if seen[l.Bill] {
			continue
		}
		seen[l.Bill] = true
		dirs, err := fs.Glob(os.DirFS(bills), path.Join(l.Bill, "v", "*", "source.xml"))
		if err != nil {
			return nil, fmt.Errorf("glob: %w", err)
		}
		for _, d := range dirs {
			code := strings.Split(d, "/")[2]
			src, err := fs.ReadFile(os.DirFS(bills), d)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", d, err)
			}
			doc, err := billxml.Parse(src, nil)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", d, err)
			}
			v := data.OfficialVersion{
				Ref:  data.VersionRef{Bill: data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr"}, Version: code},
				Type: code, PublishedAt: rel.Date, SourceSHA256: doc.SourceSHA256, Sections: doc.DataSections(),
			}
			v.Ref.Bill.Number = billNumber(l.Bill)
			set, err := citations.Build(ctx, store, rel, v, src, billxml.ParserVersion, nil)
			if err != nil {
				return nil, fmt.Errorf("%s %s: %w", l.Bill, code, err)
			}
			sets[l.Bill+"/"+code] = set
		}
	}
	return sets, nil
}

func billNumber(id string) int {
	var n int
	_, _ = fmt.Sscanf(id[strings.LastIndex(id, "-")+1:], "%d", &n)
	return max(n, 1)
}

// tally counts outcomes for one group of labelled citations.
type tally struct {
	total, found, stateOK, provOK        int
	detResolved, detResolvedOK           int // detected as resolved, and right
	labelResolved, labelResolvedFound    int
	labelLinked, labelLinkedFound, extra int
}

func (t *tally) add(o tally) {
	t.total += o.total
	t.found += o.found
	t.stateOK += o.stateOK
	t.provOK += o.provOK
	t.detResolved += o.detResolved
	t.detResolvedOK += o.detResolvedOK
	t.labelResolved += o.labelResolved
	t.labelResolvedFound += o.labelResolvedFound
	t.labelLinked += o.labelLinked
	t.labelLinkedFound += o.labelLinkedFound
	t.extra += o.extra
}

func pct(n, d int) string {
	if d == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f%% (%d/%d)", 100*float64(n)/float64(d), n, d)
}

func report(w io.Writer, rel data.Release, labels []label, sets map[string]data.CitationSet) {
	byKind := map[string]*tally{}
	var all tally
	var mismatches, unlabelled []string
	used := map[string]map[int]bool{} // set key -> link index consumed by a label
	for _, l := range labels {
		for code, n := range l.Versions {
			key := l.Bill + "/" + code
			if used[key] == nil {
				used[key] = map[int]bool{}
			}
			t := scoreLabel(l, code, n, sets[key], used[key], &mismatches)
			if byKind[l.Kind] == nil {
				byKind[l.Kind] = &tally{}
			}
			byKind[l.Kind].add(t)
			all.add(t)
		}
	}
	keys := make([]string, 0, len(sets))
	for k := range sets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for i, e := range sets[k].Links {
			if !used[k][i] {
				all.extra++
				unlabelled = append(unlabelled, fmt.Sprintf("%s: %q -> %s %v", k, e.Text, e.State, e.ProvisionIDs))
			}
		}
	}
	printReport(w, rel, all, byKind, mismatches, unlabelled)
}

// scoreLabel compares the first n detected entries with this text of one
// version with the label.
func scoreLabel(l label, code string, n int, set data.CitationSet, used map[int]bool, mism *[]string) tally {
	var t tally
	t.total = n
	if l.State == string(data.CitationResolved) {
		t.labelResolved = n
	}
	if l.State != string(data.CitationUnresolved) {
		t.labelLinked = n
	}
	got := 0
	for i, e := range set.Links {
		if got == n {
			break
		}
		if collapse(e.Text) != collapse(l.Text) || used[i] {
			continue
		}
		used[i] = true
		got++
		t.found++
		okState := string(e.State) == l.State
		okProv := slices.Equal(nonNil(e.ProvisionIDs), nonNil(l.Provisions))
		if okState {
			t.stateOK++
		}
		if okState && okProv {
			t.provOK++
		}
		if e.State == data.CitationResolved {
			t.detResolved++
			if okState && okProv {
				t.detResolvedOK++
			}
		}
		if l.State == string(data.CitationResolved) && okState && okProv {
			t.labelResolvedFound++
		}
		if l.State != string(data.CitationUnresolved) && e.State != data.CitationUnresolved {
			t.labelLinkedFound++
		}
		if !okState || !okProv {
			*mism = append(*mism, fmt.Sprintf("%s/%s %q: got %s %v (%s), want %s %v",
				l.Bill, code, l.Text, e.State, e.ProvisionIDs, e.Reason, l.State, l.Provisions))
		}
	}
	if got < n {
		*mism = append(*mism, fmt.Sprintf("%s/%s %q: found %d of %d labelled occurrences", l.Bill, code, l.Text, got, n))
	}
	return t
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

func printReport(
	w io.Writer, rel data.Release, all tally, byKind map[string]*tally, mismatches, unlabelled []string,
) {
	p := func(format string, a ...any) { _, _ = fmt.Fprintf(w, format+"\n", a...) }
	p("# Citation resolution report")
	p("")
	p("Release point `%s` (current through %s). Counts are occurrences in the labelled bill versions.", rel.ID, rel.Date.Format(time.DateOnly))
	p("")
	p("| Measure | Value |")
	p("| --- | --- |")
	p("| Labelled citations | %d |", all.total)
	p("| Resolution rate (labelled citations resolved correctly / all labelled) | %s |", pct(all.detResolvedOK, all.total))
	p("| Detection (labelled citations found) | %s |", pct(all.found, all.total))
	p("| State correct | %s |", pct(all.stateOK, all.total))
	p("| State and provision correct | %s |", pct(all.provOK, all.total))
	p("| Precision of `resolved` (detected resolved that are right) | %s |", pct(all.detResolvedOK, all.detResolved))
	p("| Coverage of `resolved` (labelled resolved that were resolved right) | %s |", pct(all.labelResolvedFound, all.labelResolved))
	p("| Linked coverage (labelled resolved or ambiguous that were linked) | %s |", pct(all.labelLinkedFound, all.labelLinked))
	p("| Detected but not labelled | %d |", all.extra)
	p("")
	p("| Kind | Labelled | Found | State correct | Precision of resolved | Coverage of resolved |")
	p("| --- | --- | --- | --- | --- | --- |")
	kinds := make([]string, 0, len(byKind))
	for k := range byKind {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		t := byKind[k]
		p("| %s | %d | %s | %s | %s | %s |", k, t.total, pct(t.found, t.total), pct(t.stateOK, t.total),
			pct(t.detResolvedOK, t.detResolved), pct(t.labelResolvedFound, t.labelResolved))
	}
	section := func(title string, lines []string) {
		p("")
		p("## %s (%d)", title, len(lines))
		for _, l := range lines {
			p("- %s", l)
		}
	}
	section("Mismatches with the labels", mismatches)
	section("Detected but not in the labels", unlabelled)
}
