package generate

import (
	"slices"
	"testing"
)

type neutralityCase struct {
	name string
	text string
	want []string
}

func neutralityCases() []neutralityCase {
	return append(descriptiveCases(), evaluativeCases()...)
}

func descriptiveCases() []neutralityCase {
	return []neutralityCase{
		{
			name: "descriptive bill sentence",
			text: "The bill requires the Secretary to report annually.",
			want: nil,
		},
		{
			name: "descriptive amount change",
			text: "Section 3 raises the cap from $5,000 to $7,500.",
			want: nil,
		},
		{
			name: "modal verbs used by bills are allowed",
			text: "The Secretary shall report, the agency must publish, and it may appeal; the rule is required and important.",
			want: nil,
		},
	}
}

func evaluativeCases() []neutralityCase {
	return []neutralityCase{
		{
			name: "evaluative sentence with two words",
			text: "Unfortunately the bill is wasteful.",
			want: []string{"unfortunately", "wasteful"},
		},
		{
			name: "evaluative sentence with hyphenated and modal words",
			text: "This is a much-needed and sensible reform; Congress should pass it.",
			want: []string{"much-needed", "sensible", "should"},
		},
		{
			name: "hyphenated word does not also report its suffix",
			text: "A long-overdue fix.",
			want: []string{"long-overdue"},
		},
		{
			name: "plain overdue is reported",
			text: "The report is overdue.",
			want: []string{"overdue"},
		},
		{
			name: "case-insensitive match",
			text: "SHOULD Congress Act. That is Good policy.",
			want: []string{"should", "good"},
		},
		{
			name: "substrings do not match",
			text: "The badge and goodwill were Bestowed on the bearer.",
			want: nil,
		},
		{
			name: "repeated word reported once in first-appearance order",
			text: "Good, better, good again; better still, GOOD.",
			want: []string{"good", "better"},
		},
		{
			name: "order follows first appearance",
			text: "Worse than before, and better in some ways.",
			want: []string{"worse", "better"},
		},
		{
			name: "empty text",
			text: "",
			want: nil,
		},
	}
}

func TestNeutrality(t *testing.T) {
	t.Parallel()

	for _, tc := range neutralityCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Neutrality(tc.text)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("Neutrality(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}
