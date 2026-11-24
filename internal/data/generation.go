package data

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Generation tasks ([architecture § AI-generated content]). TaskGrade is the
// grounding grader, a separate model call that is never part of the job.
const (
	TaskBillSummary       = "bill-summary"
	TaskChangeExplanation = "change-explanation"
	TaskBillExplanation   = "bill-explanation"
	TaskVersionChanges    = "version-changes"
	TaskGrade             = "grade"
)

// Statuses of GeneratedOutput.Status. Only GenReady outputs are published.
const (
	GenReady      = "ready"      // passed every deterministic check
	GenUnverified = "unverified" // failed fact fidelity, amendment consistency, anchors, or neutrality
	GenWithheld   = "withheld"   // the grader marked a statement contradicted
	GenRefusal    = "refusal"    // the model declined
	GenTruncated  = "truncated"  // the model ran out of output tokens
	GenInvalid    = "invalid"    // the output did not parse against the schema
)

// ReviewUnreviewed is the review state of every M1 output: no steward has
// looked at it. The UI shows it labelled auto-generated.
const ReviewUnreviewed = "unreviewed"

// Grade labels of StatementGrade.Label.
const (
	GradeSupported    = "supported"
	GradeUnsupported  = "unsupported"
	GradeContradicted = "contradicted"
)

// Roles of Statement.Role in a change explanation.
const (
	RoleNow     = "now"     // what the existing provision says now
	RoleChanges = "changes" // what the bill changes
	RoleWhy     = "why"     // why it matters
)

// GenKey is the idempotency key of one generation: (task, bill, version,
// target, prompt version, model). Target is empty for a bill summary, the
// section id (and provision id) for an explanation, and the previous
// version code for a version change summary.
type GenKey struct {
	Task          string     `json:"task"`
	Version       VersionRef `json:"version"`
	Target        string     `json:"target,omitempty"`
	PromptVersion string     `json:"prompt_version"`
	Model         string     `json:"model"`
}

// String returns the canonical key.
func (k GenKey) String() string {
	return strings.Join([]string{k.Task, k.Version.String(), k.Target, k.PromptVersion, k.Model}, "|")
}

// ID returns the stable file name of the key: 32 hex characters of its hash.
func (k GenKey) ID() string {
	sum := sha256.Sum256([]byte(k.String()))
	return hex.EncodeToString(sum[:16])
}

// Validate reports ErrInvalid for an incomplete key.
func (k GenKey) Validate() error {
	if err := k.Version.Validate(); err != nil {
		return err
	}
	if k.Task == "" || k.PromptVersion == "" || k.Model == "" || strings.ContainsAny(k.Task, "/.") {
		return fmt.Errorf("generation key %q: %w", k, ErrInvalid)
	}
	return nil
}

// dir is the directory of every output of one task on one version:
// generated/{task}/{bill}/v/{version}.
func genDir(task string, v VersionRef) string {
	return "generated/" + task + "/" + v.Bill.String() + "/v/" + v.Version
}

// ObjectPath returns the write-once path of the output.
func (k GenKey) ObjectPath() string { return genDir(k.Task, k.Version) + "/" + k.ID() + ".json" }

// RevisedPath returns the path of the revised record of the output: the replaceable record that stands in for the
// write-once one (see PutRevised).
func (k GenKey) RevisedPath() string {
	return genDir(k.Task, k.Version) + "/" + k.ID() + ".revised.json"
}

// GradePath returns the path of the grade of the output.
func (k GenKey) GradePath() string { return genDir(k.Task, k.Version) + "/" + k.ID() + ".grade.json" }

// currentPath is the pointer to the output a reader shows for a target.
func currentPath(task string, v VersionRef, target string) string {
	sum := sha256.Sum256([]byte(target))
	return genDir(task, v) + "/current-" + hex.EncodeToString(sum[:6]) + ".json"
}

// Statement is one claim with the line anchors and provisions it rests on.
// Anchors are written "<section>-l<n>" or "<section>-l<from>-<to>".
type Statement struct {
	Role         string   `json:"role,omitempty"`
	Text         string   `json:"text"`
	Anchors      []string `json:"anchors"`
	ProvisionIDs []string `json:"provision_ids,omitempty"`
}

// GeneratedOutput is one stored generation. It is platform content,
// provenance auto, review state unreviewed, and immutable once written.
type GeneratedOutput struct {
	Key         GenKey      `json:"key"`
	Status      string      `json:"status"`
	Findings    []string    `json:"findings,omitempty"` // why a check failed
	Headline    string      `json:"headline,omitempty"`
	Statements  []Statement `json:"statements"`
	Section     []string    `json:"section,omitempty"` // explanations: the bill section
	ProvisionID string      `json:"provision_id,omitempty"`
	// CurrentText is the official text of the cited provision (an
	// explanation's "what it says now"); it is copied from the input, not
	// generated.
	CurrentText string `json:"current_text,omitempty"`
	ReviewState string `json:"review_state"`
	TokensIn    int    `json:"tokens_in"`
	TokensOut   int    `json:"tokens_out"`
	// FallbackModel is the model that answered when a fallback model, not
	// the generator named in Key.Model, produced this output; empty otherwise.
	FallbackModel string `json:"fallback_model,omitempty"`
	Meta          Meta   `json:"meta"`
}

