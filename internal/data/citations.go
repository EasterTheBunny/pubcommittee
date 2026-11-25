package data

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Kinds of citation (CitationEntry.Kind).
const (
	CiteUSC        = "usc"         // a section or subdivision of the US Code
	CiteUSCChapter = "usc-chapter" // a chapter of a US Code title
	CitePublicLaw  = "public-law"  // a public law as a whole
	CiteActSection = "act-section" // a section of a named Act, such as "section 1848 of the Social Security Act"
)

// Sources of a citation (CitationEntry.Source).
const (
	SourceMarkup = "markup" // the bill XML's own external-xref element
	SourceText   = "text"   // a text pattern in the line
)

// Release is one stored US Code release point. Every provision read from it
// carries Date as the date the consolidated source was last current
// (design/01 § Existing law freshness).
type Release struct {
	ID        string    `json:"id"`         // for example "119-142"
	PublicLaw string    `json:"public_law"` // last public law the release includes, for example "119-142"
	Date      time.Time `json:"date"`       // date the code is current through
	Meta      Meta      `json:"meta"`
}

// Validate reports ErrInvalid for an empty id or a zero date.
func (r Release) Validate() error {
	if r.ID == "" || strings.ContainsAny(r.ID, "/.") || r.Date.IsZero() {
		return fmt.Errorf("release %q: %w", r.ID, ErrInvalid)
	}
	return nil
}

// ObjectPath returns the storage path of the release record.
func (r Release) ObjectPath() string { return releasePath(r.ID) }

func releasePath(id string) string { return "law/usc/" + id + "/release.json" }

// CurrentReleasePath is the storage path of the replaceable pointer to the
// newest stored release.
const CurrentReleasePath = "law/usc/current.json"

// ReleasePath returns the storage path of the release record with this id.
func ReleasePath(id string) string { return releasePath(id) }

var sectionPattern = regexp.MustCompile(`^[0-9A-Za-z]+(-[0-9A-Za-z]+)*$`)

// ProvisionRef addresses one US Code section within a release point. Section
// is normalised: the hyphen-minus replaces the dashes of the source
// ("1395cc-7").
type ProvisionRef struct {
	Release string `json:"release"`
	Title   int    `json:"title"`
	Section string `json:"section"`
}

// String returns "<release>:usc/<title>/<section>".
func (r ProvisionRef) String() string {
	return r.Release + ":" + ProvisionID(r.Title, r.Section)
}

// Validate reports ErrInvalid for a malformed reference.
func (r ProvisionRef) Validate() error {
	if r.Release == "" || strings.ContainsAny(r.Release, "/.") || r.Title <= 0 || !sectionPattern.MatchString(r.Section) {
		return fmt.Errorf("provision ref %+v: %w", r, ErrInvalid)
	}
	return nil
}

// ObjectPath returns the bucket-relative storage path of the section.
func (r ProvisionRef) ObjectPath() string {
	return "law/usc/" + r.Release + "/t" + strconv.Itoa(r.Title) + "/" + r.Section + ".json"
}

// ProvisionID returns the stable id of a provision: "usc/42/1395cc-7" for a
// section, "usc/42/1395cc-7/a/1" for a subdivision with path ["a", "1"].
func ProvisionID(title int, section string, path ...string) string {
	parts := append([]string{"usc", strconv.Itoa(title), section}, path...)
	return strings.Join(parts, "/")
}

// ParseProvisionID splits an id made by ProvisionID.
func ParseProvisionID(id string) (title int, section string, path []string, err error) {
	parts := strings.Split(id, "/")
	if len(parts) < 3 || parts[0] != "usc" {
		return 0, "", nil, fmt.Errorf("provision id %q: %w", id, ErrInvalid)
	}
	title, err = strconv.Atoi(parts[1])
	if err != nil || title <= 0 || !sectionPattern.MatchString(parts[2]) {
		return 0, "", nil, fmt.Errorf("provision id %q: %w", id, ErrInvalid)
	}
	return title, parts[2], parts[3:], nil
}

// LawSection is one US Code section at one release point with its
// subdivisions. It is official text, stored write-once per ref.
type LawSection struct {
	Ref          ProvisionRef `json:"ref"`
	Heading      string       `json:"heading"`
	Status       string       `json:"status,omitempty"` // "repealed", "transferred", ... when the source says so
	SourceSHA256 string       `json:"source_sha256"`    // hash of the title file the section came from
	// Provisions holds the section first, then each subdivision in document
	// order. Text of a provision includes the text of its subdivisions.
	Provisions []Provision `json:"provisions"`
	Meta       Meta        `json:"meta"`
}

// Find returns the provision with this id.
func (s LawSection) Find(id string) (Provision, bool) {
	for _, p := range s.Provisions {
		if p.ID == id {
			return p, true
		}
	}
	return Provision{}, false
}

// CitationRef identifies one stored set of citations: a version and the
// method that produced it (parser version, citation rules version, release
// point). The record is write-once per ref.
type CitationRef struct {
	Version VersionRef `json:"version"`
	Method  string     `json:"method"`
}

// String returns the canonical key.
func (r CitationRef) String() string { return r.Version.String() + "/citations/" + r.Method }

