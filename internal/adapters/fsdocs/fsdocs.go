// Package fsdocs implements PublicDocs, Moderation, and PlatformDocs on a
// local data root, standing in for Firestore. Each document is one JSON file
// at <collection>/<id>.json for the M1 collections bills, catalog,
// selection_policy, explanations, generations, moderation_log, and
// ingest_state. It is for local runs and tests only and must never be
// imported by a gcp build.
//
// Writes go to a temporary file, fsync, and atomic rename, so readers in
// other processes never see a partial document. Every write takes one
// exclusive lock file (<root>/.lock) that excludes other goroutines and other
// processes. ApplyLogged writes the log entry first and then the change, and
// removes the log entry again when the change fails.
//
// The catalog has no index file: ListCatalog reads the directory listing
// (file names encode the bill key), sorts it numerically by bill key, and
// reads only the documents of the page. That stays fast at about 15,000
// entries (see TestCatalogScale).
//
// Not modelled (for M1-03b to verify against Firestore): composite index
// requirements and query limits, transaction size and contention limits,
// document size limits, security rules and IAM, server timestamps, and
// catalog filters and search_terms (the PublicDocs port has no filter or
// search operation yet).
package fsdocs

import (
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/adapters/fsutil"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

const (
	defaultLimit = 50
	ext          = ".json"
	// catalogIDParts is the segment count of a bill key (jurisdiction, session, type, number).
	catalogIDParts = 4

	colBills        = "bills"
	colCatalog      = "catalog"
	colPolicy       = "selection_policy"
	colExplanations = "explanations"
	colGenerations  = "generations"
	colLog          = "moderation_log"
	colIngest       = "ingest_state"
)

// Config configures a Store.
type Config struct {
	// Root is the data root; it is created when missing.
	Root string
}

// Store is the filesystem implementation of the document ports.
type Store struct {
	root string
	// failChange, when set by tests, is called after the log entry of
	// ApplyLogged is written and before the change is; an error aborts it.
	failChange func() error
}

var (
	_ data.PublicDocs   = (*Store)(nil)
	_ data.PlatformDocs = (*Store)(nil)
	_ data.Moderation   = (*Store)(nil)
)

// New validates cfg and returns a Store. It touches only the data root.
func New(cfg Config) (*Store, error) {
	root, err := fsutil.OpenRoot(cfg.Root)
	if err != nil {
		return nil, fmt.Errorf("fsdocs: %w", err)
	}
	return &Store{root: root}, nil
}

// fileID returns a reversible file-name form of id.
func fileID(id string) string {
	e := url.PathEscape(id)
	if strings.HasPrefix(e, ".") {
		e = "%2E" + e[1:]
	}
	return e
}

func unFileID(name string) (string, error) {
	s, err := url.PathUnescape(strings.TrimSuffix(name, ext))
	if err != nil {
		return "", fmt.Errorf("file name %q: %w", name, data.ErrUnavailable)
	}
	return s, nil
}

func (s *Store) docPath(col, id string) string {
	return filepath.Join(s.root, col, fileID(id)+ext)
}

// read decodes the document into v; a missing file is ErrNotFound.
func (s *Store) read(col, id string, v any) error {
	b, err := os.ReadFile(s.docPath(col, id))
	if err != nil {
		return mapErr(col+"/"+id, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s/%s: decode: %w: %w", col, id, data.ErrUnavailable, err)
	}
	return nil
}

// write replaces the document atomically. The caller holds the lock.
func (s *Store) write(col, id string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("%s/%s: encode: %w: %w", col, id, data.ErrInvalid, err)
	}
	return replace(s.docPath(col, id), b)
}

func (s *Store) lock() (func(), error) { return lock(filepath.Join(s.root, ".lock")) }

func mapErr(op string, err error) error { return fmt.Errorf("%w", fsutil.MapErr(op, err)) }

func replace(p string, b []byte) error {
	if err := fsutil.Replace(p, b); err != nil {
		return fmt.Errorf("replace: %w", err)
	}
	return nil
}

func lock(p string) (func(), error) {
	unlock, err := fsutil.Lock(p)
	if err != nil {
		return nil, fmt.Errorf("lock: %w", err)
	}
	return unlock, nil
}

// same reports whether a and b encode to the same JSON.
func same(a, b any) bool {
	x, err1 := json.Marshal(a)
	y, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && string(x) == string(y)
}

func ctxErr(ctx context.Context, op string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

// GetBill returns bill metadata; ErrNotFound when the bill is unknown.
func (s *Store) GetBill(ctx context.Context, bill data.BillRef) (data.Bill, error) {
	if err := ctxErr(ctx, "get bill"); err != nil {
		return data.Bill{}, err
	}
	var b data.Bill
	if err := s.read(colBills, bill.String(), &b); err != nil {
		return data.Bill{}, fmt.Errorf("get bill: %w", err)
	}
	return b, nil
}

// PutBill creates or replaces bill metadata.
func (s *Store) PutBill(ctx context.Context, b data.Bill) error {
	return s.put(ctx, "put bill", data.Change{Bill: &b})
}

// PutCatalogEntry creates or replaces a catalog entry.
func (s *Store) PutCatalogEntry(ctx context.Context, e data.CatalogEntry) error {
	return s.put(ctx, "put catalog entry", data.Change{Catalog: &e})
}

// PutExplanation creates or replaces an explanation.
func (s *Store) PutExplanation(ctx context.Context, e data.ChangeExplanation) error {
	return s.put(ctx, "put explanation", data.Change{Explanation: &e})
}

// PutSelectionPolicy writes a policy version; an existing Version is
// write-once and ErrConflict when its content differs.
func (s *Store) PutSelectionPolicy(ctx context.Context, p data.SelectionPolicy) error {
	return s.put(ctx, "put selection policy", data.Change{Policy: &p})
}

func (s *Store) put(ctx context.Context, op string, c data.Change) error {
	if err := ctxErr(ctx, op); err != nil {
		return err
	}
	if err := validateChange(c); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	unlock, err := s.lock()
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer unlock()
	if err := s.checkChange(c); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if err := s.applyChange(c); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

// checkChange returns ErrConflict when a write-once policy would change.
func (s *Store) checkChange(c data.Change) error {
	if c.Policy == nil {
		return nil
	}
	var old data.SelectionPolicy
	err := s.read(colPolicy, policyID(c.Policy.Version), &old)
	if errors.Is(err, data.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !same(old, *c.Policy) {
		return fmt.Errorf("selection policy version %d: %w", c.Policy.Version, data.ErrConflict)
	}
	return nil
}

func (s *Store) applyChange(c data.Change) error {
	switch {
	case c.Bill != nil:
		return s.write(colBills, c.Bill.Ref.String(), c.Bill)
	case c.Catalog != nil:
		return s.write(colCatalog, c.Catalog.Bill.String(), c.Catalog)
	case c.Policy != nil:
		return s.write(colPolicy, policyID(c.Policy.Version), c.Policy)
	default:
		return s.write(colExplanations, explanationID(*c.Explanation), c.Explanation)
	}
}

func validateChange(c data.Change) error {
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
		return c.Bill.Ref.Validate() //nolint:wrapcheck // typed ErrInvalid
	case c.Catalog != nil:
		return c.Catalog.Bill.Validate() //nolint:wrapcheck // typed ErrInvalid
	case c.Policy != nil:
		if c.Policy.Version < 1 {
			return fmt.Errorf("selection policy version %d: %w", c.Policy.Version, data.ErrInvalid)
		}
		return nil
	default:
		return c.Explanation.Version.Validate() //nolint:wrapcheck // typed ErrInvalid
	}
}

func policyID(v int) string { return fmt.Sprintf("%08d", v) }

// explanationID starts with the version key, so one version's explanations
// share a file-name prefix; the tail makes (section, provision) unique.
func explanationID(x data.ChangeExplanation) string {
	return x.Version.String() + "|" + strings.Join(x.Section, "/") + "|" + x.ProvisionID
}

// GetSelectionPolicy returns the policy with the highest Version.
func (s *Store) GetSelectionPolicy(ctx context.Context) (data.SelectionPolicy, error) {
	if err := ctxErr(ctx, "get selection policy"); err != nil {
		return data.SelectionPolicy{}, err
	}
	names, err := s.list(colPolicy)
	if err != nil {
		return data.SelectionPolicy{}, fmt.Errorf("get selection policy: %w", err)
	}
	if len(names) == 0 {
		return data.SelectionPolicy{}, fmt.Errorf("get selection policy: %w", data.ErrNotFound)
	}
	slices.Sort(names)
	var p data.SelectionPolicy
	if err := s.read(colPolicy, names[len(names)-1], &p); err != nil {
		return data.SelectionPolicy{}, fmt.Errorf("get selection policy: %w", err)
	}
	return p, nil
}

// list returns the decoded ids of a collection; a missing collection is empty.
func (s *Store) list(col string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(s.root, col))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, mapErr("list "+col, err)
	}
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ext) || strings.HasPrefix(n, fsutil.TempPrefix) {
			continue
		}
		id, err := unFileID(n)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// GetExplanations returns the explanations of one version ordered by section
// path, then provision ID.
func (s *Store) GetExplanations(ctx context.Context, ref data.VersionRef) ([]data.ChangeExplanation, error) {
	if err := ctxErr(ctx, "get explanations"); err != nil {
		return nil, err
	}
	if err := ref.Validate(); err != nil {
		return nil, fmt.Errorf("get explanations: %w", err)
	}
	ids, err := s.list(colExplanations)
	if err != nil {
		return nil, fmt.Errorf("get explanations: %w", err)
	}
	out := []data.ChangeExplanation{}
	for _, id := range ids {
		if !strings.HasPrefix(id, ref.String()+"|") {
			continue
		}
		var x data.ChangeExplanation
		if err := s.read(colExplanations, id, &x); err != nil {
			return nil, fmt.Errorf("get explanations: %w", err)
		}
		if x.Version == ref {
			out = append(out, x)
		}
	}
	slices.SortFunc(out, func(a, b data.ChangeExplanation) int {
		if c := strings.Compare(strings.Join(a.Section, "/"), strings.Join(b.Section, "/")); c != 0 {
			return c
		}
		return strings.Compare(a.ProvisionID, b.ProvisionID)
	})
	return out, nil
}

// PutGeneration records one generation; the ID is write-once.
func (s *Store) PutGeneration(ctx context.Context, g data.GenerationRecord) error {
	if err := ctxErr(ctx, "put generation"); err != nil {
		return err
	}
	if g.ID == "" {
		return fmt.Errorf("put generation: empty id: %w", data.ErrInvalid)
	}
	unlock, err := s.lock()
	if err != nil {
		return fmt.Errorf("put generation: %w", err)
	}
	defer unlock()
	var old data.GenerationRecord
	switch err := s.read(colGenerations, g.ID, &old); {
	case err == nil:
		if same(old, g) {
			return nil
		}
		return fmt.Errorf("generation %q: %w", g.ID, data.ErrConflict)
	case !errors.Is(err, data.ErrNotFound):
		return fmt.Errorf("put generation: %w", err)
	}
	if err := s.write(colGenerations, g.ID, g); err != nil {
		return fmt.Errorf("put generation: %w", err)
	}
	return nil
}

// GetIngestState returns the state of a source; ErrNotFound when none.
func (s *Store) GetIngestState(ctx context.Context, source string) (data.IngestState, error) {
	if err := ctxErr(ctx, "get ingest state"); err != nil {
		return data.IngestState{}, err
	}
	var st data.IngestState
	if err := s.read(colIngest, source, &st); err != nil {
		return data.IngestState{}, fmt.Errorf("get ingest state: %w", err)
	}
	return st, nil
}

// PutIngestState replaces the state of a source.
func (s *Store) PutIngestState(ctx context.Context, st data.IngestState) error {
	if err := ctxErr(ctx, "put ingest state"); err != nil {
		return err
	}
	if st.Source == "" {
		return fmt.Errorf("put ingest state: empty source: %w", data.ErrInvalid)
	}
	unlock, err := s.lock()
	if err != nil {
		return fmt.Errorf("put ingest state: %w", err)
	}
	defer unlock()
	if err := s.write(colIngest, st.Source, st); err != nil {
		return fmt.Errorf("put ingest state: %w", err)
	}
	return nil
}

// Record appends a log entry. The same ID with identical content is a no-op;
// with different content ErrConflict.
func (s *Store) Record(ctx context.Context, e data.LogEntry) error {
	if err := ctxErr(ctx, "record"); err != nil {
		return err
	}
	if e.ID == "" {
		return fmt.Errorf("record log entry: empty id: %w", data.ErrInvalid)
	}
	unlock, err := s.lock()
	if err != nil {
		return fmt.Errorf("record: %w", err)
	}
	defer unlock()
	exists, err := s.checkLog(e)
	if err != nil || exists {
		return err
	}
	if err := s.write(colLog, e.ID, e); err != nil {
		return fmt.Errorf("record: %w", err)
	}
	return nil
}

// checkLog reports whether e is already stored identically; a different
// stored entry with the same ID is ErrConflict.
func (s *Store) checkLog(e data.LogEntry) (bool, error) {
	var old data.LogEntry
	err := s.read(colLog, e.ID, &old)
	if errors.Is(err, data.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if same(old, e) {
		return true, nil
	}
	return false, fmt.Errorf("log entry %q: %w", e.ID, data.ErrConflict)
}

// ApplyLogged writes the log entry and then the change under the lock; when
// the change fails the entry is removed, so neither is applied alone.
// Replaying an identical entry is a no-op.
func (s *Store) ApplyLogged(ctx context.Context, c data.Change, e data.LogEntry) error {
	if err := ctxErr(ctx, "apply logged"); err != nil {
		return err
	}
	if err := validateChange(c); err != nil {
		return fmt.Errorf("apply logged: %w", err)
	}
	if e.ID == "" {
		return fmt.Errorf("apply logged: empty log id: %w", data.ErrInvalid)
	}
	unlock, err := s.lock()
	if err != nil {
		return fmt.Errorf("apply logged: %w", err)
	}
	defer unlock()
	exists, err := s.checkLog(e)
	if err != nil || exists {
		return err
	}
	if err := s.checkChange(c); err != nil {
		return fmt.Errorf("apply logged: %w", err)
	}
	if err := s.logThenChange(c, e); err != nil {
		return fmt.Errorf("apply logged: %w", err)
	}
	return nil
}

// logThenChange writes the entry, then the change, and removes the entry
// when the change fails. The caller holds the lock.
func (s *Store) logThenChange(c data.Change, e data.LogEntry) error {
	if err := s.write(colLog, e.ID, e); err != nil {
		return fmt.Errorf("write log entry: %w", err)
	}
	err := s.applyLoggedChange(c)
	if err == nil {
		return nil
	}
	if rerr := os.Remove(s.docPath(colLog, e.ID)); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
		err = errors.Join(err, fmt.Errorf("remove log entry %q: %w", e.ID, rerr))
	}
	return err
}

func (s *Store) applyLoggedChange(c data.Change) error {
	if s.failChange != nil {
		if err := s.failChange(); err != nil {
			return err
		}
	}
	return s.applyChange(c)
}

// catalogRef parses a catalog file id back to a BillRef.
func catalogRef(id string) (data.BillRef, error) {
	parts := strings.Split(id, "/")
	if len(parts) != catalogIDParts {
		return data.BillRef{}, fmt.Errorf("catalog id %q: %w", id, data.ErrUnavailable)
	}
	n, err := strconv.Atoi(parts[3])
	if err != nil {
		return data.BillRef{}, fmt.Errorf("catalog id %q: %w", id, data.ErrUnavailable)
	}
	return data.BillRef{Jurisdiction: parts[0], Session: parts[1], Type: parts[2], Number: n}, nil
}

func billCmp(a, b data.BillRef) int {
	return cmp.Or(
		strings.Compare(a.Jurisdiction, b.Jurisdiction),
		strings.Compare(a.Session, b.Session),
		strings.Compare(a.Type, b.Type),
		cmp.Compare(a.Number, b.Number),
	)
}

func limitOf(p data.Page) (int, error) {
	switch {
	case p.Limit < 0:
		return 0, fmt.Errorf("page limit %d: %w", p.Limit, data.ErrInvalid)
	case p.Limit == 0:
		return defaultLimit, nil
	default:
		return p.Limit, nil
	}
}

func encodeCursor(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("encode cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func decodeCursor(cursor string, v any) error {
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return fmt.Errorf("cursor %q: %w", cursor, data.ErrInvalid)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("cursor %q: %w", cursor, data.ErrInvalid)
	}
	return nil
}

// ListCatalog pages through catalog entries in bill key order.
func (s *Store) ListCatalog(ctx context.Context, p data.Page) (data.Paged[data.CatalogEntry], error) {
	empty := data.Paged[data.CatalogEntry]{}
	if err := ctxErr(ctx, "list catalog"); err != nil {
		return empty, err
	}
	limit, err := limitOf(p)
	if err != nil {
		return empty, err
	}
	refs, err := s.catalogRefsAfter(p.Cursor)
	if err != nil {
		return empty, fmt.Errorf("list catalog: %w", err)
	}
	out := data.Paged[data.CatalogEntry]{Items: []data.CatalogEntry{}}
	if len(refs) > limit {
		if out.Next, err = encodeCursor(refs[limit-1]); err != nil {
			return empty, fmt.Errorf("list catalog: %w", err)
		}
		refs = refs[:limit]
	}
	for _, ref := range refs {
		var e data.CatalogEntry
		if err := s.read(colCatalog, ref.String(), &e); err != nil {
			return empty, fmt.Errorf("list catalog: %w", err)
		}
		out.Items = append(out.Items, e)
	}
	return out, nil
}

// catalogRefsAfter returns the sorted bill keys that follow the cursor.
func (s *Store) catalogRefsAfter(cursor string) ([]data.BillRef, error) {
	var after *data.BillRef
	if cursor != "" {
		var ref data.BillRef
		if err := decodeCursor(cursor, &ref); err != nil {
			return nil, err
		}
		after = &ref
	}
	ids, err := s.list(colCatalog)
	if err != nil {
		return nil, err
	}
	refs := make([]data.BillRef, 0, len(ids))
	for _, id := range ids {
		ref, err := catalogRef(id)
		if err != nil {
			return nil, err
		}
		if after == nil || billCmp(*after, ref) < 0 {
			refs = append(refs, ref)
		}
	}
	slices.SortFunc(refs, billCmp)
	return refs, nil
}

type logCursor struct {
	At time.Time `json:"at"`
	ID string    `json:"id"`
}

// logBefore reports whether a sorts before b: newer first, then ID descending.
func logBefore(a, b data.LogEntry) bool {
	if !a.At.Equal(b.At) {
		return a.At.After(b.At)
	}
	return a.ID > b.ID
}

// ListLog pages through the public log, newest first.
func (s *Store) ListLog(ctx context.Context, p data.Page) (data.Paged[data.LogEntry], error) {
	empty := data.Paged[data.LogEntry]{}
	if err := ctxErr(ctx, "list log"); err != nil {
		return empty, err
	}
	limit, err := limitOf(p)
	if err != nil {
		return empty, err
	}
	items, err := s.logAfter(p.Cursor)
	if err != nil {
		return empty, fmt.Errorf("list log: %w", err)
	}
	out := data.Paged[data.LogEntry]{Items: []data.LogEntry{}}
	if len(items) > limit {
		last := items[limit-1]
		if out.Next, err = encodeCursor(logCursor{At: last.At, ID: last.ID}); err != nil {
			return empty, fmt.Errorf("list log: %w", err)
		}
		items = items[:limit]
	}
	out.Items = append(out.Items, items...)
	return out, nil
}

// logAfter returns the log entries that follow the cursor, newest first.
func (s *Store) logAfter(cursor string) ([]data.LogEntry, error) {
	var after *data.LogEntry
	if cursor != "" {
		var lc logCursor
		if err := decodeCursor(cursor, &lc); err != nil {
			return nil, err
		}
		after = &data.LogEntry{At: lc.At, ID: lc.ID}
	}
	ids, err := s.list(colLog)
	if err != nil {
		return nil, err
	}
	items := make([]data.LogEntry, 0, len(ids))
	for _, id := range ids {
		var e data.LogEntry
		if err := s.read(colLog, id, &e); err != nil {
			return nil, err
		}
		if after == nil || logBefore(*after, e) {
			items = append(items, e)
		}
	}
	slices.SortFunc(items, logCmp)
	return items, nil
}

func logCmp(a, b data.LogEntry) int {
	switch {
	case logBefore(a, b):
		return -1
	case logBefore(b, a):
		return 1
	default:
		return 0
	}
}
