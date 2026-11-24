// Package accuracy measures the accuracy checks of generated content
// (architecture § Accuracy checks) and renders the report.
package accuracy

import "github.com/easterthebunny/pubcommittee/internal/data"

// Checks named in findings and in the report.
const (
	CheckFidelity  = "fact-fidelity"
	CheckAmendment = "amendment-consistency"
	CheckNeutral   = "neutral-wording"
	CheckAnchors   = "anchors"
)

// TaskStats counts the stored outputs of one task.
type TaskStats struct {
	Task       string
	Outputs    int            // stored outputs, any status
	Fallback   int            // outputs answered by a fallback model, not the generator
	ByStatus   map[string]int // by data.Gen* status
	Statements int            // statements in all outputs
	// Failures counts outputs with at least one finding of each check.
	Failures map[string]int
	// Graded counts graded statements; Labels counts them by data.Grade* label.
	Graded int
	Labels map[string]int
	// NotNeutral counts graded outputs the grader rated not neutral.
	NotNeutral int
}

// PlantRow is the result of one planted-error kind.
type PlantRow struct {
	Kind          string
	Check         string // deterministic check expected to catch it; "" for grader only
	Planted       int
	CaughtByCheck int // caught by the deterministic check
	CaughtByGrade int // caught by the grader (statement graded unsupported or contradicted); -1 when not run
}

// SampleItem is one statement of the operator sample with its cited lines.
// OperatorLabel is filled in by the operator: supported, unsupported or
// contradicted.
type SampleItem struct {
	ID            string   `json:"id"`  // output key id plus statement index
	Key           string   `json:"key"` // idempotency key string
	Task          string   `json:"task"`
	Statement     string   `json:"statement"`
	Cited         []string `json:"cited"` // "anchor: text" rows, and provision text
	OperatorLabel string   `json:"operator_label"`
	GraderLabel   string   `json:"grader_label,omitempty"`
	// Stratum, ValidatorStatus and ValidatorFindings are set only in the hard
	// sample (BuildHardSample): why the statement was picked, the status of its
	// output, and the deterministic validators' findings on the whole output.
	Stratum           string   `json:"stratum,omitempty"`
	ValidatorStatus   string   `json:"validator_status,omitempty"`
	ValidatorFindings []string `json:"validator_findings,omitempty"`
}

// CRSRow is the coverage of one bill summary against the CRS summary.
type CRSRow struct {
	Version     data.VersionRef
	Points      int
	Covered     int
	Contradicts int
}
