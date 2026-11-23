// Package seed writes a small fixture set of invented bills through the
// platform ports. Contract tests, the memory build with SEED_FIXTURES, and
// the compose acceptance script use it; it is never run in a deployed
// environment. All text is fictional.
package seed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// Bills the fixture set holds, as read-api bill ids.
const (
	TrackedBill    = "us-119-hr-4127" // two versions, explanations, nested lines
	TrackedOneBill = "us-119-s-88"    // one version
	CatalogBill    = "us-119-hr-9"    // untracked: metadata and links only
)

// Fixture numbers and days (of March 2026).
const (
	clinicNumber = 4127
	trailNumber  = 88
	namingNumber = 9

	dayFirst    = 1
	dayNaming   = 2
	dayTrail    = 3
	dayPolicy2  = 5
	dayLog      = 6
	dayReported = 8
	dayExplain  = 9

	policyOne = 1
	policyTwo = 2

	typeIntroduced = "introduced"
	headingTitle   = "Short title"
	labelSection1  = "Section 1."
	linkLabel      = "Congress.gov"
	nestEvery      = 3 // every third line is top level, the rest nest
)

func at(day int) time.Time { return time.Date(2026, time.March, day, 12, 0, 0, 0, time.UTC) }

func meta(class data.ContentClass, prov data.Provenance, day int) data.Meta {
	return data.Meta{Version: 1, AsOf: at(day), Provenance: prov, ContentClass: class}
}

func official(day int) data.Meta { return meta(data.ContentOfficial, data.ProvenanceSource, day) }

func platform(prov data.Provenance, day int) data.Meta { return meta(data.ContentPlatform, prov, day) }

func link(url string) []data.Link {
	return []data.Link{{Label: linkLabel, URL: url, Kind: data.LinkBillPage}}
}

func refOf(typ string, n int) data.BillRef {
	return data.BillRef{Jurisdiction: "us", Session: "119", Type: typ, Number: n}
}

// lines numbers the texts from 1; every third line is top level and the
// others nest under it, so blocks have depth to show.
func lines(texts ...string) []data.Line {
	out := make([]data.Line, len(texts))
	for i, t := range texts {
		depth := 0
		if i%nestEvery != 0 {
			depth = 1
		}
		out[i] = data.Line{N: i + 1, Depth: depth, Text: t}
	}
	return out
}

func section(path []string, label, heading string, texts ...string) data.Section {
	return data.Section{Path: path, Label: label, Heading: heading, Lines: lines(texts...)}
}

// stored is one version with its source XML.
type stored struct {
	v data.OfficialVersion
	x []byte
}

func version(ref data.BillRef, code, typ string, day int, secs ...data.Section) stored {
	xml := fmt.Appendf(nil, "<bill><!-- fixture %s %s --></bill>", ref, code)
	sum := sha256.Sum256(xml)
	return stored{x: xml, v: data.OfficialVersion{
		Ref: data.VersionRef{Bill: ref, Version: code}, Type: typ, PublishedAt: at(day),
		SourceSHA256: hex.EncodeToString(sum[:]), Sections: secs, Meta: official(day),
	}}
}

func summaries(vs ...stored) []data.VersionSummary {
	out := make([]data.VersionSummary, len(vs))
	for i, s := range vs {
		out[i] = data.VersionSummary{Ref: s.v.Ref, Type: s.v.Type, PublishedAt: s.v.PublishedAt}
	}
	return out
}

func versions() []stored {
	clinic, trail := refOf("hr", clinicNumber), refOf("s", trailNumber)
	hours := []string{
		"The Secretary shall publish clinic hours:", "(a) weekday hours;", "(b) weekend hours;",
		"Clinics shall post the hours.", "(1) at the entrance;", "(2) online.",
	}
	reported := append([]string{}, hours...)
	reported[2] = "(b) weekend and holiday hours;"
	short := section([]string{"s1"}, labelSection1, headingTitle, "This Act may be cited as the Example Clinic Act.")
	return []stored{
		version(clinic, "ih", typeIntroduced, dayFirst,
			short, section([]string{"s4"}, "Sec. 4.", "Clinic hours", hours...)),
		version(clinic, "rh", "reported", dayReported,
			short, section([]string{"s4"}, "Sec. 4.", "Clinic hours", reported...),
			section([]string{"s4", "b"}, "(b)", "Notice", "Notice is given by mail.", "(i) in two languages.")),
		version(trail, "is", typeIntroduced, dayTrail,
			section([]string{"s1"}, labelSection1, headingTitle, "This Act is the Example Trail Act.")),
	}
}

