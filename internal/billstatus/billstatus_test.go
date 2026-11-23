package billstatus_test

import (
	"errors"
	"io/fs"
	"os"
	"reflect"
	"slices"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/billstatus"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

// fixturesDir is the repository fixtures directory, relative to this package.
const fixturesDir = "../../fixtures"

// hr4313Path is the BILLSTATUS fixture of H.R. 4313, relative to fixturesDir.
const hr4313Path = "bills/us-119-hr-4313/status.xml"

// versionSummaries lists each version as "code url", in record order.
func versionSummaries(vs []data.SourceVersion) []string {
	out := make([]string, 0, len(vs))
	for _, v := range vs {
		out = append(out, v.Ref.Version+" "+v.URL)
	}
	return out
}

func TestParseFixtureHR4313(t *testing.T) {
	t.Parallel()
	raw, err := fs.ReadFile(os.DirFS(fixturesDir), hr4313Path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	src := data.SourceInfo{URL: "https://example.invalid/status.xml"}
	rec, err := billstatus.Parse(raw, src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	wantVersions := []string{
		"ih https://www.govinfo.gov/content/pkg/BILLS-119hr4313ih/xml/BILLS-119hr4313ih.xml",
		"rh https://www.govinfo.gov/content/pkg/BILLS-119hr4313rh/xml/BILLS-119hr4313rh.xml",
		"eh https://www.govinfo.gov/content/pkg/BILLS-119hr4313eh/xml/BILLS-119hr4313eh.xml",
		"rfs https://www.govinfo.gov/content/pkg/BILLS-119hr4313rfs/xml/BILLS-119hr4313rfs.xml",
	}
	tests := []struct {
		name string
		got  any
		want any
	}{
		{"bill", rec.Bill, data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: 4313}},
		{"title", rec.Title, "Hospital Inpatient Services Modernization Act"},
		{"chamber", rec.Chamber, data.ChamberHouse},
		{"sponsor id", rec.SponsorID, "B001260"},
		{"sponsor party", rec.SponsorParty, "R"},
		{"policy area", rec.PolicyArea, "Health"},
		{"subject Medicare", slices.Contains(rec.Subjects, "Medicare"), true},
		{"versions in order with URLs", versionSummaries(rec.Versions), wantVersions},
		{"committee activity Markup By", slices.ContainsFunc(rec.CommitteeActivity, func(a data.CommitteeActivity) bool {
			return a.Activity == "Markup By"
		}), true},
		{"bill page URL", rec.BillPageURL, "https://www.congress.gov/bill/119th-congress/house-bill/4313"},
		{"latest action date", rec.LatestAction.Date, "2025-12-02"},
		{"source copied", rec.Source, src},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if !reflect.DeepEqual(tc.got, tc.want) {
				t.Errorf("got %v, want %v", tc.got, tc.want)
			}
		})
	}
}

func TestParseFixturesAccepted(t *testing.T) {
	t.Parallel()
	fixtures := os.DirFS(fixturesDir)
	var files []string
	for _, pattern := range []string{
		"bills/*/status.xml",
		"catalog/*.xml",
	} {
		matches, err := fs.Glob(fixtures, pattern)
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}
		files = append(files, matches...)
	}
	if len(files) == 0 {
		t.Skip("no BILLSTATUS fixtures present")
	}
	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			t.Parallel()
			raw, err := fs.ReadFile(fixtures, file)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			rec, err := billstatus.Parse(raw, data.SourceInfo{URL: "fixture"})
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if rec.Bill.Number <= 0 {
				t.Errorf("bill number %d, want > 0", rec.Bill.Number)
			}
			if rec.Title == "" {
				t.Error("empty title")
			}
		})
	}
}

func TestParseInvalid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		xml  string
	}{
		{"malformed XML", `<billStatus><bill><number>1</number>`},
		{"no number", `<billStatus><bill><congress>119</congress><type>HR</type></bill></billStatus>`},
		{"empty input", ``},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := billstatus.Parse([]byte(tc.xml), data.SourceInfo{})
			if !errors.Is(err, data.ErrInvalid) {
				t.Errorf("err = %v, want errors.Is ErrInvalid", err)
			}
		})
	}
}
