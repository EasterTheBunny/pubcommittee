package selection

import (
	"slices"
	"strings"
	"unicode"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// Reason prefixes recorded on a tracked bill.
const (
	ReasonCommitteePrefix = "committee-action:"
	ReasonTopicPrefix     = "topic:"
	ReasonCompanionPrefix = "companion:"
	suffixTitleOnly       = ":title-only"
	suffixSubject         = ":subject"
	suffixPolicyArea      = ":policy-area"
)

// Input is everything the evaluator reads about a bill. It has no sponsor,
// party, or position field by design; TestInputHasNoSponsorFields keeps it so.
type Input struct {
	Bill         data.BillRef
	Title        string
	Status       string
	LatestAction string
	PolicyArea   string
	Subjects     []string
	Committees   []data.CommitteeActivity
	TextVersions []string // official labels of the listed text versions, for example "Engrossed in House"
	Related      []data.RelatedBill
}

// FromCatalog builds the evaluator input from a catalog entry.
func FromCatalog(e data.CatalogEntry) Input {
	in := Input{
		Bill: e.Bill, Title: e.Title, Status: e.Status, PolicyArea: e.PolicyArea,
		Subjects: e.Subjects, Committees: e.Committees, Related: e.Related,
	}
	if e.LatestAction != nil {
		in.LatestAction = e.LatestAction.Text
	}
	for _, l := range e.OfficialLinks {
		if l.Kind == data.LinkTextVersion {
			in.TextVersions = append(in.TextVersions, l.Label)
		}
	}
	return in
}

// Prior is what is already recorded about a tracked bill. A tracked bill is
// never untracked by a rule, so Prior keeps it tracked under its own policy
// version and reasons.
type Prior struct {
	Reasons       []string
	PolicyVersion int
}

// Result is the outcome of one evaluation.
type Result struct {
	Tracked         bool
	Reasons         []string
	PolicyVersion   int
	MarkupStatus    string
	ExclusionReason string
}

// Evaluate applies policy p to one bill. prior is non-nil for a bill that is
// already tracked. isTracked says whether another bill is tracked, for
// companion following; it may be nil.
func Evaluate(p data.SelectionPolicy, in Input, prior *Prior, isTracked func(data.BillRef) bool) Result {
	if prior != nil {
		return Result{
			Tracked: true, Reasons: refreshTitleOnly(prior.Reasons, topicReasons(p, in)),
			PolicyVersion: prior.PolicyVersion, MarkupStatus: data.MarkupInReview,
		}
	}
	return fresh(p, in, isTracked)
}

// fresh evaluates the bill as if it were not tracked.
func fresh(p data.SelectionPolicy, in Input, isTracked func(data.BillRef) bool) Result {
	if reason := ExclusionReason(p, in); reason != "" {
		return Result{MarkupStatus: data.MarkupNotACandidate, ExclusionReason: reason, Reasons: []string{}}
	}
	activity := committeeReasons(p, in)
	topics := topicReasons(p, in)
	companions := companionReasons(p, in, isTracked)
	qualifies := len(companions) > 0
	switch p.Mode {
	case data.PolicyModeGeneral:
		qualifies = qualifies || len(activity) > 0
	default:
		qualifies = qualifies || (len(activity) > 0 && len(topics) > 0)
	}
	if !qualifies {
		return Result{MarkupStatus: data.MarkupEligible, Reasons: []string{}}
	}
	reasons := slices.Concat(activity, topics, companions)
	return Result{Tracked: true, Reasons: reasons, PolicyVersion: p.Version, MarkupStatus: data.MarkupInReview}
}

// refreshTitleOnly keeps the reasons a bill was tracked with. A topic match
// recorded as title-only becomes subject- or policy-area-confirmed when the
// legislature's indexing has since arrived.
func refreshTitleOnly(old, fresh []string) []string {
	out := make([]string, 0, len(old))
	for _, r := range old {
		if topic, ok := strings.CutSuffix(r, suffixTitleOnly); ok {
			for _, suffix := range []string{suffixSubject, suffixPolicyArea} {
				if slices.Contains(fresh, topic+suffix) {
					r = topic + suffix
					break
				}
			}
		}
		if !slices.Contains(out, r) {
			out = append(out, r)
		}
	}
	return out
}

// MeasureKind names the kind of a bill type code, or "" for an unknown type.
func MeasureKind(billType string) string {
	switch billType {
	case "hr", "s":
		return KindBill
	case "hjres", "sjres":
		return KindJointResolution
	case "hres", "sres":
		return KindSimpleResolution
	case "hconres", "sconres":
		return KindConcurrentResolution
	}
	return ""
}

// ExclusionReason returns the human-readable reason a bill can never be
// tracked by the rules, or "" when its kind is allowed.
func ExclusionReason(p data.SelectionPolicy, in Input) string {
	kind := MeasureKind(in.Bill.Type)
	if !slices.Contains(p.MeasureKinds, kind) {
		switch kind {
		case KindSimpleResolution:
			return "simple resolution: a chamber's own statement or procedure rather than drafting of law"
		case KindConcurrentResolution:
			return "concurrent resolution: a statement of both chambers that does not become law, rather than drafting"
		default:
			return "measure type " + in.Bill.Type + " is not a bill or joint resolution"
		}
	}
	words := words(in.Title)
	if slices.Contains(p.ExcludeKinds, ExcludeWarPowers) && isWarPowers(words) {
		return "war powers resolution: a privileged yes-or-no question rather than drafting"
	}
	if slices.Contains(p.ExcludeKinds, ExcludeCommemorative) && isCommemorative(words) {
		return "commemorative measure: a statement of recognition rather than drafting of law"
	}
	return ""
}

func isWarPowers(w []string) bool {
	return containsPhrase(w, []string{"war", "powers"}) ||
		containsPhrase(w, []string{"armed", "forces", "from", "hostilities"})
}

// isCommemorative is true for a title that opens by commemorating, recognizing,
// honoring or celebrating, or that designates a day, week, month or year.
func isCommemorative(w []string) bool {
	if len(w) == 0 {
		return false
	}
	start := w[0]
	if start == "to" && len(w) > 1 {
		start = w[1]
	}
	for _, p := range []string{"commemorat", "recogniz", "honor", "celebrat"} {
		if strings.HasPrefix(start, p) {
			return true
		}
	}
	if strings.HasPrefix(start, "designat") {
		for _, x := range w {
			if slices.Contains([]string{"day", "week", "month", "year", "anniversary", "awareness"}, x) {
				return true
			}
		}
	}
	return false
}

// committeeReasons lists the policy's activity kinds the bill shows, in policy order.
func committeeReasons(p data.SelectionPolicy, in Input) []string {
	found := map[string]bool{}
	for _, c := range in.Committees {
		if k := committeeKind(c.Activity); k != "" {
			found[k] = true
		}
	}
	switch in.Status {
	case "passed-house", "passed-senate", "presented", "enacted":
		found[ActFloor] = true
	}
	for _, label := range in.TextVersions {
		if k := versionKind(label); k != "" {
			found[k] = true
		}
	}
	if la := strings.ToLower(in.LatestAction); strings.Contains(la, "placed on") && strings.Contains(la, "calendar") {
		found[ActCalendar] = true
	}
	var out []string
	for _, k := range p.CommitteeActivity {
		if found[k] {
			out = append(out, ReasonCommitteePrefix+k)
		}
	}
	return out
}

// committeeKind maps a BILLSTATUS committee activity name to an activity kind;
// referral and anything unknown map to "".
func committeeKind(name string) string {
	a := strings.ToLower(name)
	switch {
	case strings.Contains(a, "hearing"):
		return ActHearing
	case strings.Contains(a, "markup"), strings.Contains(a, "mark-up"):
		return ActMarkup
	case strings.HasPrefix(a, "reported"):
		return ActReported
	case strings.HasPrefix(a, "discharged"):
		return ActDischarged
	}
	return ""
}

// versionKind maps the label of an official text version to an activity kind.
func versionKind(label string) string {
	l := strings.ToLower(label)
	switch {
	case strings.HasPrefix(l, "engrossed"), strings.HasPrefix(l, "enrolled"), strings.HasPrefix(l, "public law"):
		return ActFloor
	case strings.HasPrefix(l, "reported"):
		return ActReported
	case strings.HasPrefix(l, "placed on calendar"):
		return ActCalendar
	}
	return ""
}

// topicReasons lists each topic the bill matches. A policy area or subject match
// confirms the topic; a title match alone is recorded as title-only.
func topicReasons(p data.SelectionPolicy, in Input) []string {
	names := make([]string, 0, len(p.Topics))
	for n := range p.Topics {
		names = append(names, n)
	}
	slices.Sort(names)
	var out []string
	titleWords := words(in.Title)
	for _, n := range names {
		t := p.Topics[n]
		base := ReasonTopicPrefix + n
		switch {
		case containsFold(t.PolicyAreas, in.PolicyArea) && in.PolicyArea != "":
			out = append(out, base+suffixPolicyArea)
		case slices.ContainsFunc(in.Subjects, func(s string) bool { return containsFold(t.Subjects, s) }):
			out = append(out, base+suffixSubject)
		case slices.ContainsFunc(t.TitleTerms, func(term string) bool { return containsPhrase(titleWords, words(term)) }):
			out = append(out, base+suffixTitleOnly)
		}
	}
	return out
}

// companionReasons lists the tracked bills the source relates to this one as
// identical or as companions.
func companionReasons(p data.SelectionPolicy, in Input, isTracked func(data.BillRef) bool) []string {
	if !p.FollowCompanions || isTracked == nil {
		return nil
	}
	var out []string
	for _, r := range in.Related {
		rel := strings.ToLower(r.Relationship)
		if (strings.Contains(rel, "identical") || strings.Contains(rel, "companion")) && isTracked(r.Bill) {
			if reason := ReasonCompanionPrefix + r.Bill.ID(); !slices.Contains(out, reason) {
				out = append(out, reason)
			}
		}
	}
	return out
}

func containsFold(list []string, s string) bool {
	return slices.ContainsFunc(list, func(x string) bool {
		return strings.EqualFold(strings.TrimSpace(x), strings.TrimSpace(s))
	})
}

// words lower-cases s and splits it into letter and digit runs.
func words(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// containsPhrase reports whether phrase occurs as consecutive words in text.
func containsPhrase(text, phrase []string) bool {
	if len(phrase) == 0 {
		return false
	}
	for i := 0; i+len(phrase) <= len(text); i++ {
		if slices.Equal(text[i:i+len(phrase)], phrase) {
			return true
		}
	}
	return false
}
