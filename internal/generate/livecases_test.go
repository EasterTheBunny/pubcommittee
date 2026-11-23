package generate

import (
	"slices"
	"strings"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// Cases from the live L3 run of 2026-11-22: outputs the validators withheld although the model was right (false
// positives), and outputs they rightly withheld that the fixes must keep withholding. Text is the minimal part of
// the stored output and of the official bill text.

func TestCitationFormResolves(t *testing.T) {
	t.Parallel()
	gc := data.GenContext{Provisions: map[string]string{
		"usc/42/1395cc-7":      "text of 1395cc-7",
		"usc/16/824a-4/b":      "text",
		"usc/42/1395iii/b/1":   "text",
		"usc/16/824a/e":        "text",
		"usc/16/796":           "text",
		"usc/15/717b/c":        "text",
		"usc/16/824a/f":        "text",
		"usc/42/1395cc-7/a/9a": "unrelated deeper provision",
	}}
	found := []string{
		"42 U.S.C. 1395cc–7",       // en dash in the printed form
		"42 U.S.C. 1395cc–7(a)(1)", // subsection of a section stored whole
		"16 U.S.C. 824a–4(b)",
		"42 U.S.C. 1395iii(b)(1)",
		"16 U.S.C. 824a(e)",
		"15 U.S.C. 717b(c)",
		"usc/16/824a-4/b", // the stored form still resolves
	}
	for _, id := range found {
		if _, ok := lookupProvision(gc, id); !ok {
			t.Errorf("lookupProvision(%q) not found, want found", id)
		}
	}
	// A citation must still name a provision of the input: another section, another title, a broader citation.
	missing := []string{"42 U.S.C. 1395cc–8", "43 U.S.C. 1395cc–7", "16 U.S.C. 824a", "16 U.S.C. 824b(e)", "usc/42/9999"}
	for _, id := range missing {
		if _, ok := lookupProvision(gc, id); ok {
			t.Errorf("lookupProvision(%q) found, want not found", id)
		}
	}
}

func TestSummaryProvisionCitationHasNoFinding(t *testing.T) {
	t.Parallel()
	gc := data.GenContext{
		Lines: map[string]string{"s2-l2": "Section 1866G(a)(1) of the Social Security Act (42 U.S.C. 1395cc–7(a)(1)) is amended."},
		// A summary lists the provision ids of the citation set with no text.
		Provisions: map[string]string{"usc/42/1395cc-7": ""},
	}
	out := ModelOutput{Headline: "", Statements: []data.Statement{{
		Text: "It amends the law.", Anchors: []string{"s2-l2"}, ProvisionIDs: []string{"42 U.S.C. 1395cc–7(a)(1)"},
	}}}
	if got := Validate(data.TaskBillSummary, out, gc); got != nil {
		t.Errorf("Validate = %q, want no findings", got)
	}
	out.Statements[0].ProvisionIDs = []string{"42 U.S.C. 1395cc–99"}
	got := Validate(data.TaskBillSummary, out, gc)
	if len(got) != 1 || !strings.Contains(got[0], "is not in the input") {
		t.Errorf("Validate with an unknown provision = %q, want one finding", got)
	}
}

func TestCitationNumbersInTextAreNotFacts(t *testing.T) {
	t.Parallel()
	gc := data.GenContext{
		Lines:      map[string]string{"s2-l3": "no person may construct a border-crossing facility without a certificate of crossing."},
		Provisions: map[string]string{"usc/16/796": "definitions"},
	}
	s := data.Statement{
		Role: data.RoleChanges, Anchors: []string{"s2-l3"},
		Text: "The 16 U.S.C. 796 definitions decide which organizations' policies apply.",
	}
	if got := Validate(data.TaskBillExplanation, ModelOutput{Statements: []data.Statement{s}}, gc); slices.ContainsFunc(
		got, func(f string) bool { return strings.Contains(f, "fact not in cited lines") },
	) {
		t.Errorf("Validate = %q, want no fact finding for a citation of a provision of the input", got)
	}
	// A citation of a provision that is not in the input is still read as numbers.
	s.Text = "The 16 U.S.C. 797 definitions decide which organizations' policies apply."
	got := Validate(data.TaskBillExplanation, ModelOutput{Statements: []data.Statement{s}}, gc)
	if !slices.ContainsFunc(got, func(f string) bool { return strings.Contains(f, "number 797") }) {
		t.Errorf("Validate = %q, want a finding for number 797", got)
	}
}

// The derived and reused numbers of the live run stay rejected: a difference of two figures, a count the lines
// never state, a section number that appears only in lines the statement does not cite.
func TestDerivedNumbersStayRejected(t *testing.T) {
	t.Parallel()
	line := "Section 1898(b)(1) is amended by striking “$1,403,000,000” and inserting “$1,400,500,000”."
	for _, tc := range []struct{ text, source, want string }{
		{"The inserted figure is $2,500,000 lower than the figure the bill strikes.", line, "number 2500000"},
		{"the Secretary of Energy must require the facility to follow the standards of three bodies", "the Electric Reliability Organization and the applicable regional entity", "word-number three"},
		{"Section 3 now amends section 1866G of the Act.", "(1) in subsection (a)(3)(E)—", "number 1866"},
	} {
		if got := FactFidelity(tc.text, tc.source); !slices.Contains(got, tc.want) {
			t.Errorf("FactFidelity(%q) = %q, want it to contain %q", tc.text, got, tc.want)
		}
	}
}

const (
	liveStrikeHearings = `Section 602(b) of the Public Utility Regulatory Policies Act of 1978 (16 U.S.C. 824a–4(b)) is amended by striking “the Commission has conducted hearings and made the findings required under section 202(e) of the Federal Power Act” and all that follows through the period at the end and inserting “the Secretary has conducted hearings and finds that the proposed transmission facilities would not impair the sufficiency of electric supply”.`
	liveStrikeState    = `Section 202(f) of the Federal Power Act (16 U.S.C. 824a(f)) is amended by striking “insofar as such State regulation does not conflict with the exercise of the Commission’s powers under or relating to subsection 202(e)”.`
	liveStudy          = `in clause (ii), by striking “the study described in subsection (b)” and inserting “the studies described in subsections (b) and (c)”; and`
)

func TestParaphrasedAmendmentTextIsNamed(t *testing.T) {
	t.Parallel()
	runAmendmentCases(t, []amendmentCase{
		{
			name: "removed text described in other words, inserted text quoted in part",
			explanation: "The bill strikes the reference to the Commission's hearings and section 202(e) findings and the rest of that " +
				"sentence. It instead requires that the Secretary has held hearings and finds that the proposed transmission " +
				"facilities would not impair the sufficiency of electric supply. It repeals section 202(e) of the Federal Power Act.",
			cited: liveStrikeHearings,
		},
		{
			name: "removed text paraphrased",
			explanation: "The bill removes related wording from section 202(f) that tied State regulation to the Commission's powers " +
				"under section 202(e).",
			cited: liveStrikeState,
		},
		{
			name: "both texts of a study amendment paraphrased",
			explanation: "It extends the data-collection reference in subsection (a)(3)(E) to cover both the studies in subsections " +
				"(b) and (c).",
			cited:        liveStudy,
			provisionIDs: []string{"usc/42/1395cc-7"},
		},
	})
}

func TestVagueAmendmentExplanationStaysRejected(t *testing.T) {
	t.Parallel()
	runAmendmentCases(t, []amendmentCase{{
		name: "a repeal sentence that shares only the section and the act with the removed text",
		explanation: "It also repeals section 202(e) of the Federal Power Act and makes related conforming changes. " +
			"The Federal Energy Regulatory Commission handles pipelines.",
		cited: liveStrikeHearings,
		want: []string{
			`removed text "the Commission has conducted hearings and made the findings required under section 202(e) of the Federal Power Act" not named`,
		},
	}})
}
