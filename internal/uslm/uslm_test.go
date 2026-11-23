package uslm_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/uslm"
)

var errStop = errors.New("stop")

// inlineTitle is a small title 1 with one section that exercises inline and
// block text, a subsection, notes, a source credit, and identifiers that are
// not top-level sections.
const inlineTitle = `<?xml version="1.0" encoding="UTF-8"?>
<uscDoc identifier="/us/usc/t1" xmlns="http://xml.house.gov/schemas/uslm/1.0">
<main><title identifier="/us/usc/t1">
<section identifier="/us/usc/t1/s2"><num value="2">§ 2.</num><heading> Inline  test </heading>
<content><p>See (<ref href="/x">x</ref>) and <i>it</i>.</p></content>
<subsection identifier="/us/usc/t1/s2/a"><num>(a)</num><content><p>Text</p></content></subsection>
<notes type="uscNote"><note><section identifier="/us/usc/t1/s2/zz"><heading>Nested</heading></section><subsection identifier="/us/usc/t1/s2/note"><p>NOTE</p></subsection></note></notes>
<sourceCredit>(Added <ref href="/x">Pub. L. 1</ref>)</sourceCredit>
</section>
<section identifier="/us/usc/t1/s2/x"><heading>not a section</heading></section>
<section identifier="/us/usc/t1/s3-"><heading>bad id</heading></section>
<section identifier="/us/usc/t1/s4–5"><heading>dash id</heading></section>
</title></main></uscDoc>`

func testRelease() data.Release {
	return data.Release{
		ID:        "119-142",
		PublicLaw: "119-142",
		Date:      time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC),
	}
}

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	src, err := fs.ReadFile(os.DirFS("testdata"), name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return src
}

// collect runs ParseTitle and gathers every section.
func collect(src []byte, release data.Release) ([]data.LawSection, error) {
	var out []data.LawSection
	err := uslm.ParseTitle(src, release, func(s data.LawSection) error {
		out = append(out, s)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect: %w", err)
	}
	return out, nil
}

func mustCollect(t *testing.T, src []byte, release data.Release) []data.LawSection {
	t.Helper()
	out, err := collect(src, release)
	if err != nil {
		t.Fatalf("ParseTitle: %v", err)
	}
	return out
}

func provisionIDs(s data.LawSection) []string {
	ids := make([]string, 0, len(s.Provisions))
	for _, p := range s.Provisions {
		ids = append(ids, p.ID)
	}
	return ids
}

func mustFind(t *testing.T, s data.LawSection, id string) data.Provision {
	t.Helper()
	p, ok := s.Find(id)
	if !ok {
		t.Fatalf("provision %q not found in %v", id, provisionIDs(s))
	}
	return p
}

func TestParseTitle_Title3Sample(t *testing.T) {
	t.Parallel()
	secs := mustCollect(t, readTestdata(t, "title3-sample.xml"), testRelease())
	if len(secs) != 2 {
		t.Fatalf("sections = %d, want 2", len(secs))
	}
	if secs[0].Ref.Section != "1" || secs[1].Ref.Section != "2" {
		t.Fatalf("section ids = %q, %q; want 1, 2", secs[0].Ref.Section, secs[1].Ref.Section)
	}
	if secs[0].Ref.Title != 3 || secs[0].Ref.Release != "119-142" {
		t.Errorf("ref = %+v", secs[0].Ref)
	}
	if got := secs[0].Provisions[0].ID; got != "usc/3/1" {
		t.Errorf("Provisions[0].ID = %q, want usc/3/1", got)
	}
}

func TestParseTitle_Title3Headings(t *testing.T) {
	t.Parallel()
	secs := mustCollect(t, readTestdata(t, "title3-sample.xml"), testRelease())
	if secs[0].Heading != "Time of appointing electors" {
		t.Errorf("heading 1 = %q", secs[0].Heading)
	}
	if !strings.HasPrefix(secs[1].Heading, "Repealed.") {
		t.Errorf("heading 2 = %q", secs[1].Heading)
	}
	if secs[0].Status != "" || secs[1].Status != "repealed" {
		t.Errorf("status = %q, %q; want empty, repealed", secs[0].Status, secs[1].Status)
	}
}

func TestParseTitle_Title3Text(t *testing.T) {
	t.Parallel()
	secs := mustCollect(t, readTestdata(t, "title3-sample.xml"), testRelease())
	text := secs[0].Provisions[0].Text
	if !strings.Contains(text, "The electors of President and Vice President") {
		t.Errorf("text = %q", text)
	}
	if strings.Contains(text, "Editorial Notes") || strings.Contains(text, "Prior Provisions") {
		t.Errorf("text includes notes: %q", text)
	}
	if strings.Contains(text, "Added") {
		t.Errorf("text includes sourceCredit: %q", text)
	}
}

func TestParseTitle_Title42Sample(t *testing.T) {
	t.Parallel()
	secs := mustCollect(t, readTestdata(t, "title42-sample.xml"), testRelease())
	if len(secs) != 1 {
		t.Fatalf("sections = %d, want 1", len(secs))
	}
	s := secs[0]
	if s.Ref.Section != "1395cc-7" || s.Ref.Title != 42 {
		t.Fatalf("ref = %+v, want title 42 section 1395cc-7", s.Ref)
	}
	ids := provisionIDs(s)
	for _, want := range []string{
		"usc/42/1395cc-7",
		"usc/42/1395cc-7/a",
		"usc/42/1395cc-7/a/1",
		"usc/42/1395cc-7/a/2/A",
	} {
		if !contains(ids, want) {
			t.Errorf("ProvisionIDs missing %q", want)
		}
	}
	if got := mustFind(t, s, "usc/42/1395cc-7/a/2/A").Citation; got != "42 U.S.C. 1395cc-7(a)(2)(A)" {
		t.Errorf("citation = %q", got)
	}
	if got := mustFind(t, s, "usc/42/1395cc-7/a/2/A").Text; !strings.HasPrefix(got, "(A) Subject to paragraph (3)(D)") {
		t.Errorf("text of /a/2/A = %q", got)
	}
}

func TestParseTitle_Title42SectionText(t *testing.T) {
	t.Parallel()
	s := mustCollect(t, readTestdata(t, "title42-sample.xml"), testRelease())[0]
	text := s.Provisions[0].Text
	for _, want := range []string{
		"Extension of Acute Hospital Care at Home initiative",
		"Acute Hospital Care at Home initiative of the Secretary",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("section text does not contain %q", want)
		}
	}
	if strings.Contains(text, "Editorial Notes") {
		t.Errorf("section text includes Editorial Notes")
	}
}

