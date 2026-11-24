package firestore

import (
	_ "embed" // the index definitions are embedded for tests and for M1-15 to read
)

//go:embed indexes/firestore.indexes.json
var indexesJSON []byte

// IndexesJSON returns the index definitions the adapter's queries need, in the
// Firebase firestore.indexes.json format: composite indexes for catalog filters
// with a keyword, and an exemption of the large body field from indexing in
// every collection. M1-15 turns them into infrastructure resources.
func IndexesJSON() []byte { return append([]byte(nil), indexesJSON...) }
