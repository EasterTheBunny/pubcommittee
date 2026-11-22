package memory

import (
	"context"
	"encoding/base64"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// defaultPageLimit is the ListChanged page size when a caller passes Limit 0.
const defaultPageLimit = 100

// Compile-time port checks for the source and generation ports.
var (
	_ data.BillSource = (*Store)(nil)
	_ data.LawSource  = (*Store)(nil)
	_ data.Generator  = (*Store)(nil)
)

// SeedBills replaces the seeded BillSource data with deep copies of the
// arguments. changed keeps the order given. statuses are keyed by their bill
// and xml by its version.
func (s *Store) SeedBills(changed []data.ChangedBill, statuses []data.BillStatus, xml map[data.VersionRef][]byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.changed = slices.Clone(changed)
	s.statuses = make(map[string]data.BillStatus, len(statuses))
	for _, st := range statuses {
		st.Versions = slices.Clone(st.Versions)
		s.statuses[st.Bill.String()] = st
	}
	s.versionXML = make(map[string][]byte, len(xml))
	for ref, b := range xml {
		s.versionXML[ref.String()] = slices.Clone(b)
	}
}

// SeedLaw replaces the seeded LawSource data with deep copies of the
// arguments. titles are stored under "<release point id>/<title>".
func (s *Store) SeedLaw(points []data.ReleasePoint, titles map[data.ReleasePoint]map[int][]byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.releasePoints = slices.Clone(points)
	s.titles = make(map[string][]byte)
	for rp, byTitle := range titles {
		for title, b := range byTitle {
			s.titles[fmt.Sprintf("%s/%d", rp.ID, title)] = slices.Clone(b)
		}
	}
}

// ListChanged pages through the seeded changed bills, keeping seeded order.
// With since set, only bills updated at or after it are listed.
func (s *Store) ListChanged(ctx context.Context, since string, p data.Page) (data.Paged[data.ChangedBill], error) {
	if err := ctx.Err(); err != nil {
		return data.Paged[data.ChangedBill]{}, fmt.Errorf("list changed: %w", err)
	}
	if p.Limit < 0 {
		return data.Paged[data.ChangedBill]{}, fmt.Errorf("list changed limit %d: %w", p.Limit, data.ErrInvalid)
	}
	start, err := decodeCursor(p.Cursor)
	if err != nil {
		return data.Paged[data.ChangedBill]{}, fmt.Errorf("list changed: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	items, err := changedSince(s.changed, since)
	if err != nil {
		return data.Paged[data.ChangedBill]{}, fmt.Errorf("list changed: %w", err)
	}
	if start > len(items) {
		return data.Paged[data.ChangedBill]{}, fmt.Errorf("list changed cursor %q: %w", p.Cursor, data.ErrInvalid)
	}
	limit := p.Limit
	if limit == 0 {
		limit = defaultPageLimit
	}
	end := min(start+limit, len(items))
	var page data.Paged[data.ChangedBill]
	page.Items = slices.Clone(items[start:end])
	if end < len(items) {
		page.Next = encodeCursor(end)
	}
	return page, nil
}

// FetchStatus returns the seeded status record of a bill.
func (s *Store) FetchStatus(ctx context.Context, bill data.BillRef) (data.BillStatus, error) {
	if err := ctx.Err(); err != nil {
		return data.BillStatus{}, fmt.Errorf("fetch status: %w", err)
	}
	if err := bill.Validate(); err != nil {
		return data.BillStatus{}, fmt.Errorf("fetch status: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	st, ok := s.statuses[bill.String()]
	if !ok {
		return data.BillStatus{}, fmt.Errorf("fetch status %s: %w", bill, data.ErrNotFound)
	}
	st.Versions = slices.Clone(st.Versions)
	return st, nil
}

// FetchVersionXML returns a copy of the seeded XML of one text version.
func (s *Store) FetchVersionXML(ctx context.Context, ref data.VersionRef) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("fetch version xml: %w", err)
	}
	if err := ref.Validate(); err != nil {
		return nil, fmt.Errorf("fetch version xml: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.versionXML[ref.String()]
	if !ok {
		return nil, fmt.Errorf("fetch version xml %s: %w", ref, data.ErrNotFound)
	}
	return slices.Clone(b), nil
}

// ListReleasePoints returns the seeded release points, in seeded order.
func (s *Store) ListReleasePoints(ctx context.Context) ([]data.ReleasePoint, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("list release points: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.releasePoints), nil
}

// FetchTitle returns a copy of the seeded XML of one title at a release point.
func (s *Store) FetchTitle(ctx context.Context, rp data.ReleasePoint, title int) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("fetch title: %w", err)
	}
	if title < 1 {
		return nil, fmt.Errorf("fetch title %d: %w", title, data.ErrInvalid)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.titles[fmt.Sprintf("%s/%d", rp.ID, title)]
	if !ok {
		return nil, fmt.Errorf("fetch title %s/%d: %w", rp.ID, title, data.ErrNotFound)
	}
	return slices.Clone(b), nil
}

// Submit runs every request at once and stores the results as a complete
// batch. Batch IDs count from 1.
func (s *Store) Submit(ctx context.Context, reqs []data.GenRequest) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("submit: %w", err)
	}
	if len(reqs) == 0 {
		return "", fmt.Errorf("submit: no requests: %w", data.ErrInvalid)
	}
	seen := make(map[string]bool, len(reqs))
	results := make([]data.GenResult, 0, len(reqs))
	for _, r := range reqs {
		if err := validateGenRequest(r); err != nil {
			return "", fmt.Errorf("submit: %w", err)
		}
		if seen[r.ID] {
			return "", fmt.Errorf("submit: duplicate request ID %q: %w", r.ID, data.ErrInvalid)
		}
		seen[r.ID] = true
		results = append(results, fakeResult(r))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.batchCount++
	id := fmt.Sprintf("batch-%d", s.batchCount)
	s.batches[id] = results
	return id, nil
}

// Collect returns the results of a batch. Memory batches complete at once, so
// done is always true.
func (s *Store) Collect(ctx context.Context, batchID string) ([]data.GenResult, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, fmt.Errorf("collect: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	results, ok := s.batches[batchID]
	if !ok {
		return nil, false, fmt.Errorf("collect batch %s: %w", batchID, data.ErrNotFound)
	}
	return slices.Clone(results), true, nil
}

// Generate runs one request synchronously with the deterministic fake model.
func (s *Store) Generate(ctx context.Context, req data.GenRequest) (data.GenResult, error) {
	if err := ctx.Err(); err != nil {
		return data.GenResult{}, fmt.Errorf("generate: %w", err)
	}
	if err := validateGenRequest(req); err != nil {
		return data.GenResult{}, fmt.Errorf("generate: %w", err)
	}
	return fakeResult(req), nil
}

// validateGenRequest reports ErrInvalid when a request lacks an ID or Task.
func validateGenRequest(r data.GenRequest) error {
	if r.ID == "" || r.Task == "" {
		return fmt.Errorf("request ID %q task %q: %w", r.ID, r.Task, data.ErrInvalid)
	}
	return nil
}

// fakeResult is the deterministic result of the memory generator.
func fakeResult(r data.GenRequest) data.GenResult {
	text := "summary: " + r.Input
	return data.GenResult{
		ID:        r.ID,
		Outcome:   "ok",
		Text:      text,
		Model:     "memory-fake",
		TokensIn:  len(r.Input),
		TokensOut: len(text),
	}
}

// changedSince returns the entries of list updated at or after since, in
// order. An empty since returns list unchanged.
func changedSince(list []data.ChangedBill, since string) ([]data.ChangedBill, error) {
	if since == "" {
		return list, nil
	}
	from, err := time.Parse(time.RFC3339, since)
	if err != nil {
		return nil, fmt.Errorf("since %q: %w", since, data.ErrInvalid)
	}
	var out []data.ChangedBill
	for _, c := range list {
		at, perr := time.Parse(time.RFC3339, c.UpdatedAt)
		if perr != nil {
			return nil, fmt.Errorf("changed bill %s updated at %q: %w", c.Bill, c.UpdatedAt, data.ErrInvalid)
		}
		if !at.Before(from) {
			out = append(out, c)
		}
	}
	return out, nil
}

// decodeCursor returns the index a cursor points at; the empty cursor is 0.
func decodeCursor(cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	raw, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return 0, fmt.Errorf("cursor %q: %w", cursor, data.ErrInvalid)
	}
	n, err := strconv.Atoi(string(raw))
	if err != nil || n < 0 {
		return 0, fmt.Errorf("cursor %q: %w", cursor, data.ErrInvalid)
	}
	return n, nil
}

// encodeCursor returns the opaque cursor for the item at index n.
func encodeCursor(n int) string {
	return base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(n)))
}
