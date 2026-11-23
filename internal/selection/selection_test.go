package selection_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/selection"
)

func testTime() time.Time { return time.Date(2026, time.November, 22, 12, 0, 0, 0, time.UTC) }

func v1(t *testing.T) data.SelectionPolicy {
	t.Helper()
	p, err := selection.V1(testTime())
	if err != nil {
		t.Fatalf("V1: %v", err)
	}
	return p
}

func ref(typ string, n int) data.BillRef {
	return data.BillRef{Jurisdiction: "us", Session: "119", Type: typ, Number: n}
}

func act(name string) []data.CommitteeActivity {
	return []data.CommitteeActivity{{Committee: "Committee", Activity: name, At: testTime()}}
}

func TestInputHasNoSponsorFields(t *testing.T) {
	t.Parallel()
	typ := reflect.TypeFor[selection.Input]()
	for f := range typ.Fields() {
		n := strings.ToLower(f.Name)
		for _, banned := range []string{"party", "sponsor", "position", "outcome"} {
			if strings.Contains(n, banned) {
				t.Errorf("Input has field %q: the evaluator must not read %s", f.Name, banned)
			}
		}
	}
}

func TestV1(t *testing.T) {
	t.Parallel()
	p := v1(t)
	if p.Version != 1 || p.Mode != data.PolicyModePilot || p.Phase != data.PolicyPhaseEarlyPilot {
		t.Errorf("v1 = version %d mode %q phase %q", p.Version, p.Mode, p.Phase)
	}
	for _, topic := range []string{"healthcare", "deficit_reduction", "affordability"} {
		if _, ok := p.Topics[topic]; !ok {
			t.Errorf("v1 has no topic %q", topic)
		}
	}
	want := "Bills and joint resolutions with committee action on affordability, deficit reduction, and healthcare."
	if got := selection.Scope(p); got != want {
		t.Errorf("Scope = %q, want %q", got, want)
	}
	if len(p.Rules) < 4 {
		t.Errorf("Rules = %v, want the plain-language statement", p.Rules)
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()
	good := v1(t)
	tests := []struct {
		name   string
		mutate func(*data.SelectionPolicy)
		want   string
	}{
		{"zero version", func(p *data.SelectionPolicy) { p.Version = 0 }, "version"},
		{"bad mode", func(p *data.SelectionPolicy) { p.Mode = "wide" }, "mode"},
		{"bad phase", func(p *data.SelectionPolicy) { p.Phase = "" }, "phase"},
		{"no measure kinds", func(p *data.SelectionPolicy) { p.MeasureKinds = nil }, "measure_kinds"},
		{"unknown measure kind", func(p *data.SelectionPolicy) { p.MeasureKinds = []string{"memo"} }, "unknown value"},
		{"unknown activity", func(p *data.SelectionPolicy) { p.CommitteeActivity = []string{"referral"} }, "committee_activity"},
		{"pilot without topics", func(p *data.SelectionPolicy) { p.Topics = nil }, "topic"},
		{"empty topic", func(p *data.SelectionPolicy) { p.Topics = map[string]data.PolicyTopic{"x": {}} }, "no terms"},
		{"bad topic name", func(p *data.SelectionPolicy) {
			p.Topics = map[string]data.PolicyTopic{"Health Care": {Subjects: []string{"a"}}}
		}, "snake_case"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := good
			tt.mutate(&p)
			err := selection.Validate(p)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Validate = %v, want error containing %q", err, tt.want)
			}
		})
	}
	if err := selection.Validate(good); err != nil {
		t.Errorf("Validate(v1) = %v", err)
	}
	if _, err := selection.Parse([]byte(`{"version":1,"mod":"pilot"}`), testTime()); err == nil {
		t.Error("Parse accepted an unknown field")
	}
}

func TestEvaluate(t *testing.T) {
	t.Parallel()
	for _, tt := range evalCases(t) {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := v1(t)
			if tt.policy != nil {
				p = tt.policy(t)
			}
			isTracked := func(b data.BillRef) bool { return slices.Contains(tt.tracked, b) }
			got := selection.Evaluate(p, tt.in, nil, isTracked)
			if got.MarkupStatus != tt.wantStatus {
				t.Errorf("status = %q, want %q", got.MarkupStatus, tt.wantStatus)
			}
			if got.Tracked != (tt.wantStatus == data.MarkupInReview) {
				t.Errorf("tracked = %v for status %q", got.Tracked, got.MarkupStatus)
			}
			if got.Tracked {
				if !slices.Equal(got.Reasons, tt.wantReason) {
					t.Errorf("reasons = %v, want %v", got.Reasons, tt.wantReason)
				}
				if got.PolicyVersion != p.Version {
					t.Errorf("policy version = %d, want %d", got.PolicyVersion, p.Version)
				}
			}
			if got.ExclusionReason != tt.wantExcl {
				t.Errorf("exclusion = %q, want %q", got.ExclusionReason, tt.wantExcl)
			}
		})
	}
}

