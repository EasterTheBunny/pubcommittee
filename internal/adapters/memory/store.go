// Package memory implements every M1 port in process memory. Nothing
// persists. One Store backs all ports, so text written through Publisher is
// readable through BillText, and documents written through PlatformDocs are
// readable through PublicDocs. Methods are split into files by port group:
// billtext.go (BillText, Publisher), docs.go (PublicDocs, PlatformDocs,
// Moderation), sources.go (BillSource, LawSource, Generator).
package memory

import (
	"sync"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// Store is the in-memory implementation of every M1 port. The zero value is
// not usable; call New.
type Store struct {
	mu sync.RWMutex

	// billtext.go: keyed by data.VersionRef.String().
	versions map[string]data.OfficialVersion
	sources  map[string][]byte
	// billtext.go: published files keyed by path; once marks write-once paths.
	files map[string][]byte
	once  map[string]bool

	// docs.go.
	bills        map[string]data.Bill // keyed by BillRef.String()
	catalog      map[string]data.CatalogEntry
	policies     map[int]data.SelectionPolicy
	explanations map[string]data.ChangeExplanation
	generations  map[string]data.GenerationRecord
	ingest       map[string]data.IngestState
	log          map[string]data.LogEntry

	// sources.go: seeded source data.
	changed       []data.ChangedBill
	statuses      map[string]data.BillStatus // keyed by BillRef.String()
	versionXML    map[string][]byte          // keyed by VersionRef.String()
	releasePoints []data.ReleasePoint
	titles        map[string][]byte // keyed by "<release point id>/<title>"
	batches       map[string][]data.GenResult
	batchCount    int
}

// New returns an empty Store.
func New() *Store {
	return &Store{
		versions:     map[string]data.OfficialVersion{},
		sources:      map[string][]byte{},
		files:        map[string][]byte{},
		once:         map[string]bool{},
		bills:        map[string]data.Bill{},
		catalog:      map[string]data.CatalogEntry{},
		policies:     map[int]data.SelectionPolicy{},
		explanations: map[string]data.ChangeExplanation{},
		generations:  map[string]data.GenerationRecord{},
		ingest:       map[string]data.IngestState{},
		log:          map[string]data.LogEntry{},
		statuses:     map[string]data.BillStatus{},
		versionXML:   map[string][]byte{},
		titles:       map[string][]byte{},
		batches:      map[string][]data.GenResult{},
	}
}
