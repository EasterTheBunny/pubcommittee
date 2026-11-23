package generate

import (
	"slices"
	"testing"
)

func TestFactFidelitySupported(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		text   string
		source string
	}{
		{name: "identical number", text: "covers 5 bills", source: "section 5 covers 5 bills"},
		{name: "scale word against comma number", text: "$5 million", source: "$5,000,000"},
		{name: "scale word against scale word", text: "5 million", source: "5 million"},
		{name: "thousands comma in text", text: "5,000 people", source: "5000 people"},
		{name: "spelled number against digits", text: "ten days", source: "10 days"},
		{name: "hundred against digits", text: "hundred", source: "100"},
		{name: "spelled number against spelled word", text: "twelve members", source: "twelve members"},
		{name: "month present", text: "Signed January 5, 2026", source: "Signed January 5, 2026."},
		{name: "percent against percent", text: "12% of members", source: "12 percent of members"},
		{name: "percent sign against percent word", text: "12 percent", source: "12%"},
		{name: "one is not checked", text: "one bill", source: ""},
		{name: "empty text", text: "", source: "5 bills"},
		{name: "empty text and source", text: "", source: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := FactFidelity(tc.text, tc.source); got != nil {
				t.Errorf("FactFidelity(%q, %q) = %q, want nil", tc.text, tc.source, got)
			}
		})
	}
}

func TestFactFidelityUnsupported(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		text   string
		source string
		want   []string
	}{
		{name: "changed number", text: "5 bills", source: "6 bills", want: []string{"number 5"}},
		{name: "changed scale value", text: "$6 million", source: "$5,000,000", want: []string{"number 6000000"}},
		{name: "month absent", text: "Signed in March 2026", source: "Signed in April 2026", want: []string{"month march"}},
		{
			name:   "spelled number absent",
			text:   "twelve members",
			source: "eleven members",
			want:   []string{"word-number twelve"},
		},
		{
			name:   "swapped section reference",
			text:   "see section 5",
			source: "see s3 of the act",
			want:   []string{"number 5"},
		},
		{name: "changed percent", text: "12%", source: "10%", want: []string{"number 12"}},

		// Repeats and order.
		{
			name:   "repeated fact reported once",
			text:   "5 bills and 5 bills and 6 bills",
			source: "5 bills",
			want:   []string{"number 6"},
		},
		{
			name:   "repeated unsupported fact reported once",
			text:   "7 bills, 7 bills",
			source: "",
			want:   []string{"number 7"},
		},
		{
			name:   "order of first appearance",
			text:   "March 3 then nine",
			source: "",
			want:   []string{"month march", "number 3", "word-number nine"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := FactFidelity(tc.text, tc.source)
			if !slices.Equal(got, tc.want) {
				t.Errorf("FactFidelity(%q, %q) = %q, want %q", tc.text, tc.source, got, tc.want)
			}
		})
	}
}