func TestParseTitle_SubdivisionText(t *testing.T) {
	t.Parallel()
	s := mustCollect(t, readTestdata(t, "title42-sample.xml"), testRelease())[0]
	a1 := mustFind(t, s, "usc/42/1395cc-7/a/1").Text
	if !strings.Contains(a1, "Acute Hospital Care at Home initiative of the Secretary") {
		t.Errorf("text of /a/1 = %q", a1)
	}
	if strings.Contains(a1, "Acute Hospital Care at Home waivers and flexibilities") {
		t.Errorf("text of /a/1 includes the text of /a/2")
	}
	a := mustFind(t, s, "usc/42/1395cc-7/a").Text
	if !strings.Contains(a, "For the purposes of paragraph (1)") {
		t.Errorf("text of /a does not include its child /a/2")
	}
}

func TestParseTitle_ProvisionOrder(t *testing.T) {
	t.Parallel()
	s := mustCollect(t, readTestdata(t, "title42-sample.xml"), testRelease())[0]
	ids := provisionIDs(s)
	if ids[0] != "usc/42/1395cc-7" {
		t.Fatalf("first provision = %q, want the section", ids[0])
	}
	if !contains(ids, "usc/42/1395cc-7/a") || indexOf(ids, "usc/42/1395cc-7/a") > indexOf(ids, "usc/42/1395cc-7/a/1") {
		t.Errorf("parent /a does not come before /a/1: %v", ids)
	}
}

func TestParseTitle_ReleaseDateFlows(t *testing.T) {
	t.Parallel()
	rel := testRelease()
	secs := mustCollect(t, readTestdata(t, "title42-sample.xml"), rel)
	s := secs[0]
	if !s.Meta.AsOf.Equal(rel.Date) {
		t.Errorf("section Meta.AsOf = %v, want %v", s.Meta.AsOf, rel.Date)
	}
	for _, p := range s.Provisions {
		if !p.CurrentAsOf.Equal(rel.Date) || !p.Meta.AsOf.Equal(rel.Date) {
			t.Errorf("provision %s: CurrentAsOf = %v, Meta.AsOf = %v", p.ID, p.CurrentAsOf, p.Meta.AsOf)
		}
	}
}

