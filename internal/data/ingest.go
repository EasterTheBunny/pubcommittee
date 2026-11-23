package data

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// SourceInfo is the provenance of one original document: where it came from
// and when it was retrieved. It is stored beside the original XML (the object
// sidecar), not inside text.json, so a retried write of the parsed version
// stays byte-identical.
type SourceInfo struct {
	URL         string    `json:"url"`
	RetrievedAt time.Time `json:"retrieved_at"`
}

// SourceInfoReader is implemented by stores that keep SourceInfo with the
// original XML of a version. ErrNotFound when the version is not stored.
type SourceInfoReader interface {
	GetSourceInfo(ctx context.Context, ref VersionRef) (SourceInfo, error)
}

// Action is one entry of a bill's action history.
type Action struct {
	Date string `json:"date"` // YYYY-MM-DD
	Text string `json:"text"`
}

// CommitteeActivity is one recorded step of a committee on a bill, for
// example "Referred To", "Markup By", "Reported By".
type CommitteeActivity struct {
	Committee string    `json:"committee"`
	Chamber   string    `json:"chamber,omitempty"`
	Activity  string    `json:"activity"`
	At        time.Time `json:"at"`
}

// RelatedBill is a bill the source relates to another.
type RelatedBill struct {
	Bill         BillRef `json:"bill"`
	Title        string  `json:"title,omitempty"`
	Relationship string  `json:"relationship,omitempty"`
}

// SourceVersion is one official text version as a bill status lists it.
type SourceVersion struct {
	Ref         VersionRef
	Type        string // for example "Introduced in House"
	PublishedAt time.Time
	URL         string // official location of the original XML
}

// BillRecord is everything a bill status says that the catalog keeps.
// Sponsors are keyed on bioguide id, never on name.
type BillRecord struct {
	Bill              BillRef
	Title             string // display title
	Chamber           string // ChamberHouse, ChamberSenate or ChamberJoint
	Introduced        string // YYYY-MM-DD
	UpdatedAt         time.Time
	SponsorID         string // bioguide id; empty when the bill has no sponsor
	Sponsor           string // display name as the source prints it
	LatestAction      Action
	PolicyArea        string
	Subjects          []string
	CommitteeActivity []CommitteeActivity
	Related           []RelatedBill
	Versions          []SourceVersion // oldest first
	BillPageURL       string          // the legislature's page for the bill
	Source            SourceInfo      // where the status itself came from
}

// SourceDocument is one fetched original, byte for byte.
type SourceDocument struct {
	XML  []byte
	Info SourceInfo
}

// BillFetcher fetches one bill's status record and its text versions from an
// official source (or its fixture copy). Not found: ErrNotFound; a source that
// is down or refuses: ErrUnavailable or ErrRateLimited.
type BillFetcher interface {
	// FetchRecord returns the status record of a bill.
	FetchRecord(ctx context.Context, bill BillRef) (BillRecord, error)
	// FetchOriginal returns the original XML of one listed text version.
	FetchOriginal(ctx context.Context, v SourceVersion) (SourceDocument, error)
}

// ID returns the bill id used in URLs, file names and fixtures, for example
// "us-119-hr-4313".
func (b BillRef) ID() string {
	return fmt.Sprintf("%s-%s-%s-%d", b.Jurisdiction, b.Session, b.Type, b.Number)
}

// ParseBillID is the inverse of BillRef.ID; ErrInvalid for anything else.
func ParseBillID(id string) (BillRef, error) {
	parts := strings.Split(id, "-")
	const idParts = 4
	if len(parts) != idParts {
		return BillRef{}, fmt.Errorf("bill id %q: %w", id, ErrInvalid)
	}
	n, err := strconv.Atoi(parts[3])
	if err != nil {
		return BillRef{}, fmt.Errorf("bill id %q: %w", id, ErrInvalid)
	}
	ref := BillRef{Jurisdiction: parts[0], Session: parts[1], Type: parts[2], Number: n}
	if err := ref.Validate(); err != nil {
		return BillRef{}, err
	}
	return ref, nil
}

// IngestFailure is one item a pull could not store.
type IngestFailure struct {
	Item  string    `json:"item"`  // bill id, or version key for one text version
	Error string    `json:"error"` // short reason, no source content
	At    time.Time `json:"at"`
}
