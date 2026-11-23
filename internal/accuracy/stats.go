package accuracy

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/generate"
)

// Collect counts, per task, the stored outputs of the given tasks planned for
// bill: for each planned task of every stored version of the bill it reads the
// stored output (skipping keys with no stored output) and, when present, its
// grade, and returns TaskStats keyed by task name. Findings of an output are
// attributed to checks with findingCheck.
func Collect(ctx context.Context, d generate.Deps, bills []data.BillRef) (map[string]*TaskStats, error) {
	stats := map[string]*TaskStats{}
	for _, bill := range bills {
		tasks, _, err := generate.PlanAll(ctx, d, bill)
		if err != nil {
			return nil, fmt.Errorf("plan %s: %w", bill, err)
		}
		for _, t := range tasks {
			if err := collectTask(ctx, d, t.Key, stats); err != nil {
				return nil, err
			}
		}
	}
	return stats, nil
}

// collectTask adds the stored output and grade of one planned key to stats.
func collectTask(ctx context.Context, d generate.Deps, key data.GenKey, stats map[string]*TaskStats) error {
	out, err := d.Ports.Generated.GetGenerated(ctx, key)
	if errors.Is(err, data.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get output %s: %w", key, err)
	}
	s := statsFor(stats, key.Task)
	s.Outputs++
	s.ByStatus[out.Status]++
	s.Statements += len(out.Statements)
	seen := map[string]bool{}
	for _, f := range out.Findings {
		c := findingCheck(f)
		if !seen[c] {
			seen[c] = true
			s.Failures[c]++
		}
	}
	return addGrade(ctx, d, key, s)
}

// addGrade adds the grade of the output with key to s; a missing grade adds nothing.
func addGrade(ctx context.Context, d generate.Deps, key data.GenKey, s *TaskStats) error {
	g, err := d.Ports.Generated.GetGrade(ctx, key)
	if errors.Is(err, data.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get grade %s: %w", key, err)
	}
	s.Graded += len(g.Statements)
	for _, st := range g.Statements {
		s.Labels[st.Label]++
	}
	if !g.Neutral {
		s.NotNeutral++
	}
	return nil
}

// statsFor returns the entry of task in stats, creating it with non-nil maps.
func statsFor(stats map[string]*TaskStats, task string) *TaskStats {
	s, ok := stats[task]
	if !ok {
		s = &TaskStats{
			Task:     task,
			ByStatus: map[string]int{},
			Failures: map[string]int{},
			Labels:   map[string]int{},
		}
		stats[task] = s
	}
	return s
}

// findingCheck maps a finding of a deterministic check to the check's name;
// findings of no known check map to "other".
func findingCheck(f string) string {
	switch {
	case strings.Contains(f, "fact not in cited lines"):
		return CheckFidelity
	case strings.Contains(f, "amendment consistency"):
		return CheckAmendment
	case strings.Contains(f, "evaluative wording"):
		return CheckNeutral
	case strings.Contains(f, "anchor"), strings.Contains(f, "not in the input"), strings.Contains(f, "no statement"):
		return CheckAnchors
	default:
		return "other"
	}
}

// Rate returns n/total as a fraction, or 0 when total is 0.
func Rate(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total)
}