func TestTrackedStaysTracked(t *testing.T) {
	t.Parallel()
	narrow := v1(t)
	narrow.Version = 2
	narrow.Topics = map[string]data.PolicyTopic{"healthcare": {Subjects: []string{"Medicare"}}}
	in := selection.Input{Bill: ref("hr", 20), Title: "To reduce the national debt", Committees: act("Referred To")}
	prior := &selection.Prior{Reasons: []string{"committee-action:markup", "topic:deficit_reduction:title-only"}, PolicyVersion: 1}
	got := selection.Evaluate(narrow, in, prior, nil)
	if !got.Tracked || got.PolicyVersion != 1 || got.MarkupStatus != data.MarkupInReview {
		t.Errorf("result = %+v, want tracked under policy version 1", got)
	}
	if !slices.Equal(got.Reasons, prior.Reasons) {
		t.Errorf("reasons = %v, want unchanged %v", got.Reasons, prior.Reasons)
	}
	// An excluded kind that was tracked stays tracked too.
	in.Bill = ref("hres", 21)
	if got := selection.Evaluate(narrow, in, prior, nil); !got.Tracked {
		t.Error("tracked bill of an excluded kind was untracked")
	}
}

func TestSubjectsConfirmTitleOnly(t *testing.T) {
	t.Parallel()
	p := v1(t)
	in := selection.Input{Bill: ref("hr", 22), Title: "Medicaid access", Subjects: []string{"Medicaid"}}
	prior := &selection.Prior{Reasons: []string{"committee-action:markup", "topic:healthcare:title-only"}, PolicyVersion: 1}
	got := selection.Evaluate(p, in, prior, nil)
	want := []string{"committee-action:markup", "topic:healthcare:subject"}
	if !slices.Equal(got.Reasons, want) {
		t.Errorf("reasons = %v, want %v", got.Reasons, want)
	}
}

func TestPublish(t *testing.T) {
	t.Parallel()
	store := &fakeStore{}
	p := v1(t)
	if err := selection.Publish(t.Context(), store, p, "", testTime()); err == nil {
		t.Error("Publish accepted an empty reason")
	}
	if err := selection.Publish(t.Context(), store, p, "initial", testTime()); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if store.logged.Reason != "initial" || store.logged.Kind != "selection-policy" {
		t.Errorf("log entry = %+v", store.logged)
	}
	if err := selection.Publish(t.Context(), store, p, "again", testTime().Add(time.Hour)); err != nil {
		t.Errorf("republish of identical content: %v", err)
	}
	changed := p
	changed.Mode = data.PolicyModeGeneral
	if err := selection.Publish(t.Context(), store, changed, "edit", testTime()); err == nil {
		t.Error("Publish changed a stored version")
	}
	p2 := p
	p2.Version = 2
	if err := selection.Publish(t.Context(), store, p2, "narrower", testTime()); err != nil {
		t.Errorf("Publish v2: %v", err)
	}
	if err := selection.Publish(t.Context(), store, p, "old", testTime()); err == nil {
		t.Error("Publish accepted a version below the latest")
	}
}

func TestBreakdown(t *testing.T) {
	t.Parallel()
	entries := []data.CatalogEntry{
		{Tracked: true, SponsorParty: "R", MarkupStatus: data.MarkupInReview, Reasons: []string{"committee-action:markup", "topic:healthcare:subject", "topic:affordability:title-only"}},
		{Tracked: true, SponsorParty: "D", MarkupStatus: data.MarkupInReview, Reasons: []string{"topic:healthcare:title-only", "companion:us-119-hr-1"}},
		{Tracked: true, MarkupStatus: data.MarkupInReview, Reasons: []string{"manual-pull"}},
		{MarkupStatus: data.MarkupEligible, SponsorParty: "R"},
	}
	b := selection.Compute(entries, 1, testTime())
	want := selection.Breakdown{
		AsOf: testTime(), PolicyVersion: 1, CatalogEntries: 4, Tracked: 3, ByCompanion: 1, ByAdministrator: 1,
		ByTopic:          map[string]int{"healthcare": 2, "affordability": 1},
		ByTopicTitleOnly: map[string]int{"healthcare": 1, "affordability": 1},
		BySponsorParty:   map[string]int{"R": 1, "D": 1, "unknown": 1},
		ByMarkupStatus:   map[string]int{data.MarkupInReview: 3, data.MarkupEligible: 1},
	}
	if !reflect.DeepEqual(b, want) {
		t.Errorf("breakdown = %+v, want %+v", b, want)
	}
}
