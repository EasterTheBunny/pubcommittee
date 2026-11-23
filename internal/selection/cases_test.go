package selection_test

import (
	"slices"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/selection"
)

// evalCase is one row of TestEvaluate.
type evalCase struct {
	name       string
	policy     func(*testing.T) data.SelectionPolicy
	in         selection.Input
	tracked    []data.BillRef
	wantStatus string
	wantReason []string
	wantExcl   string
}

// evalCases are the evaluator's rules, one case each.
func evalCases(t *testing.T) []evalCase {
	t.Helper()
	return slices.Concat(trackingCases(t), stageCases(), exclusionCases(), companionCases())
}

// trackingCases cover committee action, topic and the modes.
func trackingCases(t *testing.T) []evalCase {
	t.Helper()
	general := v1(t)
	general.Mode = data.PolicyModeGeneral
	return []evalCase{
		{
			name: "referral only never counts", in: selection.Input{
				Bill: ref("hr", 1), Title: "Health care costs act", PolicyArea: "Health",
				Committees: act("Referred To"),
			},
			wantStatus: data.MarkupEligible,
		},
		{
			name: "markup and subject", in: selection.Input{
				Bill: ref("hr", 2), Title: "A bill", Subjects: []string{"medicare"}, Committees: act("Markup By"),
			},
			wantStatus: data.MarkupInReview,
			wantReason: []string{"committee-action:markup", "topic:healthcare:subject"},
		},
		{
			name: "hearing and policy area", in: selection.Input{
				Bill: ref("s", 3), Title: "A bill", PolicyArea: "Health", Committees: act("Hearings By (full committee)"),
			},
			wantStatus: data.MarkupInReview,
			wantReason: []string{"committee-action:hearing", "topic:healthcare:policy-area"},
		},
		{
			name: "title match before indexing is recorded as title-only", in: selection.Input{
				Bill: ref("hr", 4), Title: "To reduce the national debt", Committees: act("Reported By"),
			},
			wantStatus: data.MarkupInReview,
			wantReason: []string{"committee-action:reported", "topic:deficit_reduction:title-only"},
		},
		{
			name: "action without a topic is eligible in pilot", in: selection.Input{
				Bill: ref("hr", 5), Title: "A highways bill", PolicyArea: "Transportation and Public Works", Committees: act("Markup By"),
			},
			wantStatus: data.MarkupEligible,
		},
		{
			name: "general mode ignores topics", policy: func(*testing.T) data.SelectionPolicy { return general },
			in:         selection.Input{Bill: ref("hr", 6), Title: "A highways bill", Committees: act("Discharged From")},
			wantStatus: data.MarkupInReview,
			wantReason: []string{"committee-action:discharged"},
		},
	}
}

// stageCases cover floor action and calendar placement.
func stageCases() []evalCase {
	return []evalCase{
		{
			name: "floor action by status", in: selection.Input{
				Bill: ref("hr", 7), Title: "Lower costs for families", Status: "passed-house",
			},
			wantStatus: data.MarkupInReview,
			wantReason: []string{"committee-action:floor", "topic:affordability:title-only"},
		},
		{
			name: "calendar by text version and latest action", in: selection.Input{
				Bill: ref("s", 8), Title: "Medicaid access", TextVersions: []string{"Introduced in Senate", "Placed on Calendar Senate"},
			},
			wantStatus: data.MarkupInReview,
			wantReason: []string{"committee-action:calendar", "topic:healthcare:title-only"},
		},
		{
			name: "calendar by latest action", in: selection.Input{
				Bill: ref("s", 9), Title: "Medicaid access", LatestAction: "Placed on Senate Legislative Calendar under General Orders.",
			},
			wantStatus: data.MarkupInReview,
			wantReason: []string{"committee-action:calendar", "topic:healthcare:title-only"},
		},
	}
}

// exclusionCases cover kinds the policy never tracks.
func exclusionCases() []evalCase {
	return []evalCase{
		{
			name: "simple resolution is not a candidate", in: selection.Input{
				Bill: ref("hres", 10), Title: "Health", Committees: act("Markup By"),
			},
			wantStatus: data.MarkupNotACandidate,
			wantExcl:   "simple resolution: a chamber's own statement or procedure rather than drafting of law",
		},
		{
			name: "war powers joint resolution is excluded", in: selection.Input{
				Bill: ref("hjres", 11), Title: "Directing the removal of United States Armed Forces from hostilities against Iran pursuant to the War Powers Resolution",
				Committees: act("Reported By"),
			},
			wantStatus: data.MarkupNotACandidate,
			wantExcl:   "war powers resolution: a privileged yes-or-no question rather than drafting",
		},
		{
			name: "commemorative joint resolution is excluded", in: selection.Input{
				Bill: ref("sjres", 12), Title: "Designating the week of June 1 as National Health Week", Committees: act("Reported By"),
			},
			wantStatus: data.MarkupNotACandidate,
			wantExcl:   "commemorative measure: a statement of recognition rather than drafting of law",
		},
	}
}

// companionCases cover companion following and word matching.
func companionCases() []evalCase {
	return []evalCase{
		{
			name: "companion of a tracked bill needs no action or topic", in: selection.Input{
				Bill: ref("s", 13), Title: "A bill", Committees: act("Referred To"),
				Related: []data.RelatedBill{{Bill: ref("hr", 2), Relationship: "Identical bill"}},
			},
			tracked:    []data.BillRef{ref("hr", 2)},
			wantStatus: data.MarkupInReview,
			wantReason: []string{"companion:us-119-hr-2"},
		},
		{
			name: "identical to an untracked bill is not followed", in: selection.Input{
				Bill: ref("s", 14), Title: "A bill",
				Related: []data.RelatedBill{{Bill: ref("hr", 2), Relationship: "Identical bill"}},
			},
			wantStatus: data.MarkupEligible,
		},
		{
			name: "related but not identical is not followed", in: selection.Input{
				Bill: ref("s", 15), Title: "A bill",
				Related: []data.RelatedBill{{Bill: ref("hr", 2), Relationship: "Related bill"}},
			},
			tracked:    []data.BillRef{ref("hr", 2)},
			wantStatus: data.MarkupEligible,
		},
		{
			name: "title words match whole words only", in: selection.Input{
				Bill: ref("hr", 16), Title: "Wealthy donors act", Committees: act("Markup By"),
			},
			wantStatus: data.MarkupEligible,
		},
	}
}
