package data

// PublicReadPorts is held by read-api. Snapshots (published counts) join
// the group in M2.
type PublicReadPorts struct {
	BillText   BillText
	PublicDocs PublicDocs
	// Comparisons may be nil where an adapter stores none; readers then
	// answer not found.
	Comparisons ComparisonReader
	// Citations and Law may be nil likewise.
	Citations CitationReader
	Law       LawReader
	// Feed and Exports may be nil likewise.
	Feed    FeedReader
	Exports ExportReader
}

// PlatformPorts are held by ingest, generation, exports, and admin-api;
// never by read-api or write-api.
type PlatformPorts struct {
	PublicReadPorts

	PlatformDocs    PlatformDocs
	Moderation      Moderation
	Publisher       Publisher
	ComparisonStore ComparisonWriter
	CitationStore   CitationWriter
	LawStore        LawWriter
	FeedStore       FeedWriter
	ExportStore     ExportWriter
}