// StatementGrade is the grader's verdict on one statement.
type StatementGrade struct {
	Index  int    `json:"index"`
	Label  string `json:"label"`
	Reason string `json:"reason,omitempty"`
	// Conflict is the exact cited text that a contradicted statement conflicts
	// with, quoted by the grader; empty for other labels and in grades stored
	// before the grader had to quote it.
	Conflict string `json:"conflict,omitempty"`
}

// Grade is the grounding grader's verdict on one output, stored beside it.
type Grade struct {
	Key           GenKey           `json:"key"`
	GraderModel   string           `json:"grader_model"`
	PromptVersion string           `json:"prompt_version"`
	Statements    []StatementGrade `json:"statements"`
	Neutral       bool             `json:"neutral"`
	At            time.Time        `json:"at"`
}

// Contradicted reports whether the grader marked any statement contradicted.
func (g Grade) Contradicted() bool {
	return slices.ContainsFunc(g.Statements, func(s StatementGrade) bool { return s.Label == GradeContradicted })
}

// GenContext is what the collect phase needs to check one output without
// rebuilding its input: the official text the statements may cite.
type GenContext struct {
	// Lines maps each citable anchor of the input to its official text.
	Lines map[string]string `json:"lines"`
	// Provisions maps each citable provision id to its text.
	Provisions map[string]string `json:"provisions,omitempty"`
	// Section and ProvisionID name the explanation target.
	Section     []string `json:"section,omitempty"`
	ProvisionID string   `json:"provision_id,omitempty"`
}

// GenBatchItem is one request of a submitted batch.
type GenBatchItem struct {
	Key       GenKey     `json:"key"`
	RequestID string     `json:"request_id"`
	Context   GenContext `json:"context"`
}

// GenBatch is the manifest of one submitted batch. Done becomes true once
// its results are collected.
type GenBatch struct {
	ID          string         `json:"id"`
	Items       []GenBatchItem `json:"items"`
	SubmittedAt time.Time      `json:"submitted_at"`
	Done        bool           `json:"done"`
}

// GeneratedReader reads stored generations. Not found: ErrNotFound.
type GeneratedReader interface {
	// GetGenerated returns the output stored under exactly this key: its revised record when one exists.
	GetGenerated(ctx context.Context, key GenKey) (GeneratedOutput, error)
	// GetCurrent returns the published output for a target of a version.
	GetCurrent(ctx context.Context, task string, v VersionRef, target string) (GeneratedOutput, error)
	// GetGrade returns the grade of the output with this key.
	GetGrade(ctx context.Context, key GenKey) (Grade, error)
}

// GeneratedWriter stores generations, grades, and batch manifests.
type GeneratedWriter interface {
	// PutGenerated stores an output at its write-once path. Identical
	// content is a no-op; different content is ErrConflict.
	PutGenerated(ctx context.Context, o GeneratedOutput) error
	// PutRevised stores a revised record of an output whose write-once record exists, under the same key: a new
	// status, findings and statements, from validation repeated with newer rules or from a retry of a rejected
	// output. Readers of the key (GetGenerated) see the revised record; the write-once record stays as written. A
	// later revision replaces an earlier one. No write-once record is ErrNotFound.
	PutRevised(ctx context.Context, o GeneratedOutput) error
	// SetCurrent makes a GenReady output the one readers show for its
	// target, replacing the earlier pointer. Another status is ErrInvalid.
	SetCurrent(ctx context.Context, o GeneratedOutput) error
	// PutGrade stores or replaces the grade of an output.
	PutGrade(ctx context.Context, g Grade) error
	// SaveBatch stores or replaces a batch manifest; a batch with Done false
	// is listed by PendingBatches until it is saved with Done true.
	SaveBatch(ctx context.Context, b GenBatch) error
	// PendingBatches returns the batches not yet collected, oldest first.
	PendingBatches(ctx context.Context) ([]GenBatch, error)
}

// AnchorID formats the anchor of lines from..to of a section.
func AnchorID(section []string, from, to int) string {
	id := strings.Join(section, ".") + "-l" + strconv.Itoa(from)
	if to > from {
		id += "-" + strconv.Itoa(to)
	}
	return id
}

// ParseAnchorID splits an anchor written by AnchorID. ok is false when s is
// not in that form.
func ParseAnchorID(s string) (section []string, from, to int, ok bool) {
	i := strings.LastIndex(s, "-l")
	if i <= 0 {
		return nil, 0, 0, false
	}
	rest := s[i+2:]
	lo, hi, found := strings.Cut(rest, "-")
	from, err := strconv.Atoi(lo)
	if err != nil || from < 1 {
		return nil, 0, 0, false
	}
	to = from
	if found {
		if to, err = strconv.Atoi(hi); err != nil || to < from {
			return nil, 0, 0, false
		}
	}
	return strings.Split(s[:i], "."), from, to, true
}
