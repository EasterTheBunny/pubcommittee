package conformance

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// SampleRelease returns a valid release record; day moves its date.
func SampleRelease(id string, day int) data.Release {
	return data.Release{
		ID: id, PublicLaw: id,
		Date: time.Date(2026, 11, day, 0, 0, 0, 0, time.UTC),
		Meta: SampleMeta(),
	}
}

// SampleLawSection returns a valid section "1395cc-7" of title 42 at release
// with one subdivision.
func SampleLawSection(release string) data.LawSection {
	rel := SampleRelease(release, 10)
	return data.LawSection{
		Ref:          data.ProvisionRef{Release: release, Title: 42, Section: "1395cc-7"},
		Heading:      "Extension of Acute Hospital Care at Home initiative",
		SourceSHA256: "00",
		Provisions: []data.Provision{
			{ID: "usc/42/1395cc-7", Citation: "42 U.S.C. 1395cc-7", Text: "(a) In general (1) Extension", CurrentAsOf: rel.Date, Meta: SampleMeta()},
			{ID: "usc/42/1395cc-7/a", Citation: "42 U.S.C. 1395cc-7(a)", Text: "(a) In general (1) Extension", CurrentAsOf: rel.Date, Meta: SampleMeta()},
		},
		Meta: SampleMeta(),
	}
}

// SampleCitationSet returns a valid citation set for version "ih" of bill n.
func SampleCitationSet(n int, method string) data.CitationSet {
	v := data.VersionRef{Bill: SampleBill(n), Version: "ih"}
	return data.CitationSet{
		Ref:         data.CitationRef{Version: v, Method: method},
		Release:     "119-142",
		ReleaseDate: SampleRelease("119-142", 10).Date,
		Links: []data.CitationEntry{{
			Version: v, Section: []string{"s2"}, Line: 3, Text: "42 U.S.C. 1395cc-7(a)",
			ProvisionIDs: []string{"usc/42/1395cc-7/a"}, State: data.CitationResolved,
			Kind: data.CiteUSC, Target: "usc/42/1395cc-7/a", Source: data.SourceMarkup,
		}, {
			Version: v, Section: []string{"s3"}, Line: 5, Text: "Public Law 111-163",
			ProvisionIDs: []string{}, State: data.CitationUnresolved,
			Kind: data.CitePublicLaw, Reason: "no source for public law text", Source: data.SourceMarkup,
		}},
		Provisions: []data.CitedProvision{{
			ID: "usc/42/1395cc-7/a", Citation: "42 U.S.C. 1395cc-7(a)", Text: "(a) In general", CurrentAsOf: SampleRelease("119-142", 10).Date, Meta: SampleMeta(),
			Release: "119-142",
		}},
		Freshness: []data.FreshnessFlag{},
		Meta:      data.Meta{Version: 1, AsOf: SampleMeta().AsOf, Provenance: data.ProvenanceAuto, ContentClass: data.ContentPlatform},
	}
}

// Law runs the conformance checks for LawReader and LawWriter. Not checked:
// concurrent writers.
func Law(t *testing.T, factory func(t *testing.T) (data.LawReader, data.LawWriter)) {
	t.Helper()
	cases := []struct {
		name string
		run  func(t *testing.T, r data.LawReader, w data.LawWriter)
	}{
		{"missing", checkLawMissing},
		{"section round trip and write once", checkLawSection},
		{"release write once", checkLawRelease},
		{"current pointer", checkLawCurrent},
		{"invalid", checkLawInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r, w := factory(t)
			c.run(t, r, w)
		})
	}
}

func checkLawMissing(t *testing.T, r data.LawReader, w data.LawWriter) {
	t.Helper()
	ctx := context.Background()
	if _, err := r.CurrentRelease(ctx); !errors.Is(err, data.ErrNotFound) {
		t.Errorf("CurrentRelease empty: err = %v, want ErrNotFound", err)
	}
	if _, err := r.GetRelease(ctx, "119-142"); !errors.Is(err, data.ErrNotFound) {
		t.Errorf("GetRelease unknown: err = %v, want ErrNotFound", err)
	}
	if _, err := r.GetLawSection(ctx, SampleLawSection("119-142").Ref); !errors.Is(err, data.ErrNotFound) {
		t.Errorf("GetLawSection unknown: err = %v, want ErrNotFound", err)
	}
	if err := w.SetCurrentRelease(ctx, "119-142"); !errors.Is(err, data.ErrNotFound) {
		t.Errorf("SetCurrentRelease unknown: err = %v, want ErrNotFound", err)
	}
}

