// Package conformance holds one exported suite per port. Every adapter's
// tests call the suites for the ports it implements; an adapter that fails a
// suite cannot be wired into a service.
package conformance

import (
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// SampleBill returns a valid bill ref; n varies the number.
func SampleBill(n int) data.BillRef {
	return data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: n}
}

// SampleMeta returns valid record metadata.
func SampleMeta() data.Meta {
	return data.Meta{
		Version:      1,
		AsOf:         time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Provenance:   data.ProvenanceSource,
		ContentClass: data.ContentOfficial,
	}
}

// SampleVersion returns a valid version of bill n with one section "s1"
// holding lines 1..4; line 1 is depth 0 and lines 2..3 are nested under it.
func SampleVersion(n int, code string) data.OfficialVersion {
	return data.OfficialVersion{
		Ref:          data.VersionRef{Bill: SampleBill(n), Version: code},
		Type:         "introduced",
		PublishedAt:  time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		SourceSHA256: "00",
		Meta:         SampleMeta(),
		Sections: []data.Section{{
			Path:    []string{"s1"},
			Label:   "Sec. 1",
			Heading: "Findings",
			Lines: []data.Line{
				{N: 1, Depth: 0, Text: "Congress finds the following:"},
				{N: 2, Depth: 1, Text: "first finding;"},
				{N: 3, Depth: 1, Text: "second finding."},
				{N: 4, Depth: 0, Text: "This Act takes effect on enactment."},
			},
		}},
	}
}
