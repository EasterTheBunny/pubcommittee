package selection

import (
	"slices"
	"strings"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// TopicLabel is the plain name of a topic key: "deficit_reduction" becomes "deficit reduction".
func TopicLabel(key string) string { return strings.ReplaceAll(key, "_", " ") }

// Scope is the plain-language scope statement for the UI, derived from the
// policy, for example "Bills and joint resolutions with committee action on
// affordability, deficit reduction, and healthcare".
func Scope(p data.SelectionPolicy) string {
	kinds := make([]string, 0, len(p.MeasureKinds))
	for _, k := range p.MeasureKinds {
		kinds = append(kinds, strings.ReplaceAll(k, "_", " ")+"s")
	}
	s := "Bills with committee action"
	if len(kinds) > 0 {
		s = capital(joinAnd(kinds)) + " with committee action"
	}
	if p.Mode == data.PolicyModePilot {
		s += " on " + joinAnd(topicLabels(p))
	}
	return s + "."
}

// Rules is the plain-language statement of a policy, one rule per line.
func Rules(p data.SelectionPolicy) []string {
	out := []string{Scope(p)}
	out = append(out, "Committee action means "+joinAnd(activityWords(p))+
		"; referral to a committee alone does not count.")
	if p.Mode == data.PolicyModePilot {
		out = append(out, "During the pilot a bill must also match a focus topic by the legislature's policy area, "+
			"legislative subjects, or title words; a match on title words alone is recorded as such.")
	}
	if p.FollowCompanions {
		out = append(out, "A bill identical to, or the other chamber's companion of, a tracked bill is tracked too.")
	}
	if len(p.ExcludeKinds) > 0 || len(p.MeasureKinds) > 0 {
		out = append(out, "Other resolutions, war powers resolutions, and commemorative measures are not markup candidates.")
	}
	if p.Phase == data.PolicyPhaseEarlyPilot {
		out = append(out,
			"Administrators may select a bill directly while the pilot is early; each selection is logged publicly.")
	} else {
		out = append(out, "A bill selected directly needs sign-off by stewards from two parties.")
	}
	return append(out, "Once a bill is tracked it stays tracked; a change of rules never removes it.")
}

func topicLabels(p data.SelectionPolicy) []string {
	keys := make([]string, 0, len(p.Topics))
	for k := range p.Topics {
		keys = append(keys, TopicLabel(k))
	}
	slices.Sort(keys)
	return keys
}

func activityWords(p data.SelectionPolicy) []string {
	names := map[string]string{
		ActHearing: "a hearing", ActMarkup: "a markup", ActReported: "a committee report", ActDischarged: "a discharge",
		ActFloor: "floor action", ActCalendar: "placement on a calendar",
	}
	out := make([]string, 0, len(p.CommitteeActivity))
	for _, a := range p.CommitteeActivity {
		out = append(out, names[a])
	}
	return out
}

func joinAnd(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2: //nolint:mnd // two items read "a and b"
		return items[0] + " and " + items[1]
	}
	return strings.Join(items[:len(items)-1], ", ") + ", and " + items[len(items)-1]
}

func capital(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
