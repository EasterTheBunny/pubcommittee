package data

import (
	"fmt"
	"strings"
	"time"
)

// ContentClass says who wrote a record's text; consumers treat anything but
// ContentOfficial as data, never as instructions (design/10).
type ContentClass string

// Content classes.
const (
	ContentOfficial ContentClass = "official"
	ContentPlatform ContentClass = "platform"
	ContentUser     ContentClass = "user"
)

// Provenance says how a record was produced.
type Provenance string

// Provenance values; the first three match the UI's provenance flags.
const (
	ProvenanceAuto           Provenance = "auto"
	ProvenanceStewardChecked Provenance = "steward-checked"
	ProvenanceBillOnly       Provenance = "bill-only"
	ProvenanceSource         Provenance = "source"
	ProvenanceStewardEntered Provenance = "steward-entered"
)

// Meta is carried by every content record that holds text.
type Meta struct {
	Version      int          `json:"version"` // revision of this record, starting at 1
	AsOf         time.Time    `json:"as_of"`
	Provenance   Provenance   `json:"provenance"`
	ContentClass ContentClass `json:"content_class"`
}

// BillRef identifies a bill within a jurisdiction and session.
type BillRef struct {
	Jurisdiction string `json:"jurisdiction"` // for example "us"
	Session      string `json:"session"`      // for example "119"
	Type         string `json:"type"`         // for example "hr", "s"
	Number       int    `json:"number"`
}

// String returns the canonical key, for example "us/119/hr/1234".
func (b BillRef) String() string {
	return fmt.Sprintf("%s/%s/%s/%d", b.Jurisdiction, b.Session, b.Type, b.Number)
}

// Validate reports ErrInvalid when a field is empty.
func (b BillRef) Validate() error {
	if b.Jurisdiction == "" || b.Session == "" || b.Type == "" || b.Number <= 0 {
		return fmt.Errorf("bill ref %+v: %w", b, ErrInvalid)
	}
	return nil
}

// VersionRef identifies one official version of a bill.
type VersionRef struct {
	Bill    BillRef `json:"bill"`
	Version string  `json:"version"` // version code, for example "ih", "eh", "enr"
}

// String returns the canonical key, for example "us/119/hr/1234/ih".
func (v VersionRef) String() string { return v.Bill.String() + "/" + v.Version }

// Validate reports ErrInvalid for an empty or malformed reference.
func (v VersionRef) Validate() error {
	if err := v.Bill.Validate(); err != nil {
		return err
	}
	if v.Version == "" || strings.Contains(v.Version, "/") {
		return fmt.Errorf("version %q: %w", v.Version, ErrInvalid)
	}
	return nil
}

// Line is the smallest addressable text unit of a version.
type Line struct {
	N        int    `json:"n"`     // unique within a version, from 1
	Depth    int    `json:"depth"` // 0 for top level; deeper lines nest under the previous shallower line
	Text     string `json:"text"`
	SourceID string `json:"source_id,omitempty"` // XML id where one exists
}

// Section is a structural unit of a version.
type Section struct {
	Path     []string `json:"path"` // section path, for example ["s4", "b"]
	Label    string   `json:"label"`
	Heading  string   `json:"heading"`
	Lines    []Line   `json:"lines"`
	SourceID string   `json:"source_id,omitempty"`
}

// OfficialVersion is an immutable snapshot of a bill's text.
type OfficialVersion struct {
	Ref           VersionRef `json:"ref"`
	Type          string     `json:"type"` // introduced, reported, engrossed, ...
	PublishedAt   time.Time  `json:"published_at"`
	SourceSHA256  string     `json:"source_sha256"`            // hash of the original XML
	ParserVersion string     `json:"parser_version,omitempty"` // parser that produced Sections, for example "billxml/1"
	// Source is where the original came from. A Publisher keeps it with
	// source.xml; it is never part of the stored parsed text.
	Source         SourceInfo `json:"-"`
	Sections       []Section  `json:"sections"`
	Meta           Meta       `json:"meta"`
	StewardEntered bool       `json:"steward_entered,omitempty"`
}

// VersionSummary lists a version without its text.
type VersionSummary struct {
	Ref         VersionRef `json:"ref"`
	Type        string     `json:"type"`
	PublishedAt time.Time  `json:"published_at"`
}

// Anchor is a stable reference to a line or line range in one official
// version: (bill, version, section path, line or line range).
type Anchor struct {
	Version  VersionRef `json:"version"`
	Section  []string   `json:"section"`
	From     int        `json:"from"`
	To       int        `json:"to"` // equals From for a single line
	SourceID string     `json:"source_id,omitempty"`
}

// Validate reports ErrInvalid for a malformed anchor.
func (a Anchor) Validate() error {
	if err := a.Version.Validate(); err != nil {
		return err
	}
	if len(a.Section) == 0 || a.From < 1 || a.To < a.From {
		return fmt.Errorf("anchor %+v: %w", a, ErrInvalid)
	}
	return nil
}

