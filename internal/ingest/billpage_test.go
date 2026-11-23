package ingest_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/billstatus"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/ingest"
)

func TestDerivedBillPageURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ref  data.BillRef
		want string
	}{
		{data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: 4127}, "https://www.congress.gov/bill/119th-congress/house-bill/4127"},
		{data.BillRef{Jurisdiction: "us", Session: "119", Type: "s", Number: 7}, "https://www.congress.gov/bill/119th-congress/senate-bill/7"},
		{data.BillRef{Jurisdiction: "us", Session: "119", Type: "hjres", Number: 1}, "https://www.congress.gov/bill/119th-congress/house-joint-resolution/1"},
		{data.BillRef{Jurisdiction: "us", Session: "119", Type: "sjres", Number: 2}, "https://www.congress.gov/bill/119th-congress/senate-joint-resolution/2"},
		{data.BillRef{Jurisdiction: "us", Session: "119", Type: "hconres", Number: 3}, "https://www.congress.gov/bill/119th-congress/house-concurrent-resolution/3"},
		{data.BillRef{Jurisdiction: "us", Session: "119", Type: "sconres", Number: 4}, "https://www.congress.gov/bill/119th-congress/senate-concurrent-resolution/4"},
		{data.BillRef{Jurisdiction: "us", Session: "119", Type: "hres", Number: 5}, "https://www.congress.gov/bill/119th-congress/house-resolution/5"},
		{data.BillRef{Jurisdiction: "us", Session: "119", Type: "sres", Number: 6}, "https://www.congress.gov/bill/119th-congress/senate-resolution/6"},
		{data.BillRef{Jurisdiction: "us", Session: "1", Type: "hr", Number: 1}, "https://www.congress.gov/bill/1st-congress/house-bill/1"},
		{data.BillRef{Jurisdiction: "us", Session: "2", Type: "hr", Number: 1}, "https://www.congress.gov/bill/2nd-congress/house-bill/1"},
		{data.BillRef{Jurisdiction: "us", Session: "3", Type: "hr", Number: 1}, "https://www.congress.gov/bill/3rd-congress/house-bill/1"},
		{data.BillRef{Jurisdiction: "us", Session: "11", Type: "hr", Number: 1}, "https://www.congress.gov/bill/11th-congress/house-bill/1"},
		{data.BillRef{Jurisdiction: "us", Session: "12", Type: "hr", Number: 1}, "https://www.congress.gov/bill/12th-congress/house-bill/1"},
		{data.BillRef{Jurisdiction: "us", Session: "13", Type: "hr", Number: 1}, "https://www.congress.gov/bill/13th-congress/house-bill/1"},
		{data.BillRef{Jurisdiction: "us", Session: "21", Type: "hr", Number: 1}, "https://www.congress.gov/bill/21st-congress/house-bill/1"},
		{data.BillRef{Jurisdiction: "us", Session: "22", Type: "hr", Number: 1}, "https://www.congress.gov/bill/22nd-congress/house-bill/1"},
		{data.BillRef{Jurisdiction: "us", Session: "101", Type: "hr", Number: 1}, "https://www.congress.gov/bill/101st-congress/house-bill/1"},
		{data.BillRef{Jurisdiction: "us", Session: "111", Type: "hr", Number: 1}, "https://www.congress.gov/bill/111th-congress/house-bill/1"},
		{data.BillRef{Jurisdiction: "us", Session: "112", Type: "hr", Number: 1}, "https://www.congress.gov/bill/112th-congress/house-bill/1"},
		{data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: 1}, "https://www.congress.gov/bill/119th-congress/house-bill/1"},
		{data.BillRef{Jurisdiction: "ca", Session: "119", Type: "hr", Number: 1}, ""},
		{data.BillRef{Jurisdiction: "us", Session: "119", Type: "xyz", Number: 1}, ""},
		{data.BillRef{Jurisdiction: "us", Session: "x", Type: "hr", Number: 1}, ""},
	}
	for _, tc := range tests {
		if got := ingest.DerivedBillPageURL(tc.ref); got != tc.want {
			t.Errorf("DerivedBillPageURL(%v) = %q, want %q", tc.ref, got, tc.want)
		}
	}
}

// TestCatalogFixturesAllLinkTheBillPage parses every catalog fixture: each ends
// with a bill-page link, the record's own link is preferred, and where a record
// has one the derived link equals it.
func TestCatalogFixturesAllLinkTheBillPage(t *testing.T) {
	t.Parallel()
	root := os.DirFS(filepath.Join("..", "..", "fixtures"))
	files, err := fs.Glob(root, "catalog/us-*.xml")
	if err != nil || len(files) == 0 {
		t.Fatalf("catalog fixtures: %v (%d files)", err, len(files))
	}
	for _, f := range files {
		raw, err := fs.ReadFile(root, f)
		if err != nil {
			t.Fatal(err)
		}
		rec, err := billstatus.Parse(raw, data.SourceInfo{URL: "https://example.invalid/status.xml"})
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		checkBillPageLink(t, f, rec)
	}
}

// checkBillPageLink checks the bill-page link of one parsed fixture.
func checkBillPageLink(t *testing.T, name string, rec data.BillRecord) {
	t.Helper()
	links := ingest.OfficialLinks(rec)
	i := slices.IndexFunc(links, func(l data.Link) bool { return l.Kind == data.LinkBillPage })
	if i < 0 {
		t.Errorf("%s: no bill-page link", name)
		return
	}
	l := links[i]
	if rec.BillPageURL == "" {
		if l.Label != ingest.BillPageLabelDerived {
			t.Errorf("%s: derived link not labelled: %+v", name, l)
		}
		return
	}
	if l.URL != rec.BillPageURL || l.Label != "Congress.gov" {
		t.Errorf("%s: link %+v does not keep the record's own %q", name, l, rec.BillPageURL)
	}
	if got := ingest.DerivedBillPageURL(rec.Bill); got != rec.BillPageURL {
		t.Errorf("%s: derived %q differs from the record's %q", name, got, rec.BillPageURL)
	}
}

func TestEvaluateAllFillsMissingBillPage(t *testing.T) {
	t.Parallel()
	store, d, p := storeCatalog(t)
	entries := allEntries(t, store)
	for _, e := range entries {
		e.OfficialLinks = slices.DeleteFunc(slices.Clone(e.OfficialLinks), func(l data.Link) bool { return l.Kind == data.LinkBillPage })
		if err := store.PutCatalogEntry(t.Context(), e); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ingest.EvaluateAll(t.Context(), d, p, false); err != nil {
		t.Fatalf("EvaluateAll: %v", err)
	}
	for _, e := range allEntries(t, store) {
		if countLinks(e.OfficialLinks, data.LinkBillPage) != 1 {
			t.Errorf("%s: bill-page links = %d, want 1", e.Bill.ID(), countLinks(e.OfficialLinks, data.LinkBillPage))
		}
	}
}
