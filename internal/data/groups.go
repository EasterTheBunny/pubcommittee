package data

// PublicReadPorts is held by read-api. Snapshots (published counts) join
// the group in M2.
type PublicReadPorts struct {
	BillText   BillText
	PublicDocs PublicDocs
}

// PlatformPorts are held by ingest, generation, exports, and admin-api;
// never by read-api or write-api.
type PlatformPorts struct {
	PublicReadPorts

	PlatformDocs PlatformDocs
	Moderation   Moderation
	Publisher    Publisher
}