func checkLawSection(t *testing.T, r data.LawReader, w data.LawWriter) {
	t.Helper()
	ctx := context.Background()
	s := SampleLawSection("119-142")
	if err := w.PutLawSection(ctx, s); err != nil {
		t.Fatal(err)
	}
	if err := w.PutLawSection(ctx, s); err != nil {
		t.Errorf("identical put: err = %v, want nil", err)
	}
	got, err := r.GetLawSection(ctx, s.Ref)
	if err != nil || !reflect.DeepEqual(got, s) {
		t.Errorf("round trip: got %+v, err %v; want %+v", got, err, s)
	}
	other := SampleLawSection("119-142")
	other.Provisions[0].Text = "changed"
	if err := w.PutLawSection(ctx, other); !errors.Is(err, data.ErrConflict) {
		t.Errorf("different put: err = %v, want ErrConflict", err)
	}
	// The same section at another release is a different record.
	if err := w.PutLawSection(ctx, SampleLawSection("119-143")); err != nil {
		t.Errorf("other release: %v", err)
	}
}

func checkLawRelease(t *testing.T, r data.LawReader, w data.LawWriter) {
	t.Helper()
	ctx := context.Background()
	rel := SampleRelease("119-142", 10)
	if err := w.PutRelease(ctx, rel); err != nil {
		t.Fatal(err)
	}
	if err := w.PutRelease(ctx, rel); err != nil {
		t.Errorf("identical put: err = %v, want nil", err)
	}
	other := rel
	other.PublicLaw = "119-999"
	if err := w.PutRelease(ctx, other); !errors.Is(err, data.ErrConflict) {
		t.Errorf("different put: err = %v, want ErrConflict", err)
	}
	got, err := r.GetRelease(ctx, rel.ID)
	if err != nil || !reflect.DeepEqual(got, rel) {
		t.Errorf("GetRelease = %+v, %v; want %+v", got, err, rel)
	}
}

func checkLawCurrent(t *testing.T, r data.LawReader, w data.LawWriter) {
	t.Helper()
	ctx := context.Background()
	older, newer := SampleRelease("119-141", 3), SampleRelease("119-142", 10)
	for _, rel := range []data.Release{older, newer} {
		if err := w.PutRelease(ctx, rel); err != nil {
			t.Fatal(err)
		}
	}
	// The pointer moves forward and never back to an older release.
	for _, step := range []struct{ set, want string }{
		{older.ID, older.ID}, {newer.ID, newer.ID}, {older.ID, newer.ID},
	} {
		if err := w.SetCurrentRelease(ctx, step.set); err != nil {
			t.Fatal(err)
		}
		if got, err := r.CurrentRelease(ctx); err != nil || got.ID != step.want {
			t.Fatalf("after set %s: CurrentRelease = %+v, %v; want %s", step.set, got, err, step.want)
		}
	}
}

func checkLawInvalid(t *testing.T, r data.LawReader, w data.LawWriter) {
	t.Helper()
	ctx := context.Background()
	for name, mut := range map[string]func(*data.LawSection){
		"empty release": func(s *data.LawSection) { s.Ref.Release = "" },
		"zero title":    func(s *data.LawSection) { s.Ref.Title = 0 },
		"slash section": func(s *data.LawSection) { s.Ref.Section = "1/2" },
		"empty section": func(s *data.LawSection) { s.Ref.Section = "" },
	} {
		s := SampleLawSection("119-142")
		mut(&s)
		if err := w.PutLawSection(ctx, s); !errors.Is(err, data.ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
		if _, err := r.GetLawSection(ctx, s.Ref); !errors.Is(err, data.ErrInvalid) {
			t.Errorf("%s: get err = %v, want ErrInvalid", name, err)
		}
	}
	rel := SampleRelease("119-142", 10)
	rel.Date = time.Time{}
	if err := w.PutRelease(ctx, rel); !errors.Is(err, data.ErrInvalid) {
		t.Errorf("zero date: err = %v, want ErrInvalid", err)
	}
}

// Citations runs the conformance checks for CitationReader and
// CitationWriter. Not checked: concurrent writers.
func Citations(t *testing.T, factory func(t *testing.T) (data.CitationReader, data.CitationWriter)) {
	t.Helper()
	cases := []struct {
		name string
		run  func(t *testing.T, r data.CitationReader, w data.CitationWriter)
	}{
		{"missing", checkCitationsMissing},
		{"round trip", checkCitationsRoundTrip},
		{"write once", checkCitationsWriteOnce},
		{"invalid", checkCitationsInvalid},
		{"keys differ", checkCitationsKeys},
		{"freshness replaceable", checkFreshnessReplaceable},
		{"freshness invalid", checkFreshnessInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r, w := factory(t)
			c.run(t, r, w)
		})
	}
}

func checkCitationsMissing(t *testing.T, r data.CitationReader, _ data.CitationWriter) {
	t.Helper()
	_, err := r.GetCitations(context.Background(), SampleCitationSet(1, "m/1").Ref)
	if !errors.Is(err, data.ErrNotFound) {
		t.Errorf("GetCitations unknown: err = %v, want ErrNotFound", err)
	}
}

