package data

import "context"

// BillText reads immutable version text. Not found: ErrNotFound.
type BillText interface {
	// ListVersions returns the stored versions of a bill, oldest first.
	ListVersions(ctx context.Context, bill BillRef) ([]VersionSummary, error)
	// GetVersion returns one official version with all sections and lines.
	GetVersion(ctx context.Context, ref VersionRef) (OfficialVersion, error)
	// GetLines returns the lines an anchor addresses; ErrNotFound when the
	// section or any line of the range does not exist.
	GetLines(ctx context.Context, a Anchor) ([]Line, error)
	// GetSource returns the original source XML of a version.
	GetSource(ctx context.Context, ref VersionRef) ([]byte, error)
}

// PublicDocs reads the public mutable documents.
type PublicDocs interface {
	// GetBill returns bill metadata.
	GetBill(ctx context.Context, bill BillRef) (Bill, error)
	// ListCatalog pages through catalog entries ordered by bill key.
	ListCatalog(ctx context.Context, p Page) (Paged[CatalogEntry], error)
	// GetExplanations returns all change explanations of one version.
	GetExplanations(ctx context.Context, ref VersionRef) ([]ChangeExplanation, error)
	// GetSelectionPolicy returns the latest policy version.
	GetSelectionPolicy(ctx context.Context) (SelectionPolicy, error)
	// GetSelectionPolicyVersion returns one policy version; ErrNotFound if
	// that version does not exist.
	GetSelectionPolicyVersion(ctx context.Context, version int) (SelectionPolicy, error)
	// ListLog pages through the public log, newest first.
	ListLog(ctx context.Context, p Page) (Paged[LogEntry], error)
}

// Moderation records actions in the public log.
type Moderation interface {
	// Record appends an entry; recording an entry with the same ID and
	// identical content is a no-op, with different content ErrConflict.
	Record(ctx context.Context, e LogEntry) error
}

// Change is one platform document write. Exactly one field is set;
// otherwise ErrInvalid.
type Change struct {
	Bill        *Bill
	Catalog     *CatalogEntry
	Policy      *SelectionPolicy
	Explanation *ChangeExplanation
}

// PlatformDocs writes the public documents the platform itself changes.
type PlatformDocs interface {
	// PutBill creates or replaces bill metadata.
	PutBill(ctx context.Context, b Bill) error
	// PutCatalogEntry creates or replaces a catalog entry (also sets tracked).
	PutCatalogEntry(ctx context.Context, e CatalogEntry) error
	// PutSelectionPolicy writes a policy version; an existing Version
	// number is write-once (ErrConflict if content differs).
	PutSelectionPolicy(ctx context.Context, p SelectionPolicy) error
	// PutExplanation creates or replaces an explanation, keyed by
	// (version, section, provision).
	PutExplanation(ctx context.Context, e ChangeExplanation) error
	// PutGeneration records one generation; the ID is write-once.
	PutGeneration(ctx context.Context, g GenerationRecord) error
	// GetIngestState returns the state of a source; ErrNotFound if none.
	GetIngestState(ctx context.Context, source string) (IngestState, error)
	// PutIngestState replaces the state of a source.
	PutIngestState(ctx context.Context, s IngestState) error
	// ApplyLogged writes the change and its log entry atomically, or neither.
	// Replaying the same entry ID with the same content is a no-op.
	ApplyLogged(ctx context.Context, c Change, e LogEntry) error
}

// Publisher writes immutable text, exports, and feeds.
type Publisher interface {
	// PutVersion stores a version and its source XML at a write-once
	// path. Identical content is idempotent; different content is ErrConflict.
	PutVersion(ctx context.Context, v OfficialVersion, sourceXML []byte) error
	// PutImmutable stores a file at a write-once path (exports).
	PutImmutable(ctx context.Context, path string, data []byte) error
	// PutReplaceable stores a file that later calls may replace (feeds)
	// and returns its content hash as ETag.
	PutReplaceable(ctx context.Context, path string, data []byte) (etag string, err error)
	// GetFile reads a published file; ErrNotFound if absent.
	GetFile(ctx context.Context, path string) ([]byte, error)
}

// ChangedBill is one entry of a source's changed-bill list.
type ChangedBill struct {
	Bill      BillRef
	UpdatedAt string // source timestamp, RFC 3339
}

// BillStatus is a source's status record for one bill.
type BillStatus struct {
	Bill     BillRef
	Title    string
	Sponsor  string
	Status   string
	Versions []VersionSummary
}

// BillSource lists and fetches bills from an official source.
type BillSource interface {
	// ListChanged lists bills changed at or after the watermark (RFC 3339;
	// empty means all), ordered by update time then bill key.
	ListChanged(ctx context.Context, since string, p Page) (Paged[ChangedBill], error)
	// FetchStatus returns the status record of a bill.
	FetchStatus(ctx context.Context, bill BillRef) (BillStatus, error)
	// FetchVersionXML returns the original XML of one text version.
	FetchVersionXML(ctx context.Context, ref VersionRef) ([]byte, error)
}

// ReleasePoint identifies one consolidated-law release.
type ReleasePoint struct {
	ID string // for example "119-12"
}

// LawSource lists and fetches existing-law release points.
type LawSource interface {
	// ListReleasePoints lists known release points, newest first.
	ListReleasePoints(ctx context.Context) ([]ReleasePoint, error)
	// FetchTitle returns the XML of one US Code title at a release point.
	FetchTitle(ctx context.Context, rp ReleasePoint, title int) ([]byte, error)
}

// GenRequest is one summarization task.
type GenRequest struct {
	ID    string // caller-chosen, unique within a batch
	Task  string
	Input string
}

// GenResult is the outcome of one GenRequest.
type GenResult struct {
	ID        string
	Outcome   string // ok, refusal, invalid, truncated, error
	Text      string
	Model     string
	TokensIn  int
	TokensOut int
}

// Generator submits generation tasks.
type Generator interface {
	// Submit starts a batch and returns its ID. Empty requests: ErrInvalid.
	Submit(ctx context.Context, reqs []GenRequest) (batchID string, err error)
	// Collect returns the results of a batch; done is false while pending.
	Collect(ctx context.Context, batchID string) (results []GenResult, done bool, err error)
	// Generate runs one request synchronously (steward-requested).
	Generate(ctx context.Context, req GenRequest) (GenResult, error)
}
