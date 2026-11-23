package generate

import (
	"slices"
	"testing"
)

// amendmentCase is one row of an AmendmentConsistency table.
type amendmentCase struct {
	name         string
	explanation  string
	cited        string
	provisionIDs []string
	want         []string
}

const (
	cited1395 = `Section 1395 of the Social Security Act is amended by striking "$5,000" and inserting "$7,500".`
	cited12   = "Section 12 of the Act is repealed."
	citedAdd  = `The Act is amended by adding at the end the following: "New text."`
	citedStrk = `The Act is amended by striking "old text".`
)

// runAmendmentCases runs every case as a parallel subtest.
func runAmendmentCases(t *testing.T, cases []amendmentCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := AmendmentConsistency(tc.explanation, tc.cited, tc.provisionIDs)
			if !slices.Equal(got, tc.want) {
				t.Errorf("AmendmentConsistency() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAmendmentStrike(t *testing.T) {
	t.Parallel()
	runAmendmentCases(t, []amendmentCase{
		{
			name:        "strike and insert named in order",
			explanation: "Replaces $5,000 with $7,500 in section 1395.",
			cited:       cited1395,
		},
		{
			name:        "inserted text named before removed text",
			explanation: "Replaces $7,500 with $5,000 in section 1395.",
			cited:       cited1395,
			want:        []string{"inserted text named before removed text"},
		},
		{
			name:        "inserted text not named",
			explanation: "Removes $5,000 from section 1395.",
			cited:       cited1395,
			want:        []string{`inserted text "$7,500" not named`},
		},
		{
			name:        "removed text not named",
			explanation: "Replaces $6 with $7,500 in section 1395.",
			cited:       cited1395,
			want:        []string{`removed text "$5,000" not named`},
		},
		{
			name:        "empty explanation names nothing",
			explanation: "",
			cited:       cited1395,
			want: []string{
				`removed text "$5,000" not named`,
				`inserted text "$7,500" not named`,
				"target section 1395 not named",
			},
		},
		{
			name:        "strike only named",
			explanation: "Removes old text.",
			cited:       citedStrk,
		},
		{
			name:        "strike only removed text not named",
			explanation: "Adds new text.",
			cited:       citedStrk,
			want:        []string{`removed text "old text" not named`},
		},
	})
}

func TestAmendmentTarget(t *testing.T) {
	t.Parallel()
	runAmendmentCases(t, []amendmentCase{
		{
			name:        "target section not named",
			explanation: "Replaces $5,000 with $7,500 in section 7.",
			cited:       cited1395,
			want:        []string{"target section 1395 not named"},
		},
		{
			name:         "target named by provision id",
			explanation:  "Replaces $5,000 with $7,500 in section 7.",
			cited:        cited1395,
			provisionIDs: []string{"usc/42/1395"},
		},
		{
			name:        "curly quotes",
			explanation: "Replaces $5,000 with $7,500 in section 1395.",
			cited:       "Section 1395 of the Social Security Act is amended by striking “$5,000” and inserting “$7,500”.",
		},
		{
			name:        "doubled backtick and apostrophe quotes",
			explanation: "Replaces $5,000 with $7,500 in section 1395.",
			cited:       "Section 1395 of the Social Security Act is amended by striking ``$5,000'' and inserting ``$7,500''.",
		},
	})
}

func TestAmendmentRepealAdd(t *testing.T) {
	t.Parallel()
	runAmendmentCases(t, []amendmentCase{
		{
			name:        "repeal described as repeal",
			explanation: "The bill repeals section 12.",
			cited:       cited12,
		},
		{
			name:        "repeal described as addition",
			explanation: "The bill adds new requirements to section 12.",
			cited:       cited12,
			want: []string{
				"repeal not described",
				"explanation describes an addition but the instruction repeals",
			},
		},
		{
			name:        "repeal not described",
			explanation: "The bill changes section 12.",
			cited:       cited12,
			want:        []string{"repeal not described"},
		},
		{
			name:        "add described as removal",
			explanation: "The bill repeals the Act.",
			cited:       citedAdd,
			want:        []string{"explanation describes removal but the instruction adds text"},
		},
		{
			name:        "add described as addition",
			explanation: "Adds new text at the end of the Act.",
			cited:       citedAdd,
		},
		{
			name:        "no amendment instruction",
			explanation: "The bill repeals section 12.",
			cited:       "The Secretary shall report annually.",
		},
	})
}
