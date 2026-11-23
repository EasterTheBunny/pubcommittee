package citations_test

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/billxml"
	"github.com/easterthebunny/pubcommittee/internal/citations"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/uslm"
)

const root = "../../fixtures"

func readFixture(t *testing.T, rel string) []byte {
	t.Helper()
	b, err := fs.ReadFile(os.DirFS(root), rel)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func releaseDate() time.Time { return time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC) }

// loadLaw stores the cited sections of fixtures/uscode as release 119-142.
func loadLaw(t *testing.T, s *memory.Store) data.Release {
	t.Helper()
	ctx := context.Background()
	rel := data.Release{
		ID: "119-142", PublicLaw: "119-142", Date: releaseDate(),
		Meta: data.Meta{Version: 1, AsOf: releaseDate(), Provenance: data.ProvenanceSource, ContentClass: data.ContentOfficial},
	}
	files, err := fs.Glob(os.DirFS(root), "uscode/119-142/t*-s*.xml")
	if err != nil || len(files) == 0 {
		t.Fatalf("fixtures: %v (%d files)", err, len(files))
	}
	for _, f := range files {
		var title int
		if _, err := fmt.Sscanf(filepath.Base(f), "t%d-", &title); err != nil {
			t.Fatal(err)
		}
		frag := readFixture(t, f)
		doc := fmt.Sprintf(`<uscDoc xmlns="http://xml.house.gov/schemas/uslm/1.0" identifier="/us/usc/t%d"><main>%s</main></uscDoc>`, title, frag)
		if err := uslm.ParseTitle([]byte(doc), rel, func(sec data.LawSection) error {
			return s.PutLawSection(ctx, sec)
		}); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
	}
	if err := s.PutRelease(ctx, rel); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCurrentRelease(ctx, rel.ID); err != nil {
		t.Fatal(err)
	}
	return rel
}

// loadBill stores a fixture bill version and returns it with its source.
func loadBill(t *testing.T, s *memory.Store, bill, code string) (data.OfficialVersion, []byte) {
	t.Helper()
	src := readFixture(t, path.Join("bills", "us-119-"+bill, "v", code, "source.xml"))
	doc, err := billxml.Parse(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if _, err := fmt.Sscanf(bill, "hr-%d", &n); err != nil {
		t.Fatal(err)
	}
	ref := data.VersionRef{Bill: data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: n}, Version: code}
	v := data.OfficialVersion{
		Ref: ref, Type: "introduced", PublishedAt: time.Date(2025, 7, 10, 0, 0, 0, 0, time.UTC),
		SourceSHA256: doc.SourceSHA256, Sections: doc.DataSections(),
		Meta: data.Meta{Version: 1, AsOf: releaseDate(), Provenance: data.ProvenanceSource, ContentClass: data.ContentOfficial},
	}
	if err := s.PutVersion(context.Background(), v, src); err != nil {
		t.Fatal(err)
	}
	return v, src
}

func TestBuildExitBills(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := memory.New()
	rel := loadLaw(t, s)
	v, src := loadBill(t, s, "hr-4313", "ih")
	set, err := citations.Build(ctx, s, rel, v, src, billxml.ParserVersion, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Links) != 2 {
		t.Fatalf("links = %d, want 2: %+v", len(set.Links), set.Links)
	}
	checkLink(t, set.Links[0], data.CitationResolved, "usc/42/1395cc-7/a/1")
	checkLink(t, set.Links[1], data.CitationResolved, "usc/42/1395cc-7")
	if len(set.Provisions) != 2 || set.Provisions[0].Release != "119-142" || !set.Provisions[0].CurrentAsOf.Equal(releaseDate()) {
		t.Errorf("provisions = %+v", set.Provisions)
	}
	if set.Ref.Method != "billxml/1+citations/1+usc@119-142" {
		t.Errorf("method = %s", set.Ref.Method)
	}
}

func checkLink(t *testing.T, l data.CitationEntry, state data.CitationState, id string) {
	t.Helper()
	if l.State != state || fmt.Sprint(l.ProvisionIDs) != "["+id+"]" || l.Source != data.SourceMarkup {
		t.Errorf("link = %+v, want %s %s from markup", l, state, id)
	}
	if l.Line == 0 || len(l.Section) == 0 {
		t.Errorf("link has no location: %+v", l)
	}
}

