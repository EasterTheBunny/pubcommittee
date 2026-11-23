// Package selection evaluates the published bill selection policy against a
// catalog entry ([architecture § Bill selection]). Evaluation is a pure
// function: it reads process signals and the legislature's own subject
// indexing only. Sponsor, party, position, and predicted outcome are not
// fields of Input, so no rule can read them (Input is checked by a test).
package selection

import (
	_ "embed" // policies/v1.json
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// Measure kinds of a policy's measure_kinds.
const (
	KindBill                 = "bill"
	KindJointResolution      = "joint_resolution"
	KindSimpleResolution     = "simple_resolution"
	KindConcurrentResolution = "concurrent_resolution"
)

// Exclude kinds of a policy's exclude_kinds.
const (
	ExcludeWarPowers     = "war_powers_resolution"
	ExcludeCommemorative = "commemorative"
)

// Committee activity kinds of a policy's committee_activity. Referral alone
// is not one of them.
const (
	ActHearing    = "hearing"
	ActMarkup     = "markup"
	ActReported   = "reported"
	ActDischarged = "discharged"
	ActFloor      = "floor"
	ActCalendar   = "calendar"
)

func measureKinds() []string {
	return []string{KindBill, KindJointResolution, KindSimpleResolution, KindConcurrentResolution}
}

func excludeKinds() []string { return []string{ExcludeWarPowers, ExcludeCommemorative} }

func activityKinds() []string {
	return []string{ActHearing, ActMarkup, ActReported, ActDischarged, ActFloor, ActCalendar}
}

//go:embed policies/v1.json
var v1JSON []byte

// V1 returns the initial policy version (pilot mode, early pilot phase) with
// its plain-language rules filled in and Meta set from at. The topic terms are
// drafts; policies/README.md lists their sources.
func V1(at time.Time) (data.SelectionPolicy, error) {
	return Parse(v1JSON, at)
}

// Parse decodes a policy document, validates it, fills Rules from the
// structured fields, and sets Meta (version 1 of the record, platform class).
// Unknown fields are an error so a misspelt key cannot silently drop a rule.
func Parse(doc []byte, at time.Time) (data.SelectionPolicy, error) {
	var p data.SelectionPolicy
	dec := json.NewDecoder(strings.NewReader(string(doc)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return data.SelectionPolicy{}, fmt.Errorf("selection policy: %w: %w", data.ErrInvalid, err)
	}
	if err := Validate(p); err != nil {
		return data.SelectionPolicy{}, err
	}
	p.Rules = Rules(p)
	p.Meta = data.Meta{Version: 1, AsOf: at.UTC(), Provenance: data.ProvenanceSource, ContentClass: data.ContentPlatform}
	return p, nil
}

// Validate checks a policy document. Every failure wraps data.ErrInvalid.
func Validate(p data.SelectionPolicy) error {
	var errs []string
	bad := func(format string, a ...any) { errs = append(errs, fmt.Sprintf(format, a...)) }
	if p.Version < 1 {
		bad("version must be 1 or more")
	}
	if p.Mode != data.PolicyModePilot && p.Mode != data.PolicyModeGeneral {
		bad("mode must be %q or %q", data.PolicyModePilot, data.PolicyModeGeneral)
	}
	if p.Phase != data.PolicyPhaseEarlyPilot && p.Phase != data.PolicyPhaseStewardSignoff {
		bad("phase must be %q or %q", data.PolicyPhaseEarlyPilot, data.PolicyPhaseStewardSignoff)
	}
	checkSet(p.MeasureKinds, "measure_kinds", measureKinds(), true, bad)
	checkSet(p.ExcludeKinds, "exclude_kinds", excludeKinds(), false, bad)
	checkSet(p.CommitteeActivity, "committee_activity", activityKinds(), true, bad)
	validateTopics(p, bad)
	if len(errs) > 0 {
		return fmt.Errorf("selection policy: %w: %s", data.ErrInvalid, strings.Join(errs, "; "))
	}
	return nil
}

// checkSet reports values outside the allowed set, and an empty list when required.
func checkSet(got []string, field string, allowed []string, required bool, bad func(string, ...any)) {
	if required && len(got) == 0 {
		bad("%s must not be empty", field)
	}
	seen := map[string]bool{}
	for _, g := range got {
		if !slices.Contains(allowed, g) {
			bad("%s: unknown value %q (allowed: %s)", field, g, strings.Join(allowed, ", "))
		}
		if seen[g] {
			bad("%s: duplicate value %q", field, g)
		}
		seen[g] = true
	}
}

// validateTopics checks the focus topics of a policy.
func validateTopics(p data.SelectionPolicy, bad func(string, ...any)) {
	if p.Mode == data.PolicyModePilot && len(p.Topics) == 0 {
		bad("pilot mode needs at least one topic")
	}
	for name, t := range p.Topics {
		validateTopic(name, t, bad)
	}
}

// validateTopic checks one focus topic.
func validateTopic(name string, t data.PolicyTopic, bad func(string, ...any)) {
	if name == "" || name != strings.ToLower(name) || strings.ContainsAny(name, " -:") {
		bad("topic name %q must be lower case snake_case", name)
	}
	if len(t.PolicyAreas)+len(t.Subjects)+len(t.TitleTerms) == 0 {
		bad("topic %q has no terms", name)
	}
	for _, list := range [][]string{t.PolicyAreas, t.Subjects, t.TitleTerms} {
		for _, term := range list {
			if strings.TrimSpace(term) == "" {
				bad("topic %q has an empty term", name)
			}
		}
	}
}