// Block returns the anchor range of the block starting at line n of s: the
// line and every following line nested deeper than it. ok is false when the
// section has no line n.
func Block(s Section, n int) (from, to int, ok bool) {
	for i, l := range s.Lines {
		if l.N != n {
			continue
		}
		to = n
		for _, next := range s.Lines[i+1:] {
			if next.Depth <= l.Depth {
				break
			}
			to = next.N
		}
		return n, to, true
	}
	return 0, 0, false
}

// Chamber values of Bill.Chamber and CatalogEntry.Chamber.
const (
	ChamberHouse  = "house"
	ChamberSenate = "senate"
	ChamberJoint  = "joint"
)

// Review states of Bill.ReviewState.
const (
	ReviewOpen   = "open"
	ReviewClosed = "closed"
)

// Markup statuses of CatalogEntry.MarkupStatus.
const (
	MarkupInReview      = "in-review"
	MarkupEligible      = "eligible"
	MarkupNotACandidate = "not-a-candidate"
)

// Link kinds of Link.Kind.
const (
	LinkBillPage    = "bill-page"
	LinkTextVersion = "text-version"
	LinkOther       = "other"
)

// Link points to an official page for a bill.
type Link struct {
	Label string `json:"label"`
	URL   string `json:"url"`
	Kind  string `json:"kind,omitempty"`
}

// Bill holds a bill's metadata and its version list. ReviewState is empty
// for documents written before M1-10; readers treat empty as ReviewOpen.
type Bill struct {
	Ref            BillRef          `json:"ref"`
	Title          string           `json:"title"`
	Sponsor        string           `json:"sponsor"`
	SponsorID      string           `json:"sponsor_id,omitempty"` // bioguide id
	Status         string           `json:"status"`
	Chamber        string           `json:"chamber,omitempty"`
	ReviewState    string           `json:"review_state,omitempty"`
	OfficialLinks  []Link           `json:"official_links,omitempty"`
	TrackedReasons []string         `json:"tracked_reasons,omitempty"`
	PolicyVersion  int              `json:"policy_version,omitempty"`
	Versions       []VersionSummary `json:"versions"`
	Meta           Meta             `json:"meta"`
}

// CatalogEntry is one row of the public catalog. Untracked entries carry no
// bill text, only metadata and official links.
type CatalogEntry struct {
	Bill              BillRef             `json:"bill"`
	Title             string              `json:"title"`
	Status            string              `json:"status,omitempty"`
	Chamber           string              `json:"chamber,omitempty"`
	Subjects          []string            `json:"subjects,omitempty"`
	CommitteeActivity bool                `json:"committee_activity,omitempty"`
	Tracked           bool                `json:"tracked"`
	Reasons           []string            `json:"reasons"`
	PolicyVersion     int                 `json:"policy_version,omitempty"` // policy version that tracked the bill
	TrackedSince      string              `json:"tracked_since,omitempty"`  // YYYY-MM-DD the bill was first tracked
	MarkupStatus      string              `json:"markup_status,omitempty"`
	ExclusionReason   string              `json:"exclusion_reason,omitempty"`
	OfficialLinks     []Link              `json:"official_links,omitempty"`
	SearchTerms       []string            `json:"search_terms,omitempty"` // bill number, title words, subjects
	SponsorID         string              `json:"sponsor_id,omitempty"`   // bioguide id
	Sponsor           string              `json:"sponsor,omitempty"`
	SponsorParty      string              `json:"sponsor_party,omitempty"` // disclosure counts only; never a policy input
	Introduced        string              `json:"introduced,omitempty"`    // YYYY-MM-DD
	LatestAction      *Action             `json:"latest_action,omitempty"`
	PolicyArea        string              `json:"policy_area,omitempty"`
	Committees        []CommitteeActivity `json:"committees,omitempty"`
	Related           []RelatedBill       `json:"related,omitempty"`
	Meta              Meta                `json:"meta"`
}

// Provision is an addressable unit of existing law.
type Provision struct {
	ID          string    `json:"id"`       // for example "usc/42/1395"
	Citation    string    `json:"citation"` // for example "42 U.S.C. 1395"
	Text        string    `json:"text"`
	CurrentAsOf time.Time `json:"current_as_of"` // date the consolidated source was last current
	Meta        Meta      `json:"meta"`
}

// CitationState is the confidence state of a citation link.
type CitationState string

// Citation states.
const (
	CitationResolved         CitationState = "resolved"
	CitationAmbiguous        CitationState = "ambiguous"
	CitationUnresolved       CitationState = "unresolved"
	CitationStewardConfirmed CitationState = "steward-confirmed"
)