func bills(vs []stored) []data.Bill {
	return []data.Bill{{
		Ref: refOf("hr", clinicNumber), Title: "Example Clinic Act", Sponsor: "Rep. Example", Status: "reported",
		Chamber: data.ChamberHouse, ReviewState: data.ReviewOpen, PolicyVersion: policyTwo,
		TrackedReasons: []string{"policy v2: committee reported"}, OfficialLinks: link("https://example.test/hr4127"),
		Versions: summaries(vs[0], vs[1]), Meta: official(dayReported),
	}, {
		Ref: refOf("s", trailNumber), Title: "Example Trail Act", Sponsor: "Sen. Sample", Status: typeIntroduced,
		Chamber: data.ChamberSenate, ReviewState: data.ReviewOpen, PolicyVersion: policyTwo,
		TrackedReasons: []string{"policy v2: sponsor request"}, OfficialLinks: link("https://example.test/s88"),
		Versions: summaries(vs[2]), Meta: official(dayTrail),
	}}
}

func entries() []data.CatalogEntry {
	return []data.CatalogEntry{{
		Bill: refOf("hr", clinicNumber), Title: "Example Clinic Act", Status: "reported", Chamber: data.ChamberHouse,
		Subjects: []string{"Health"}, CommitteeActivity: true, Tracked: true, Reasons: []string{"committee reported"},
		MarkupStatus: data.MarkupInReview, OfficialLinks: link("https://example.test/hr4127"), Meta: official(dayReported),
	}, {
		Bill: refOf("s", trailNumber), Title: "Example Trail Act", Status: typeIntroduced, Chamber: data.ChamberSenate,
		Subjects: []string{"Public lands"}, Tracked: true, Reasons: []string{"sponsor request"},
		MarkupStatus: data.MarkupInReview, OfficialLinks: link("https://example.test/s88"), Meta: official(dayTrail),
	}, {
		Bill: refOf("hr", namingNumber), Title: "Example Post Office Naming Act", Status: typeIntroduced,
		Chamber: data.ChamberHouse, Subjects: []string{"Postal service"}, MarkupStatus: data.MarkupNotACandidate,
		ExclusionReason: "naming bill", OfficialLinks: link("https://example.test/hr9"), Meta: official(dayNaming),
	}}
}

func policies() []data.SelectionPolicy {
	steward := data.ProvenanceStewardEntered
	return []data.SelectionPolicy{
		{Version: policyOne, Rules: []string{"Bills that a committee has reported."}, Meta: platform(steward, dayFirst)},
		{
			Version: policyTwo,
			Rules:   []string{"Bills that a committee has reported.", "Bills a sponsor asks us to review."},
			Meta:    platform(steward, dayPolicy2),
		},
	}
}

// putAll applies put to every item, naming what failed.
func putAll[T any](what string, items []T, put func(T) error) error {
	for _, it := range items {
		if err := put(it); err != nil {
			return fmt.Errorf("seed %s: %w", what, err)
		}
	}
	return nil
}

// Apply writes the fixture set. It is safe to run again.
func Apply(ctx context.Context, p data.PlatformPorts) error {
	vs := versions()
	expl := data.ChangeExplanation{
		Version: vs[1].v.Ref, Section: []string{"s4"}, ProvisionID: "usc/42/1395", CurrentText: "Clinics may post hours.",
		Anchors:     []string{"s4-l4-6"},
		WhatChanges: "Clinics must post hours.", WhyMatters: "Patients can plan visits.",
		Flags: []data.Provenance{data.ProvenanceAuto}, AsAmended: "Clinics must post hours at the entrance.",
		Meta: platform(data.ProvenanceAuto, dayExplain),
	}
	// Stored before planning skipped short-title sections; read-api does not serve it.
	shortExpl := data.ChangeExplanation{
		Version: vs[1].v.Ref, Section: []string{"s1"}, WhatChanges: "This section names the Act.",
		Flags: []data.Provenance{data.ProvenanceAuto, data.ProvenanceBillOnly},
		Meta:  platform(data.ProvenanceAuto, dayExplain),
	}
	entry := data.LogEntry{
		ID: "log-0001", At: at(dayLog), Kind: "hide", Subject: "/v1/bills/" + TrackedBill, Reason: "duplicate entry",
		Meta: platform(data.ProvenanceStewardEntered, dayLog),
	}
	steps := []func() error{
		func() error {
			return putAll("version", vs, func(s stored) error { return p.Publisher.PutVersion(ctx, s.v, s.x) })
		},
		func() error {
			return putAll("bill", bills(vs), func(b data.Bill) error { return p.PlatformDocs.PutBill(ctx, b) })
		},
		func() error {
			return putAll("catalog", entries(), func(e data.CatalogEntry) error {
				return p.PlatformDocs.PutCatalogEntry(ctx, e)
			})
		},
		func() error {
			return putAll("policy", policies(), func(s data.SelectionPolicy) error {
				return p.PlatformDocs.PutSelectionPolicy(ctx, s)
			})
		},
		func() error { return p.PlatformDocs.PutExplanation(ctx, expl) },
		func() error { return p.PlatformDocs.PutExplanation(ctx, shortExpl) },
		func() error { return p.Moderation.Record(ctx, entry) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}
