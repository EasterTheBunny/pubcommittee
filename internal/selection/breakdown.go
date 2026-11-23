package selection

import (
	"slices"
	"strings"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// BreakdownPath is the published file the tracked-set breakdown is written to.
const BreakdownPath = "exports/tracked-set-breakdown.json"

// ReasonManual is the tracked reason of a bill an operator pulled with --track.
const ReasonManual = "manual-pull"

// Breakdown is the tracked set's disclosure for
// [09 § Coverage neutrality]: counts by topic, by sponsor party, and
// administrator-selected. Party is read here, after selection, and nowhere in
// the evaluator.
type Breakdown struct {
	AsOf             time.Time      `json:"as_of"`
	PolicyVersion    int            `json:"policy_version"`
	CatalogEntries   int            `json:"catalog_entries"`
	Tracked          int            `json:"tracked"`
	ByTopic          map[string]int `json:"by_topic"`
	ByTopicTitleOnly map[string]int `json:"by_topic_title_only"`
	ByCompanion      int            `json:"companion"`
	ByAdministrator  int            `json:"administrator_selected"`
	BySponsorParty   map[string]int `json:"by_sponsor_party"`
	ByMarkupStatus   map[string]int `json:"by_markup_status"`
}

// ReasonAdministrator marks a bill an administrator selected directly (M1-12).
const ReasonAdministrator = "administrator"

// Compute counts the tracked set of entries. Party "" is counted as "unknown".
func Compute(entries []data.CatalogEntry, policyVersion int, asOf time.Time) Breakdown {
	b := Breakdown{
		AsOf: asOf.UTC(), PolicyVersion: policyVersion, CatalogEntries: len(entries),
		ByTopic: map[string]int{}, ByTopicTitleOnly: map[string]int{},
		BySponsorParty: map[string]int{}, ByMarkupStatus: map[string]int{},
	}
	for _, e := range entries {
		b.ByMarkupStatus[e.MarkupStatus]++
		if e.Tracked {
			b.addTracked(e)
		}
	}
	return b
}

func hasPrefix(list []string, prefix string) bool {
	return slices.ContainsFunc(list, func(s string) bool { return strings.HasPrefix(s, prefix) })
}

func contains(list []string, s string) bool { return slices.Contains(list, s) }

// countTopics counts each topic of a bill's reasons once.
func (b *Breakdown) countTopics(reasons []string) {
	counted := map[string]bool{}
	for _, r := range reasons {
		rest, ok := strings.CutPrefix(r, ReasonTopicPrefix)
		if !ok {
			continue
		}
		topic, basis, _ := strings.Cut(rest, ":")
		if counted[topic] {
			continue
		}
		counted[topic] = true
		b.ByTopic[topic]++
		if basis == "title-only" {
			b.ByTopicTitleOnly[topic]++
		}
	}
}

// addTracked counts one tracked entry.
func (b *Breakdown) addTracked(e data.CatalogEntry) {
	b.Tracked++
	party := e.SponsorParty
	if party == "" {
		party = "unknown"
	}
	b.BySponsorParty[party]++
	b.countTopics(e.Reasons)
	if hasPrefix(e.Reasons, ReasonCompanionPrefix) {
		b.ByCompanion++
	}
	if contains(e.Reasons, ReasonManual) || contains(e.Reasons, ReasonAdministrator) {
		b.ByAdministrator++
	}
}