func TestEveryCitationHasState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := memory.New()
	rel := loadLaw(t, s)
	xref := regexp.MustCompile(`<external-xref`)
	for _, bill := range []string{"hr-4313", "hr-3062", "hr-3427"} {
		for _, code := range []string{"ih", "rh", "eh", "rfs"} {
			v, src := loadBill(t, s, bill, code)
			set, err := citations.Build(ctx, s, rel, v, src, billxml.ParserVersion, nil)
			if err != nil {
				t.Fatal(err)
			}
			if n := len(xref.FindAll(src, -1)); len(set.Links) < n {
				t.Errorf("%s %s: %d links for %d xrefs", bill, code, len(set.Links), n)
			}
			for _, l := range set.Links {
				checkStated(t, bill+" "+code, l)
			}
		}
	}
}

func checkStated(t *testing.T, where string, l data.CitationEntry) {
	t.Helper()
	known := l.State == data.CitationResolved || l.State == data.CitationAmbiguous || l.State == data.CitationUnresolved
	if !known {
		t.Errorf("%s: %q has state %q", where, l.Text, l.State)
	}
	if l.State == data.CitationUnresolved && (l.Reason == "" || len(l.ProvisionIDs) != 0) {
		t.Errorf("%s: unresolved %q: reason %q ids %v", where, l.Text, l.Reason, l.ProvisionIDs)
	}
}

func TestStepIdempotentAndFreshness(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := memory.New()
	rel := loadLaw(t, s)
	v, _ := loadBill(t, s, "hr-4313", "ih")
	later := []citations.LaterLaw{{
		PublicLaw: "119-150", EnactedAt: releaseDate().AddDate(0, 0, 20), Sections: []string{"usc/42/1395cc-7"},
	}, {
		PublicLaw: "119-100", EnactedAt: releaseDate().AddDate(0, 0, -20), Sections: []string{"usc/42/1395cc-7"},
	}}
	for range 2 {
		n, err := citations.Step(ctx, s, s, s, v.Ref.Bill, billxml.ParserVersion, later)
		if err != nil || n != 1 {
			t.Fatalf("Step = %d, %v", n, err)
		}
	}
	set, err := s.GetCitations(ctx, data.CitationRef{Version: v.Ref, Method: citations.Method(billxml.ParserVersion, rel.ID)})
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Freshness) != 2 { // two cited provisions in section 1395cc-7; only the law after the release flags
		t.Fatalf("freshness = %+v", set.Freshness)
	}
	for _, f := range set.Freshness {
		if f.PublicLaw != "119-150" {
			t.Errorf("flag from %s, want only the later law", f.PublicLaw)
		}
	}
}

func TestExtractTextPatterns(t *testing.T) {
	t.Parallel()
	v := data.OfficialVersion{Sections: []data.Section{{Path: []string{"s1"}, Lines: []data.Line{
		{N: 1, Text: "Section 552 of title 5, United States Code, is amended."},
		{N: 2, Text: "Section 45A of the Internal Revenue Code of 1986 is amended; see 15 U.S.C. 717b(c) et seq. and Public Law 111–163."},
		{N: 3, Text: "Section 1866G of the Social Security Act (42 U.S.C. 1395cc–7) and section 1848 of the Social Security Act."},
	}}}}
	src := []byte(`<bill><section id="a"><text id="t3">Section 1866G of the Social Security Act (<external-xref legal-doc="usc" parsable-cite="usc/42/1395cc-7">42 U.S.C. 1395cc–7</external-xref>) and section 1848 of the Social Security Act.</text></section></bill>`)
	v.Sections[0].Lines[2].SourceID = "t3"
	raws, err := citations.Extract(v, src)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(raws))
	for _, r := range raws {
		got = append(got, fmt.Sprintf("%d:%s:%s", r.Line, r.Kind, strings.TrimSpace(r.Text)))
	}
	want := []string{
		"1:usc:Section 552 of title 5, United States Code",
		"2:act-section:Section 45A of the Internal Revenue Code of 1986",
		"2:usc:15 U.S.C. 717b(c)",
		"2:public-law:Public Law 111–163",
		"3:act-section:Section 1866G of the Social Security Act",
		"3:usc:42 U.S.C. 1395cc–7",
		"3:act-section:section 1848 of the Social Security Act",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestUnresolvedListedForExitBills(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := memory.New()
	rel := loadLaw(t, s)
	v, src := loadBill(t, s, "hr-3062", "ih")
	set, err := citations.Build(ctx, s, rel, v, src, billxml.ParserVersion, nil)
	if err != nil {
		t.Fatal(err)
	}
	var open []string
	for _, e := range set.Unresolved() {
		if e.Reason == "" {
			t.Errorf("%q is listed without a reason", e.Text)
		}
		open = append(open, e.Text+" "+string(e.State))
	}
	// "et seq." names a range, so it is listed as ambiguous, never resolved.
	want := "42 U.S.C. 4321 et seq. ambiguous"
	if len(open) != 1 || open[0] != want {
		t.Errorf("unresolved list = %q, want [%q]", open, want)
	}
}