func TestParseTitle_Meta(t *testing.T) {
	t.Parallel()
	s := mustCollect(t, readTestdata(t, "title3-sample.xml"), testRelease())[0]
	want := data.Meta{
		Version:      1,
		AsOf:         testRelease().Date,
		Provenance:   data.ProvenanceSource,
		ContentClass: data.ContentOfficial,
	}
	if s.Meta.Version != want.Version || s.Meta.Provenance != want.Provenance || s.Meta.ContentClass != want.ContentClass {
		t.Errorf("section Meta = %+v, want %+v", s.Meta, want)
	}
	if got := s.Provisions[0].Meta; !reflect.DeepEqual(got, s.Meta) {
		t.Errorf("provision Meta = %+v, section Meta = %+v", got, s.Meta)
	}
	if len(s.SourceSHA256) != 64 || s.SourceSHA256 != strings.ToLower(s.SourceSHA256) {
		t.Errorf("SourceSHA256 = %q", s.SourceSHA256)
	}
}

func TestParseTitle_Deterministic(t *testing.T) {
	t.Parallel()
	src := readTestdata(t, "title42-sample.xml")
	first := mustCollect(t, src, testRelease())
	second := mustCollect(t, src, testRelease())
	if !reflect.DeepEqual(first, second) {
		t.Fatal("two parses of the same bytes differ")
	}
}

func TestParseTitle_InlineText(t *testing.T) {
	t.Parallel()
	secs := mustCollect(t, []byte(inlineTitle), testRelease())
	if len(secs) != 2 {
		t.Fatalf("sections = %d, want 2 (s2 and 4-5); got %+v", len(secs), secs)
	}
	s := secs[0]
	if s.Ref.Section != "2" || s.Heading != "Inline test" {
		t.Errorf("section = %q heading %q", s.Ref.Section, s.Heading)
	}
	if got := s.Provisions[0].Text; !strings.Contains(got, "See (x) and it.") {
		t.Errorf("inline text = %q", got)
	}
	if got := provisionIDs(s); !reflect.DeepEqual(got, []string{"usc/1/2", "usc/1/2/a"}) {
		t.Errorf("provisions = %v, want the section and /a only", got)
	}
	if got := s.Provisions[0].Text; strings.Contains(got, "NOTE") || strings.Contains(got, "Pub. L.") {
		t.Errorf("section text includes notes or sourceCredit: %q", got)
	}
}

func TestParseTitle_DashNormalisation(t *testing.T) {
	t.Parallel()
	secs := mustCollect(t, []byte(inlineTitle), testRelease())
	if secs[1].Ref.Section != "4-5" || secs[1].Heading != "dash id" {
		t.Errorf("section = %q heading %q; want 4-5 'dash id'", secs[1].Ref.Section, secs[1].Heading)
	}
	if secs[1].Provisions[0].ID != "usc/1/4-5" {
		t.Errorf("ID = %q", secs[1].Provisions[0].ID)
	}
}

func TestParseTitle_FnErrorStops(t *testing.T) {
	t.Parallel()
	calls := 0
	err := uslm.ParseTitle(readTestdata(t, "title3-sample.xml"), testRelease(), func(data.LawSection) error {
		calls++
		return errStop
	})
	if !errors.Is(err, errStop) {
		t.Fatalf("err = %v, want errStop unchanged", err)
	}
	if calls != 1 {
		t.Errorf("fn called %d times, want 1", calls)
	}
}

func TestParseTitle_InvalidInput(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"empty":         "",
		"other root":    `<?xml version="1.0"?><html identifier="/us/usc/t3"></html>`,
		"no identifier": `<?xml version="1.0"?><uscDoc xmlns="http://xml.house.gov/schemas/uslm/1.0"></uscDoc>`,
		"appendix root": `<?xml version="1.0"?><uscDoc identifier="/us/usc/t5a" xmlns="http://xml.house.gov/schemas/uslm/1.0"></uscDoc>`,
		"zero title":    `<?xml version="1.0"?><uscDoc identifier="/us/usc/t0" xmlns="http://xml.house.gov/schemas/uslm/1.0"></uscDoc>`,
		"unclosed":      `<?xml version="1.0"?><uscDoc identifier="/us/usc/t1"><section identifier="/us/usc/t1/s1">`,
		"mismatched":    `<?xml version="1.0"?><uscDoc identifier="/us/usc/t1"></wrong>`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := collect([]byte(src), testRelease())
			if !errors.Is(err, data.ErrInvalid) {
				t.Errorf("err = %v, want ErrInvalid", err)
			}
		})
	}
}

func contains(xs []string, want string) bool {
	return indexOf(xs, want) >= 0
}

func indexOf(xs []string, want string) int {
	for i, x := range xs {
		if x == want {
			return i
		}
	}
	return -1
}