// CitationLink resolves a reference in a bill section to provisions.
type CitationLink struct {
	Version      VersionRef    `json:"version"`
	Section      []string      `json:"section"`
	Line         int           `json:"line,omitempty"` // line holding the citation, when known
	Text         string        `json:"text"`           // the citation as written in the bill
	ProvisionIDs []string      `json:"provision_ids"`
	State        CitationState `json:"state"`
}

// ChangeExplanation is attached to a (bill section, provision) pair.
type ChangeExplanation struct {
	Version     VersionRef   `json:"version"`
	Section     []string     `json:"section"`
	ProvisionID string       `json:"provision_id,omitempty"`
	CurrentText string       `json:"current_text,omitempty"`
	WhatChanges string       `json:"what_changes"`
	WhyMatters  string       `json:"why_it_matters"`
	Flags       []Provenance `json:"flags"`
	AsAmended   string       `json:"as_amended,omitempty"` // unofficial preview
	Meta        Meta         `json:"meta"`
}

// GenerationRecord records one model generation for audit.
type GenerationRecord struct {
	ID         string    `json:"id"`
	Task       string    `json:"task"`
	Model      string    `json:"model"`
	PromptHash string    `json:"prompt_hash"`
	Outcome    string    `json:"outcome"` // ok, refusal, invalid, truncated, error
	At         time.Time `json:"at"`
	TokensIn   int       `json:"tokens_in"`
	TokensOut  int       `json:"tokens_out"`
	Meta       Meta      `json:"meta"`
}

// SelectionPolicy is one version of the rules that choose tracked bills
// ([architecture § Bill selection]). Rules is the plain-language statement of
// the structured fields below, generated from them; versions stored before the
// structured fields existed carry Rules only. A version is immutable.
type SelectionPolicy struct {
	Version           int                    `json:"version"`
	Rules             []string               `json:"rules"`
	Mode              string                 `json:"mode,omitempty"`  // PolicyModePilot or PolicyModeGeneral
	Phase             string                 `json:"phase,omitempty"` // PolicyPhaseEarlyPilot or PolicyPhaseStewardSignoff
	MeasureKinds      []string               `json:"measure_kinds,omitempty"`
	ExcludeKinds      []string               `json:"exclude_kinds,omitempty"`
	CommitteeActivity []string               `json:"committee_activity,omitempty"`
	FollowCompanions  bool                   `json:"follow_companions,omitempty"`
	Topics            map[string]PolicyTopic `json:"topics,omitempty"`
	Meta              Meta                   `json:"meta"`
}

// PolicyTopic is one focus topic: the legislature's own policy areas and
// legislative subjects, and title terms that let a bill match before subject
// indexing arrives.
type PolicyTopic struct {
	PolicyAreas []string `json:"policy_areas,omitempty"`
	Subjects    []string `json:"subjects,omitempty"`
	TitleTerms  []string `json:"title_terms,omitempty"`
}

// Selection policy modes and phases.
const (
	PolicyModePilot           = "pilot"
	PolicyModeGeneral         = "general"
	PolicyPhaseEarlyPilot     = "early_pilot"
	PolicyPhaseStewardSignoff = "steward_signoff"
)

// LogEntry is a public log entry (moderation or platform action).
type LogEntry struct {
	ID      string    `json:"id"`
	At      time.Time `json:"at"`
	Kind    string    `json:"kind"`
	Subject string    `json:"subject"`
	Reason  string    `json:"reason"`
	// Actor is the named administrator of an administrator action (M1-12):
	// operator id and public name, never a Google principal. Empty for
	// entries no administrator made.
	Actor *LogOperator `json:"actor,omitempty"`
	// Operator is the subject of a roster entry (operator_added,
	// operator_renamed, operator_removed): the id and public name after the
	// change (the last public name, for a removal). Actor is then the
	// approving operator.
	Operator *LogOperator `json:"operator,omitempty"`
	// Bootstrap marks a roster entry of the very first roster, which needs
	// no approving operator.
	Bootstrap bool `json:"bootstrap,omitempty"`
	Meta      Meta `json:"meta"`
}

// LogOperator is an administrator as the public log names one.
type LogOperator struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// IngestState records the progress of ingestion for one source.
type IngestState struct {
	Source    string    `json:"source"`
	Watermark time.Time `json:"watermark"`
	// Failed lists items a pull could not store, for the next pull to retry
	// and for operators to inspect.
	Failed []IngestFailure `json:"failed,omitempty"`
	Meta   Meta            `json:"meta"`
}

// Page requests one page of a list. Cursor is opaque and owned by the
// adapter; the empty cursor starts at the beginning. Limit 0 means the
// adapter default.
type Page struct {
	Cursor string
	Limit  int
}

// Paged is one page of results; Next is empty on the last page.
type Paged[T any] struct {
	Items []T
	Next  string
}
