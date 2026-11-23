package memory_test

import (
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/data/conformance"
)

// statusIntroduced is the status text of every seeded bill.
const statusIntroduced = "introduced"

func seededBill(n int) data.BillRef {
	return data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: n}
}

func TestBillSource(t *testing.T) {
	t.Parallel()
	conformance.BillSource(t, func(t *testing.T) conformance.BillSourceFixture {
		t.Helper()
		s := memory.New()
		version := data.VersionRef{Bill: seededBill(1), Version: "ih"}
		changed := []data.ChangedBill{
			{Bill: seededBill(1), UpdatedAt: "2026-01-01T00:00:00Z"},
			{Bill: seededBill(2), UpdatedAt: "2026-01-02T00:00:00Z"},
			{Bill: seededBill(3), UpdatedAt: "2026-01-03T00:00:00Z"},
		}
		statuses := []data.BillStatus{
			{
				Bill:    seededBill(1),
				Title:   "Example Act",
				Sponsor: "Rep. Example",
				Status:  statusIntroduced,
				Versions: []data.VersionSummary{{
					Ref:         version,
					Type:        "introduced",
					PublishedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				}},
			},
			{Bill: seededBill(2), Title: "Second Act", Sponsor: "Sen. Example", Status: statusIntroduced, Versions: nil},
			{Bill: seededBill(3), Title: "Third Act", Sponsor: "Rep. Example", Status: statusIntroduced, Versions: nil},
		}
		s.SeedBills(changed, statuses, map[data.VersionRef][]byte{version: []byte("<bill/>")})
		return conformance.BillSourceFixture{
			Source:  s,
			Changed: changed,
			Status:  statuses[0],
			Version: version,
			XML:     []byte("<bill/>"),
		}
	})
}

func TestLawSource(t *testing.T) {
	t.Parallel()
	conformance.LawSource(t, func(t *testing.T) conformance.LawSourceFixture {
		t.Helper()
		s := memory.New()
		points := []data.ReleasePoint{{ID: "119-2"}, {ID: "119-1"}}
		titles := map[data.ReleasePoint]map[int][]byte{
			{ID: "119-2"}: {42: []byte("<title/>")},
			{ID: "119-1"}: {42: []byte("<title/>")},
		}
		s.SeedLaw(points, titles)
		return conformance.LawSourceFixture{
			Source: s,
			Points: points,
			Title:  42,
			XML:    []byte("<title/>"),
		}
	})
}

func TestGenerator(t *testing.T) {
	t.Parallel()
	conformance.Generator(t, func(t *testing.T) data.Generator {
		t.Helper()
		return memory.New()
	})
}
