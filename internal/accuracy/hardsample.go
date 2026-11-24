package accuracy

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// Strata of the hard sample.
const (
	StratumContradicted = "grader-contradicted"
	StratumRejected     = "validator-rejected"
	StratumControl      = "grader-supported-control"
)

// minRejectedWords is the fewest words a rejected statement needs to be worth
// grading: empty and "placeholder" statements carry nothing to grade.
const minRejectedWords = 4

// HardMix is how many statements the hard sample takes from each stratum.
// A stratum with fewer candidates gives all it has.
type HardMix struct {
	Contradicted int
	Rejected     int
	Control      int
}

// BuildHardSample picks the statements the first sample could not hold:
// those the grader marked contradicted, those of outputs the validators did
// not pass (unverified or withheld, with their findings), and supported
// controls. Statements whose id is in exclude (the first sample) are skipped.
// The choice is deterministic: strata are drawn in key order, evenly spread,
// and the result is ordered by a hash of the id so that the order does not
// reveal the stratum.
func BuildHardSample(entries []SampleEntry, mix HardMix, exclude map[string]bool) []SampleItem {
	sorted := append([]SampleEntry(nil), entries...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Output.Key.String() < sorted[j].Output.Key.String()
	})
	var contra, rejected, control []SampleItem
	for _, e := range sorted {
		for i, st := range e.Output.Statements {
			it := sampleItem(e, i)
			if exclude[it.ID] {
				continue
			}
			it.ValidatorStatus = e.Output.Status
			switch {
			case e.Output.Status != data.GenReady:
				if len(strings.Fields(st.Text)) < minRejectedWords || len(it.Cited) == 0 {
					continue
				}
				it.Stratum = StratumRejected
				it.ValidatorFindings = e.Output.Findings
				rejected = append(rejected, it)
			case it.GraderLabel == data.GradeContradicted:
				it.Stratum = StratumContradicted
				contra = append(contra, it)
			case it.GraderLabel == data.GradeSupported:
				it.Stratum = StratumControl
				control = append(control, it)
			}
		}
	}
	out := make([]SampleItem, 0, mix.Contradicted+mix.Rejected+mix.Control)
	out = append(out, spread(contra, mix.Contradicted)...)
	out = append(out, spread(rejected, mix.Rejected)...)
	out = append(out, spread(control, mix.Control)...)
	sort.SliceStable(out, func(i, j int) bool { return idHash(out[i].ID) < idHash(out[j].ID) })
	return out
}

// spread takes n items evenly spaced over the list (all when n >= len).
func spread(items []SampleItem, n int) []SampleItem {
	if n <= 0 || len(items) == 0 {
		return nil
	}
	if n >= len(items) {
		return items
	}
	out := make([]SampleItem, 0, n)
	for i := range n {
		out = append(out, items[i*len(items)/n])
	}
	return out
}

func idHash(id string) string {
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:])
}
