package fixturesrc_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/adapters/fixturesrc"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/data/conformance"
)

// fixturesDir is the repository fixtures directory, relative to this package.
const fixturesDir = "../../../fixtures"

// fixedNow is the clock every test source uses.
func fixedNow() time.Time { return time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC) }

// hr4313Ref is the exit bill the suite checks against.
func hr4313Ref() data.BillRef {
	return data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: 4313}
}

const hr4313Title = "Hospital Inpatient Services Modernization Act"

func newSource(t *testing.T) *fixturesrc.Source {
	t.Helper()
	src, err := fixturesrc.New(fixturesrc.Config{FS: os.DirFS(fixturesDir), Now: fixedNow})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return src
}

// versionByCode returns the listed version with code, or fails the test.
func versionByCode(t *testing.T, vs []data.SourceVersion, code string) data.SourceVersion {
	t.Helper()
	for _, v := range vs {
		if v.Ref.Version == code {
			return v
		}
	}
	t.Fatalf("no listed version with code %q", code)
	return data.SourceVersion{}
}

func TestBillFetcherConformance(t *testing.T) {
	t.Parallel()
	conformance.BillFetcher(t, func(t *testing.T) conformance.FetcherFixture {
		t.Helper()
		src := newSource(t)
		rec, err := src.FetchRecord(context.Background(), hr4313Ref())
		if err != nil {
			t.Fatalf("FetchRecord: %v", err)
		}
		orig := versionByCode(t, rec.Versions, "ih")
		xml, err := fs.ReadFile(os.DirFS(fixturesDir), path.Join("bills", hr4313Ref().ID(), "v", "ih", "source.xml"))
		if err != nil {
			t.Fatalf("read fixture: %v", err)
		}
		return conformance.FetcherFixture{
			Fetcher:      src,
			Bill:         hr4313Ref(),
			Title:        hr4313Title,
			VersionCodes: []string{"ih", "rh", "eh", "rfs"},
			Original:     orig,
			OriginalXML:  xml,
			Missing:      data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: 999999},
		}
	})
}

func TestCatalogOnlyRecord(t *testing.T) {
	t.Parallel()
	fsys := os.DirFS(fixturesDir)
	files, err := fs.Glob(fsys, "catalog/*.xml")
	if err != nil {
		t.Fatalf("glob catalog: %v", err)
	}
	if len(files) == 0 {
		t.Skip("no catalog files in fixtures")
	}
	ref, err := data.ParseBillID(strings.TrimSuffix(path.Base(files[0]), ".xml"))
	if err != nil {
		t.Fatalf("ParseBillID(%s): %v", files[0], err)
	}
	src := newSource(t)
	rec, err := src.FetchRecord(context.Background(), ref)
	if err != nil {
		t.Fatalf("FetchRecord(%s): %v", ref, err)
	}
	if rec.Bill != ref {
		t.Fatalf("FetchRecord Bill = %s, want %s", rec.Bill, ref)
	}
	if len(rec.Versions) == 0 {
		t.Skip("catalog record lists no text versions")
	}
	_, err = src.FetchOriginal(context.Background(), rec.Versions[0])
	if !errors.Is(err, data.ErrNotFound) {
		t.Fatalf("FetchOriginal of a catalog-only version: err = %v, want ErrNotFound", err)
	}
}

func TestNewNilFSIsInvalid(t *testing.T) {
	t.Parallel()
	_, err := fixturesrc.New(fixturesrc.Config{})
	if !errors.Is(err, data.ErrInvalid) {
		t.Fatalf("New with nil FS: err = %v, want ErrInvalid", err)
	}
}

func TestLawSource(t *testing.T) {
	t.Parallel()
	src := newSource(t)
	ctx := context.Background()
	rps, err := src.ListReleasePoints(ctx)
	if err != nil || len(rps) != 1 || rps[0].ID != "119-142" || rps[0].Date.Format(time.DateOnly) != "2026-11-10" {
		t.Fatalf("ListReleasePoints = %v, %v", rps, err)
	}
	titles, err := src.LawTitles(ctx, rps[0])
	if err != nil || len(titles) != 5 {
		t.Fatalf("LawTitles = %v, %v", titles, err)
	}
	b, err := src.FetchTitle(ctx, rps[0], 42)
	if err != nil || !strings.HasPrefix(string(b), `<uscDoc identifier="/us/usc/t42">`) {
		t.Fatalf("FetchTitle 42: %v", err)
	}
	if _, err := src.FetchTitle(ctx, rps[0], 7); !errors.Is(err, data.ErrNotFound) {
		t.Fatalf("FetchTitle 7 err = %v, want ErrNotFound", err)
	}
}
