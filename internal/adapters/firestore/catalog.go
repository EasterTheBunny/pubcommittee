package firestore

import (
	"context"
	"fmt"
	"slices"
	"strings"

	fs "cloud.google.com/go/firestore"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// CatalogQuery filters the catalog. Zero fields do not filter. Keyword words
// are matched against search_terms: every word must match a term or a word of
// a term, case-insensitively. Page order is always bill key order.
type CatalogQuery struct {
	Tracked      *bool
	Chamber      string
	Status       string
	PolicyArea   string
	MarkupStatus string
	Keyword      string
}

// searchTerms returns the stored form of a catalog entry's search terms: each
// term and each word of it, lowercase, without duplicates.
func searchTerms(terms []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, t := range terms {
		t = strings.ToLower(strings.TrimSpace(t))
		for _, w := range append([]string{t}, strings.Fields(t)...) {
			if w != "" && !seen[w] {
				seen[w] = true
				out = append(out, w)
			}
		}
	}
	slices.Sort(out)
	return out
}

// catalogFields are the query fields stored beside the body of an entry.
func catalogFields(e data.CatalogEntry) map[string]any {
	return map[string]any{
		"tracked":       e.Tracked,
		"chamber":       e.Chamber,
		"status":        e.Status,
		"policy_area":   e.PolicyArea,
		"markup_status": e.MarkupStatus,
		"search_terms":  searchTerms(e.SearchTerms),
	}
}

// ListCatalog pages through catalog entries in bill key order.
func (s *Store) ListCatalog(ctx context.Context, p data.Page) (data.Paged[data.CatalogEntry], error) {
	return s.QueryCatalog(ctx, CatalogQuery{}, p)
}

// QueryCatalog pages through the catalog entries that match q, in bill key
// order. The cursor is the id of the last entry of the previous page, so pages
// stay stable while entries are added or changed. It is not yet a port
// operation: PublicDocs has no filter or search method.
func (s *Store) QueryCatalog(
	ctx context.Context, q CatalogQuery, p data.Page,
) (data.Paged[data.CatalogEntry], error) {
	empty := data.Paged[data.CatalogEntry]{}
	limit, err := limitOf(p)
	if err != nil {
		return empty, err
	}
	var after string
	if p.Cursor != "" {
		if err := decodeCursor(p.Cursor, &after); err != nil || after == "" {
			return empty, fmt.Errorf("list catalog: cursor %q: %w", p.Cursor, data.ErrInvalid)
		}
	}
	base, rest := s.catalogQuery(q)
	items, ids, err := s.catalogScan(ctx, base, rest, after, limit+1)
	if err != nil {
		return empty, err
	}
	out := data.Paged[data.CatalogEntry]{Items: items}
	if len(items) > limit {
		if out.Next, err = encodeCursor(ids[limit-1]); err != nil {
			return empty, fmt.Errorf("list catalog: %w", err)
		}
		out.Items = items[:limit]
	}
	return out, nil
}

// catalogQuery builds the server side of q and returns the keyword words the
// server cannot match, because a query allows one array-contains.
func (s *Store) catalogQuery(q CatalogQuery) (fs.Query, []string) {
	query := s.client.Collection(colCatalog).Query
	if q.Tracked != nil {
		query = query.Where("tracked", "==", *q.Tracked)
	}
	for field, v := range map[string]string{
		"chamber": q.Chamber, "status": q.Status, "policy_area": q.PolicyArea, "markup_status": q.MarkupStatus,
	} {
		if v != "" {
			query = query.Where(field, "==", v)
		}
	}
	words := strings.Fields(strings.ToLower(q.Keyword))
	var rest []string
	if len(words) > 0 {
		// The longest word is the most selective.
		slices.SortFunc(words, func(a, b string) int { return len(b) - len(a) })
		query = query.Where("search_terms", "array-contains", words[0])
		rest = words[1:]
	}
	return query.OrderBy(fs.DocumentID, fs.Asc), rest
}

// catalogScan reads entries after the id until want entries match rest.
func (s *Store) catalogScan(
	ctx context.Context, base fs.Query, rest []string, after string, want int,
) ([]data.CatalogEntry, []string, error) {
	items := []data.CatalogEntry{}
	ids := []string{}
	for len(items) < want {
		q := base.Limit(want)
		if after != "" {
			q = q.StartAfter(after)
		}
		snaps, err := q.Documents(ctx).GetAll()
		if err != nil {
			return nil, nil, mapErr("list catalog", err)
		}
		for _, snap := range snaps {
			after = snap.Ref.ID
			var e data.CatalogEntry
			if err := decode(snap, &e); err != nil {
				return nil, nil, fmt.Errorf("list catalog: %w", err)
			}
			if len(items) < want && hasWords(e, rest) {
				items = append(items, e)
				ids = append(ids, snap.Ref.ID)
			}
		}
		if len(snaps) < want {
			break
		}
	}
	return items, ids, nil
}

func hasWords(e data.CatalogEntry, words []string) bool {
	if len(words) == 0 {
		return true
	}
	have := searchTerms(e.SearchTerms)
	for _, w := range words {
		if !slices.Contains(have, w) {
			return false
		}
	}
	return true
}
