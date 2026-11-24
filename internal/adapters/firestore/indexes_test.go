package firestore

import (
	"encoding/json"
	"slices"
	"testing"
)

type indexFile struct {
	Indexes []struct {
		CollectionGroup string `json:"collectionGroup"`
		Fields          []struct {
			FieldPath   string `json:"fieldPath"`
			ArrayConfig string `json:"arrayConfig"`
		} `json:"fields"`
	} `json:"indexes"`
	FieldOverrides []struct {
		CollectionGroup string `json:"collectionGroup"`
		FieldPath       string `json:"fieldPath"`
		Indexes         []any  `json:"indexes"`
	} `json:"fieldOverrides"`
}

func loadIndexes(t *testing.T) indexFile {
	t.Helper()
	var idx indexFile
	if err := json.Unmarshal(IndexesJSON(), &idx); err != nil {
		t.Fatal(err)
	}
	return idx
}

// TestIndexesExemptBody checks the embedded index file exempts the body field
// of every collection that has one.
func TestIndexesExemptBody(t *testing.T) {
	t.Parallel()
	idx := loadIndexes(t)
	exempt := map[string]bool{}
	for _, o := range idx.FieldOverrides {
		if o.FieldPath == fieldBody && len(o.Indexes) == 0 {
			exempt[o.CollectionGroup] = true
		}
	}
	for _, col := range Collections() {
		if col != colLogSeq && !exempt[col] { // the counter holds no body
			t.Errorf("collection %s: body is not exempt from indexing", col)
		}
	}
}

// TestIndexesCoverKeywordFilters checks there is a keyword index for each
// catalog filter field the query code uses.
func TestIndexesCoverKeywordFilters(t *testing.T) {
	t.Parallel()
	idx := loadIndexes(t)
	for _, field := range []string{"tracked", "chamber", "status", "policy_area", "markup_status"} {
		found := slices.ContainsFunc(idx.Indexes, func(ix struct {
			CollectionGroup string `json:"collectionGroup"`
			Fields          []struct {
				FieldPath   string `json:"fieldPath"`
				ArrayConfig string `json:"arrayConfig"`
			} `json:"fields"`
		},
		) bool {
			f := ix.Fields
			return ix.CollectionGroup == colCatalog && len(f) == 3 && f[0].FieldPath == field &&
				f[1].FieldPath == "search_terms" && f[1].ArrayConfig == "CONTAINS"
		})
		if !found {
			t.Errorf("no keyword index for filter %s", field)
		}
	}
}