func checkCitationsRoundTrip(t *testing.T, r data.CitationReader, w data.CitationWriter) {
	t.Helper()
	ctx := context.Background()
	s := SampleCitationSet(1, "m/1")
	if err := w.PutCitations(ctx, s); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetCitations(ctx, s.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, s) {
		t.Errorf("round trip:\n got %+v\nwant %+v", got, s)
	}
	if n := len(got.Unresolved()); n != 1 {
		t.Errorf("Unresolved() = %d entries, want 1", n)
	}
}

func checkCitationsWriteOnce(t *testing.T, r data.CitationReader, w data.CitationWriter) {
	t.Helper()
	ctx := context.Background()
	s := SampleCitationSet(1, "m/1")
	if err := w.PutCitations(ctx, s); err != nil {
		t.Fatal(err)
	}
	if err := w.PutCitations(ctx, s); err != nil {
		t.Errorf("identical put: err = %v, want nil", err)
	}
	other := SampleCitationSet(1, "m/1")
	other.Links[0].State = data.CitationAmbiguous
	if err := w.PutCitations(ctx, other); !errors.Is(err, data.ErrConflict) {
		t.Errorf("different put: err = %v, want ErrConflict", err)
	}
	got, err := r.GetCitations(ctx, s.Ref)
	if err != nil || !reflect.DeepEqual(got, s) {
		t.Errorf("after conflict: got %+v, err %v; want the first write", got, err)
	}
}

func checkCitationsInvalid(t *testing.T, _ data.CitationReader, w data.CitationWriter) {
	t.Helper()
	for name, mut := range map[string]func(*data.CitationSet){
		"empty bill":    func(s *data.CitationSet) { s.Ref.Version.Bill = data.BillRef{} },
		"empty version": func(s *data.CitationSet) { s.Ref.Version.Version = "" },
		"empty method":  func(s *data.CitationSet) { s.Ref.Method = "" },
	} {
		s := SampleCitationSet(1, "m/1")
		mut(&s)
		if err := w.PutCitations(context.Background(), s); !errors.Is(err, data.ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}

func checkCitationsKeys(t *testing.T, r data.CitationReader, w data.CitationWriter) {
	t.Helper()
	ctx := context.Background()
	a, b, c := SampleCitationSet(1, "m/1"), SampleCitationSet(1, "m/2"), SampleCitationSet(2, "m/1")
	for _, x := range []data.CitationSet{a, b, c} {
		if err := w.PutCitations(ctx, x); err != nil {
			t.Fatal(err)
		}
	}
	for _, x := range []data.CitationSet{a, b, c} {
		got, err := r.GetCitations(ctx, x.Ref)
		if err != nil || got.Ref != x.Ref {
			t.Errorf("GetCitations(%s) = %v, %v", x.Ref, got.Ref, err)
		}
	}
}

// checkFreshnessReplaceable: a missing doc is not found, a put reads back, and
// a second put replaces it (unlike the write-once set beside it).
func checkFreshnessReplaceable(t *testing.T, r data.CitationReader, w data.CitationWriter) {
	t.Helper()
	ctx := context.Background()
	set := SampleCitationSet(1, "m/1")
	if _, err := r.GetFreshness(ctx, set.Ref); !errors.Is(err, data.ErrNotFound) {
		t.Errorf("GetFreshness unknown: err = %v, want ErrNotFound", err)
	}
	first := data.FreshnessDoc{Ref: set.Ref, Flags: []data.FreshnessFlag{}, Laws: []string{}}
	second := data.FreshnessDoc{
		Ref: set.Ref, Laws: []string{"119-150"},
		Flags: []data.FreshnessFlag{{
			ProvisionID: "usc/42/1395", PublicLaw: "119-150", EnactedAt: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
		}},
	}
	for _, d := range []data.FreshnessDoc{first, second} {
		if err := w.PutFreshness(ctx, d); err != nil {
			t.Fatal(err)
		}
		got, err := r.GetFreshness(ctx, set.Ref)
		if err != nil || !reflect.DeepEqual(got, d) {
			t.Errorf("GetFreshness = %+v, %v; want %+v", got, err, d)
		}
	}
	if _, err := r.GetCitations(ctx, set.Ref); !errors.Is(err, data.ErrNotFound) {
		t.Errorf("a freshness doc is not a citation set: err = %v, want ErrNotFound", err)
	}
	if err := w.PutCitations(ctx, set); err != nil {
		t.Errorf("set beside its freshness doc: %v", err)
	}
}

func checkFreshnessInvalid(t *testing.T, _ data.CitationReader, w data.CitationWriter) {
	t.Helper()
	d := data.FreshnessDoc{Ref: SampleCitationSet(1, "m/1").Ref}
	d.Ref.Method = ""
	if err := w.PutFreshness(context.Background(), d); !errors.Is(err, data.ErrInvalid) {
		t.Errorf("empty method: err = %v, want ErrInvalid", err)
	}
}