// Validate reports ErrInvalid for an empty or malformed reference.
func (r CitationRef) Validate() error {
	if err := r.Version.Validate(); err != nil {
		return err
	}
	if r.Method == "" {
		return fmt.Errorf("citations %s: empty method: %w", r, ErrInvalid)
	}
	return nil
}

// ObjectPath returns the bucket-relative storage path. "/" inside the method
// is written as "_".
func (r CitationRef) ObjectPath() string {
	return "bills/" + r.Version.Bill.String() + "/v/" + r.Version.Version + "/citations/" +
		strings.ReplaceAll(r.Method, "/", "_") + ".json"
}

// FreshnessPath is the object path of the set's FreshnessDoc, beside the set.
func (r CitationRef) FreshnessPath() string {
	return strings.TrimSuffix(r.ObjectPath(), ".json") + ".freshness.json"
}

// CitationEntry is a CitationLink with the reasoning kept for the public
// list: what kind of reference it is, what it was normalised to, and why it
// is not resolved.
type CitationEntry struct {
	CitationLink

	Kind   string `json:"kind"`
	Target string `json:"target,omitempty"` // normalised target as cited, for example "usc/42/1395cc-7/a/1"
	Reason string `json:"reason,omitempty"` // why the state is not resolved
	Source string `json:"source"`           // SourceMarkup or SourceText
}

// CitedProvision is a cited provision excerpt with its release point, as
// shown beside a bill section. Text is cut at the excerpt limit; Truncated
// says so.
type CitedProvision struct {
	Provision

	Release   string `json:"release"`
	Truncated bool   `json:"truncated,omitempty"`
}

// FreshnessFlag says that a public law enacted after the release point may
// have changed a cited provision, so its explanation needs a steward.
type FreshnessFlag struct {
	ProvisionID string    `json:"provision_id"`
	PublicLaw   string    `json:"public_law"` // for example "119-150"
	EnactedAt   time.Time `json:"enacted_at"`
}

// FreshnessDoc holds the current freshness flags of one citation set. The set
// is write-once; this document is replaceable: every citation step recomputes
// it from the later laws stored at that time, so a law enacted after the set
// was first written is still flagged (design/01 § Existing law freshness).
type FreshnessDoc struct {
	Ref   CitationRef     `json:"ref"`
	Flags []FreshnessFlag `json:"flags"`
	// Laws lists the later public laws the flags were computed from.
	Laws []string `json:"laws"`
}

// CitationSet is everything the citation step stores for one version.
type CitationSet struct {
	Ref         CitationRef      `json:"ref"`
	Release     string           `json:"release"`
	ReleaseDate time.Time        `json:"release_date"`
	Links       []CitationEntry  `json:"links"`
	Provisions  []CitedProvision `json:"provisions"`
	// Freshness is empty in sets the step writes: the current flags are in the
	// set's FreshnessDoc. Sets written before it existed keep the flags of
	// their first write.
	Freshness []FreshnessFlag `json:"freshness"`
	Meta      Meta            `json:"meta"`
}

// Unresolved returns the entries whose state is unresolved or ambiguous.
func (s CitationSet) Unresolved() []CitationEntry {
	var out []CitationEntry
	for _, e := range s.Links {
		if e.State == CitationUnresolved || e.State == CitationAmbiguous {
			out = append(out, e)
		}
	}
	return out
}

// CitationReader reads stored citation sets. Not found: ErrNotFound.
type CitationReader interface {
	// GetCitations returns the set with exactly this ref.
	GetCitations(ctx context.Context, ref CitationRef) (CitationSet, error)
	// GetFreshness returns the current freshness flags of the set with this ref.
	GetFreshness(ctx context.Context, ref CitationRef) (FreshnessDoc, error)
}

// CitationWriter stores citation sets.
type CitationWriter interface {
	// PutCitations stores a set at a write-once ref. Identical content is a
	// no-op; different content is ErrConflict; an invalid ref is ErrInvalid.
	PutCitations(ctx context.Context, s CitationSet) error
	// PutFreshness stores or replaces the freshness flags of a set; an invalid
	// ref is ErrInvalid.
	PutFreshness(ctx context.Context, d FreshnessDoc) error
}

// LawReader reads stored US Code release points and sections. Not found:
// ErrNotFound.
type LawReader interface {
	// CurrentRelease returns the newest stored release; ErrNotFound if none.
	CurrentRelease(ctx context.Context) (Release, error)
	// GetRelease returns the release with this id.
	GetRelease(ctx context.Context, id string) (Release, error)
	// GetLawSection returns one section with its subdivisions.
	GetLawSection(ctx context.Context, ref ProvisionRef) (LawSection, error)
}

// LawWriter stores US Code release points and sections. Nothing stored is
// ever overwritten, except the pointer to the current release.
type LawWriter interface {
	// PutLawSection stores a section at a write-once ref. Identical content
	// is a no-op; different content is ErrConflict.
	PutLawSection(ctx context.Context, s LawSection) error
	// PutRelease stores a release record, write-once like PutLawSection.
	PutRelease(ctx context.Context, r Release) error
	// SetCurrentRelease points "current" at a stored release; ErrNotFound if
	// the release is not stored. It never moves the pointer to an older
	// release (by Date): that is a no-op.
	SetCurrentRelease(ctx context.Context, id string) error
}
