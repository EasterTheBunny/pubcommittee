package memory

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// docDefaultLimit is the page size used when Page.Limit is 0.
const docDefaultLimit = 50

// docLogCursor is the decoded form of a ListLog cursor: the sort key of the
// last entry returned.
type docLogCursor struct {
	At time.Time `json:"at"`
	ID string    `json:"id"`
}

var (
	_ data.PublicDocs   = (*Store)(nil)
	_ data.PlatformDocs = (*Store)(nil)
	_ data.Moderation   = (*Store)(nil)
)

// GetBill returns bill metadata; ErrNotFound when the bill is unknown.
func (s *Store) GetBill(ctx context.Context, bill data.BillRef) (data.Bill, error) {
	if err := docCheck(ctx, "get bill"); err != nil {
		return data.Bill{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.bills[bill.String()]
	if !ok {
		return data.Bill{}, fmt.Errorf("get bill %s: %w", bill.String(), data.ErrNotFound)
	}
	return docCloneBill(b), nil
}

// PutBill creates or replaces bill metadata; ErrInvalid for a bad BillRef.
func (s *Store) PutBill(ctx context.Context, b data.Bill) error {
	if err := docCheck(ctx, "put bill"); err != nil {
		return err
	}
	if err := b.Ref.Validate(); err != nil {
		return fmt.Errorf("put bill: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.applyChangeLocked(data.Change{Bill: &b})
	return nil
}

// ListCatalog pages through catalog entries in bill key order.
func (s *Store) ListCatalog(ctx context.Context, p data.Page) (data.Paged[data.CatalogEntry], error) {
	if err := docCheck(ctx, "list catalog"); err != nil {
		return data.Paged[data.CatalogEntry]{}, err
	}
	limit, err := docLimit(p)
	if err != nil {
		return data.Paged[data.CatalogEntry]{}, err
	}
	var after *data.BillRef
	if p.Cursor != "" {
		var ref data.BillRef
		if err := docDecodeCursor(p.Cursor, &ref); err != nil {
			return data.Paged[data.CatalogEntry]{}, fmt.Errorf("list catalog: %w", err)
		}
		after = &ref
	}
	items := s.catalogAfter(after)
	return docPage(items, limit, func(e data.CatalogEntry) (string, error) {
		return docEncodeCursor(e.Bill)
	})
}

// catalogAfter returns copies of the catalog entries whose bill key sorts
// after the cursor, in bill key order.
func (s *Store) catalogAfter(after *data.BillRef) []data.CatalogEntry {
	s.mu.RLock()
	items := make([]data.CatalogEntry, 0, len(s.catalog))
	for _, e := range s.catalog {
		if after != nil && !docBillLess(*after, e.Bill) {
			continue
		}
		items = append(items, docCloneCatalog(e))
	}
	s.mu.RUnlock()
	slices.SortFunc(items, func(a, b data.CatalogEntry) int {
		switch {
		case docBillLess(a.Bill, b.Bill):
			return -1
		case docBillLess(b.Bill, a.Bill):
			return 1
		default:
			return 0
		}
	})
	return items
}

// PutCatalogEntry creates or replaces a catalog entry by bill.
func (s *Store) PutCatalogEntry(ctx context.Context, e data.CatalogEntry) error {
	if err := docCheck(ctx, "put catalog entry"); err != nil {
		return err
	}
	if err := e.Bill.Validate(); err != nil {
		return fmt.Errorf("put catalog entry: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.applyChangeLocked(data.Change{Catalog: &e})
	return nil
}

// GetSelectionPolicy returns the policy version with the highest Version;
// ErrNotFound when no policy is stored.
func (s *Store) GetSelectionPolicy(ctx context.Context) (data.SelectionPolicy, error) {
	if err := docCheck(ctx, "get selection policy"); err != nil {
		return data.SelectionPolicy{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest data.SelectionPolicy
	found := false
	for _, p := range s.policies {
		if !found || p.Version > latest.Version {
			latest = p
			found = true
		}
	}
	if !found {
		return data.SelectionPolicy{}, fmt.Errorf("get selection policy: %w", data.ErrNotFound)
	}
	return docClonePolicy(latest), nil
}

// GetSelectionPolicyVersion returns one policy version; ErrNotFound when it
// does not exist.
func (s *Store) GetSelectionPolicyVersion(ctx context.Context, version int) (data.SelectionPolicy, error) {
	if err := docCheck(ctx, "get selection policy version"); err != nil {
		return data.SelectionPolicy{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.policies[version]
	if !ok {
		return data.SelectionPolicy{}, fmt.Errorf("get selection policy version %d: %w", version, data.ErrNotFound)
	}
	return docClonePolicy(p), nil
}

// PutSelectionPolicy writes a policy version; an existing Version is
// write-once and ErrConflict when its content differs.
func (s *Store) PutSelectionPolicy(ctx context.Context, p data.SelectionPolicy) error {
	if err := docCheck(ctx, "put selection policy"); err != nil {
		return err
	}
	if p.Version < 1 {
		return fmt.Errorf("selection policy version %d: %w", p.Version, data.ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkPolicyLocked(p); err != nil {
		return err
	}
	s.applyChangeLocked(data.Change{Policy: &p})
	return nil
}

// GetExplanations returns the explanations of one version ordered by section
// path, then provision ID. A version with none returns an empty list.
func (s *Store) GetExplanations(ctx context.Context, ref data.VersionRef) ([]data.ChangeExplanation, error) {
	if err := docCheck(ctx, "get explanations"); err != nil {
		return nil, err
	}
	if err := ref.Validate(); err != nil {
		return nil, fmt.Errorf("get explanations: %w", err)
	}
	s.mu.RLock()
	out := make([]data.ChangeExplanation, 0)
	for _, x := range s.explanations {
		if x.Version == ref {
			out = append(out, docCloneExplanation(x))
		}
	}
	s.mu.RUnlock()
	slices.SortFunc(out, func(a, b data.ChangeExplanation) int {
		if c := strings.Compare(strings.Join(a.Section, "/"), strings.Join(b.Section, "/")); c != 0 {
			return c
		}
		return strings.Compare(a.ProvisionID, b.ProvisionID)
	})
	return out, nil
}

// PutExplanation creates or replaces an explanation keyed by version,
// section, and provision ID.
func (s *Store) PutExplanation(ctx context.Context, e data.ChangeExplanation) error {
	if err := docCheck(ctx, "put explanation"); err != nil {
		return err
	}
	if err := e.Version.Validate(); err != nil {
		return fmt.Errorf("put explanation: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.applyChangeLocked(data.Change{Explanation: &e})
	return nil
}

// PutGeneration records one generation; the ID is write-once and ErrConflict
// when an existing record differs.
func (s *Store) PutGeneration(ctx context.Context, g data.GenerationRecord) error {
	if err := docCheck(ctx, "put generation"); err != nil {
		return err
	}
	if g.ID == "" {
		return fmt.Errorf("put generation: empty id: %w", data.ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.generations[g.ID]; ok {
		if reflect.DeepEqual(old, g) {
			return nil
		}
		return fmt.Errorf("generation %q: %w", g.ID, data.ErrConflict)
	}
	s.generations[g.ID] = g
	return nil
}

// GetIngestState returns the state of a source; ErrNotFound when none.
func (s *Store) GetIngestState(ctx context.Context, source string) (data.IngestState, error) {
	if err := docCheck(ctx, "get ingest state"); err != nil {
		return data.IngestState{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	st, ok := s.ingest[source]
	if !ok {
		return data.IngestState{}, fmt.Errorf("ingest state %q: %w", source, data.ErrNotFound)
	}
	return st, nil
}

// PutIngestState replaces the state of a source; ErrInvalid for an empty
// source.
func (s *Store) PutIngestState(ctx context.Context, st data.IngestState) error {
	if err := docCheck(ctx, "put ingest state"); err != nil {
		return err
	}
	if st.Source == "" {
		return fmt.Errorf("put ingest state: empty source: %w", data.ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ingest[st.Source] = st
	return nil
}

// ListLog pages through the public log, newest first by At then ID.
func (s *Store) ListLog(ctx context.Context, p data.Page) (data.Paged[data.LogEntry], error) {
	if err := docCheck(ctx, "list log"); err != nil {
		return data.Paged[data.LogEntry]{}, err
	}
	limit, err := docLimit(p)
	if err != nil {
		return data.Paged[data.LogEntry]{}, err
	}
	var after *data.LogEntry
	if p.Cursor != "" {
		var c docLogCursor
		if err := docDecodeCursor(p.Cursor, &c); err != nil {
			return data.Paged[data.LogEntry]{}, fmt.Errorf("list log: %w", err)
		}
		after = &data.LogEntry{At: c.At, ID: c.ID}
	}
	items := s.logAfter(after)
	return docPage(items, limit, func(e data.LogEntry) (string, error) {
		return docEncodeCursor(docLogCursor{At: e.At, ID: e.ID})
	})
}

// logAfter returns copies of the log entries that sort after the cursor
// entry, newest first.
func (s *Store) logAfter(after *data.LogEntry) []data.LogEntry {
	s.mu.RLock()
	items := make([]data.LogEntry, 0, len(s.log))
	for _, e := range s.log {
		if after != nil && !docLogBefore(*after, e) {
			continue
		}
		items = append(items, e)
	}
	s.mu.RUnlock()
	slices.SortFunc(items, func(a, b data.LogEntry) int {
		switch {
		case docLogBefore(a, b):
			return -1
		case docLogBefore(b, a):
			return 1
		default:
			return 0
		}
	})
	return items
}

// Record appends a log entry. The same ID with identical content is a no-op;
// the same ID with different content is ErrConflict.
func (s *Store) Record(ctx context.Context, e data.LogEntry) error {
	if err := docCheck(ctx, "record"); err != nil {
		return err
	}
	if e.ID == "" {
		return fmt.Errorf("record log entry: empty id: %w", data.ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	same, err := s.checkLogLocked(e)
	if err != nil || same {
		return err
	}
	s.log[e.ID] = e
	return nil
}

// ApplyLogged validates the change and the entry, then writes both or
// neither. Replaying an identical entry is a no-op.
func (s *Store) ApplyLogged(ctx context.Context, c data.Change, e data.LogEntry) error {
	if err := docCheck(ctx, "apply logged"); err != nil {
		return err
	}
	if err := docValidateChange(c); err != nil {
		return fmt.Errorf("apply logged: %w", err)
	}
	if e.ID == "" {
		return fmt.Errorf("apply logged: empty log id: %w", data.ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	same, err := s.checkLogLocked(e)
	if err != nil || same {
		return err
	}
	if c.Policy != nil {
		if err := s.checkPolicyLocked(*c.Policy); err != nil {
			return err
		}
	}
	s.applyChangeLocked(c)
	s.log[e.ID] = e
	return nil
}

// checkLogLocked reports whether e is already recorded with identical
// content. It returns ErrConflict when the ID holds different content.
// The caller holds s.mu.
func (s *Store) checkLogLocked(e data.LogEntry) (bool, error) {
	old, ok := s.log[e.ID]
	if !ok {
		return false, nil
	}
	if reflect.DeepEqual(old, e) {
		return true, nil
	}
	return false, fmt.Errorf("log entry %q: %w", e.ID, data.ErrConflict)
}

// checkPolicyLocked returns ErrConflict when a policy with the same Version
// holds different content. The caller holds s.mu.
func (s *Store) checkPolicyLocked(p data.SelectionPolicy) error {
	old, ok := s.policies[p.Version]
	if ok && !reflect.DeepEqual(old, p) {
		return fmt.Errorf("selection policy version %d: %w", p.Version, data.ErrConflict)
	}
	return nil
}

// applyChangeLocked writes a change that has been validated and checked for
// conflicts. The caller holds s.mu.
func (s *Store) applyChangeLocked(c data.Change) {
	switch {
	case c.Bill != nil:
		s.bills[c.Bill.Ref.String()] = docCloneBill(*c.Bill)
	case c.Catalog != nil:
		s.catalog[c.Catalog.Bill.String()] = docCloneCatalog(*c.Catalog)
	case c.Policy != nil:
		s.policies[c.Policy.Version] = docClonePolicy(*c.Policy)
	default:
		x := *c.Explanation
		s.explanations[docExplanationKey(x)] = docCloneExplanation(x)
	}
}

// docValidateChange reports ErrInvalid unless exactly one field is set and
// that field is valid.
func docValidateChange(c data.Change) error {
	n := 0
	for _, set := range []bool{c.Bill != nil, c.Catalog != nil, c.Policy != nil, c.Explanation != nil} {
		if set {
			n++
		}
	}
	if n != 1 {
		return fmt.Errorf("change sets %d fields: %w", n, data.ErrInvalid)
	}
	switch {
	case c.Bill != nil:
		return docWrapInvalid(c.Bill.Ref.Validate())
	case c.Catalog != nil:
		return docWrapInvalid(c.Catalog.Bill.Validate())
	case c.Policy != nil:
		if c.Policy.Version < 1 {
			return fmt.Errorf("selection policy version %d: %w", c.Policy.Version, data.ErrInvalid)
		}
		return nil
	default:
		return docWrapInvalid(c.Explanation.Version.Validate())
	}
}

// docWrapInvalid returns err wrapped with a message, or nil when err is nil.
func docWrapInvalid(err error) error {
	if err != nil {
		return fmt.Errorf("change: %w", err)
	}
	return nil
}

// docExplanationKey identifies an explanation by version, section path, and
// provision ID.
func docExplanationKey(x data.ChangeExplanation) string {
	return x.Version.String() + "\x00" + strings.Join(x.Section, "/") + "\x00" + x.ProvisionID
}

// docBillLess orders bill keys by jurisdiction, session, type, and number.
func docBillLess(a, b data.BillRef) bool {
	if a.Jurisdiction != b.Jurisdiction {
		return a.Jurisdiction < b.Jurisdiction
	}
	if a.Session != b.Session {
		return a.Session < b.Session
	}
	if a.Type != b.Type {
		return a.Type < b.Type
	}
	return a.Number < b.Number
}

// docLogBefore reports whether log entry a sorts before b: newer first by At,
// then by ID descending.
func docLogBefore(a, b data.LogEntry) bool {
	if !a.At.Equal(b.At) {
		return a.At.After(b.At)
	}
	return a.ID > b.ID
}

// docCheck returns the context error, wrapped, when ctx is done.
func docCheck(ctx context.Context, op string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

// docLimit returns the page size for p, rejecting negative limits.
func docLimit(p data.Page) (int, error) {
	switch {
	case p.Limit < 0:
		return 0, fmt.Errorf("page limit %d: %w", p.Limit, data.ErrInvalid)
	case p.Limit == 0:
		return docDefaultLimit, nil
	default:
		return p.Limit, nil
	}
}

// docPage cuts sorted items to one page. Next is the cursor of the last
// item on the page when more items follow, and empty otherwise.
func docPage[T any](items []T, limit int, cursorOf func(T) (string, error)) (data.Paged[T], error) {
	out := data.Paged[T]{Items: []T{}}
	if len(items) > limit {
		next, err := cursorOf(items[limit-1])
		if err != nil {
			return data.Paged[T]{}, err
		}
		out.Next = next
		items = items[:limit]
	}
	out.Items = append(out.Items, items...)
	return out, nil
}

// docEncodeCursor returns an opaque cursor for v.
func docEncodeCursor(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("encode cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// docDecodeCursor decodes a cursor made by docEncodeCursor into v. A cursor
// that does not decode is ErrInvalid.
func docDecodeCursor(cursor string, v any) error {
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return fmt.Errorf("cursor %q: %w", cursor, data.ErrInvalid)
	}
	if err = json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("cursor %q: %w", cursor, data.ErrInvalid)
	}
	return nil
}

// docCopy returns a copy of in that keeps nil as nil.
func docCopy[T any](in []T) []T {
	if in == nil {
		return nil
	}
	out := make([]T, len(in))
	copy(out, in)
	return out
}

func docCloneBill(b data.Bill) data.Bill {
	b.Versions = docCopy(b.Versions)
	b.OfficialLinks = docCopy(b.OfficialLinks)
	b.TrackedReasons = docCopy(b.TrackedReasons)
	return b
}

func docCloneCatalog(e data.CatalogEntry) data.CatalogEntry {
	e.Reasons = docCopy(e.Reasons)
	e.Subjects = docCopy(e.Subjects)
	e.OfficialLinks = docCopy(e.OfficialLinks)
	return e
}

func docClonePolicy(p data.SelectionPolicy) data.SelectionPolicy {
	p.Rules = docCopy(p.Rules)
	return p
}

func docCloneExplanation(x data.ChangeExplanation) data.ChangeExplanation {
	x.Section = docCopy(x.Section)
	x.Flags = docCopy(x.Flags)
	return x
}
