package adminapi

import (
	"encoding/json"

	"github.com/easterthebunny/pubcommittee/internal/ingest"
)

// Routes of admin-api. Operators call all but RosterRoute; the deploy service
// account calls RosterRoute only.
const (
	PathPrefix      = "/admin/v1"
	RosterRoute     = PathPrefix + "/operators/roster"
	PolicyRoute     = PathPrefix + "/selection-policy"
	GenerationRoute = PathPrefix + "/generations"
	FailuresRoute   = PathPrefix + "/ingest/failures"
	JobsRoute       = PathPrefix + "/jobs"
)

// Reasoned is embedded in every mutating request body.
type Reasoned struct {
	// Reason is public; required.
	Reason string `json:"reason"`
}

// ActionResult answers a mutating bill action.
type ActionResult struct {
	Bill string `json:"bill"`
	// Changed is false when the bill already was in the requested state;
	// nothing was written and nothing was logged.
	Changed bool   `json:"changed"`
	LogID   string `json:"log_id,omitempty"`
	// Note names what the caller still has to do or know.
	Note string `json:"note,omitempty"`
	// Pull is the text pull that follows a track or a reingest.
	Pull *ingest.Report `json:"pull,omitempty"`
	// PullError is set when the pull stopped; the change itself stays applied.
	PullError string `json:"pull_error,omitempty"`
}

// CatalogPatch is the body of PATCH /bills/{bill}/catalog: the fields that
// ingestion can get wrong. Nil fields are left alone. Tracking, reasons and
// sponsor party cannot be patched.
type CatalogPatch struct {
	Reasoned

	Title      *string   `json:"title,omitempty"`
	Status     *string   `json:"status,omitempty"`
	Chamber    *string   `json:"chamber,omitempty"`
	Introduced *string   `json:"introduced,omitempty"`
	PolicyArea *string   `json:"policy_area,omitempty"`
	Subjects   *[]string `json:"subjects,omitempty"`
}

// ReparseResult answers a reparse.
type ReparseResult struct {
	Bill          string `json:"bill"`
	Version       string `json:"version"` // version code, for example "eh"
	StoredParser  string `json:"stored_parser_version"`
	CurrentParser string `json:"current_parser_version"`
	Changed       bool   `json:"changed"`
	Note          string `json:"note,omitempty"`
}

// PolicyRequest is the body of POST /selection-policy: the policy document
// (the same JSON job-ingest --publish-policy reads) and the public reason.
type PolicyRequest struct {
	Reasoned

	Policy json.RawMessage `json:"policy"`
}

// PolicyResult answers a policy publication.
type PolicyResult struct {
	Version int    `json:"version"`
	Changed bool   `json:"changed"`
	LogID   string `json:"log_id,omitempty"`
}

// GenerationRequest is the body of POST /generations.
type GenerationRequest struct {
	Reasoned

	Task    string `json:"task"`
	Bill    string `json:"bill"`
	Version string `json:"version"` // version code, for example "eh"
}

// SourceState is the ingest state of one source, for ingest failures and jobs.
type SourceState struct {
	Source      string       `json:"source"`
	Watermark   string       `json:"watermark,omitempty"`
	AsOf        string       `json:"as_of,omitempty"`
	FailedCount int          `json:"failed_count"`
	Failed      []FailedItem `json:"failed,omitempty"`
}

// FailedItem is one item a pull could not store.
type FailedItem struct {
	Item  string `json:"item"`
	Error string `json:"error"`
	At    string `json:"at"`
}

// StateList is the body of GET /ingest/failures and GET /jobs.
type StateList struct {
	Sources []SourceState `json:"sources"`
}

// RosterEntry is one operator of the public roster: id and public name only.
type RosterEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// RosterRequest is the body of POST /operators/roster. Principals are never sent.
type RosterRequest struct {
	Reasoned

	// Operators is the full new roster.
	Operators []RosterEntry `json:"operators"`
	// Approver is the id of the approving operator; empty only for the bootstrap.
	Approver string `json:"approver,omitempty"`
}

// RosterResult answers a roster call.
type RosterResult struct {
	Written   []string `json:"written"` // log ids written; empty when nothing changed
	Bootstrap bool     `json:"bootstrap,omitempty"`
}
