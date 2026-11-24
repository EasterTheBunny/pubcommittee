package firestore

import (
	"context"
	"fmt"
	"strings"

	fs "cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

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
	if err := validateChange(c); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	ref, fields, err := s.changeDoc(c)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if c.Policy == nil {
		if _, err := ref.Set(ctx, fields); err != nil {
			return mapErr(op, err)
		}
		return nil
	}
	err = s.client.RunTransaction(ctx, func(_ context.Context, tx *fs.Transaction) error {
		if err := checkPolicy(tx, ref, c.Policy); err != nil {
			return err
		}
		return tx.Set(ref, fields)
	})
	if err != nil {
		return mapErr(op, err)
	}
	return nil
}

// checkPolicy returns ErrConflict when a write-once policy would change. It
// reads inside tx, so it must run before the transaction's first write.
func checkPolicy(tx *fs.Transaction, ref *fs.DocumentRef, p *data.SelectionPolicy) error {
	snap, err := tx.Get(ref)
	if status.Code(err) == codes.NotFound {
		return nil
	}
	if err != nil {
		return mapErr("read selection policy", err)
	}
	var old data.SelectionPolicy
	if err := decode(snap, &old); err != nil {
		return err
	}
	if !same(old, *p) {
		return fmt.Errorf("selection policy version %d: %w", p.Version, data.ErrConflict)
	}
	return nil
}

// changeDoc returns the document a change writes and its stored fields.
func (s *Store) changeDoc(c data.Change) (*fs.DocumentRef, map[string]any, error) {
	var (
		col, id string
		v       any
		extra   map[string]any
	)
	switch {
	case c.Bill != nil:
		col, id, v = colBills, billID(c.Bill.Ref), c.Bill
	case c.Catalog != nil:
		col, id, v, extra = colCatalog, billID(c.Catalog.Bill), c.Catalog, catalogFields(*c.Catalog)
	case c.Policy != nil:
		col, id, v, extra = colPolicy, policyID(c.Policy.Version), c.Policy, map[string]any{"version": c.Policy.Version}
	default:
		x := c.Explanation
		col, id, v, extra = colExplanations, explanationID(*x), x, map[string]any{"version_key": x.Version.String()}
	}
	fields, err := encode(v, extra)
	if err != nil {
		return nil, nil, err
	}
	return s.doc(col, id), fields, nil
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

// PutGeneration records one generation; the ID is write-once.
func (s *Store) PutGeneration(ctx context.Context, g data.GenerationRecord) error {
	if g.ID == "" {
		return fmt.Errorf("put generation: empty id: %w", data.ErrInvalid)
	}
	fields, err := encode(g, nil)
	if err != nil {
		return fmt.Errorf("put generation: %w", err)
	}
	ref := s.doc(colGenerations, docID(g.ID))
	err = s.client.RunTransaction(ctx, func(_ context.Context, tx *fs.Transaction) error {
		snap, err := tx.Get(ref)
		switch {
		case status.Code(err) == codes.NotFound:
			return tx.Set(ref, fields)
		case err != nil:
			return err //nolint:wrapcheck // mapped by the caller of RunTransaction
		}
		var old data.GenerationRecord
		if err := decode(snap, &old); err != nil {
			return err
		}
		if !same(old, g) {
			return fmt.Errorf("generation %q: %w", g.ID, data.ErrConflict)
		}
		return nil
	})
	if err != nil {
		return mapErr("put generation", err)
	}
	return nil
}

// PutIngestState replaces the state of a source.
func (s *Store) PutIngestState(ctx context.Context, st data.IngestState) error {
	if st.Source == "" {
		return fmt.Errorf("put ingest state: empty source: %w", data.ErrInvalid)
	}
	fields, err := encode(st, nil)
	if err != nil {
		return fmt.Errorf("put ingest state: %w", err)
	}
	if _, err := s.doc(colIngest, docID(st.Source)).Set(ctx, fields); err != nil {
		return mapErr("put ingest state", err)
	}
	return nil
}

// logDay is the sortable day of a coarsened entry.
const logDayLayout = "2006-01-02"

// logOrderKey sorts entries by day, then by write order.
func logOrderKey(e data.LogEntry) string {
	return fmt.Sprintf("%s|%019d", e.At.UTC().Format(logDayLayout), e.Seq)
}

// logState is what a transaction learns about a log entry before writing.
type logState struct {
	ref    *fs.DocumentRef
	exists bool
	next   int64
}

// readLog reads the log document of e and the sequence counter, for use as
// the first reads of a transaction. A stored entry that differs from e is
// ErrConflict.
func (s *Store) readLog(tx *fs.Transaction, e data.LogEntry) (logState, error) {
	st := logState{ref: s.doc(colLog, docID(e.ID))}
	snap, err := tx.Get(st.ref)
	switch {
	case err == nil:
		var old data.LogEntry
		if err := decode(snap, &old); err != nil {
			return st, err
		}
		if !same(data.CoarseLogEntry(old), data.CoarseLogEntry(e)) {
			return st, fmt.Errorf("log entry %q: %w", e.ID, data.ErrConflict)
		}
		st.exists = true
		return st, nil
	case status.Code(err) != codes.NotFound:
		return st, mapErr("read log entry", err)
	}
	cur, err := tx.Get(s.doc(colLogSeq, logSeqDoc))
	switch {
	case status.Code(err) == codes.NotFound:
		st.next = 1
	case err != nil:
		return st, mapErr("read log counter", err)
	default:
		var c struct {
			Last int64 `firestore:"last"`
		}
		if err := cur.DataTo(&c); err != nil {
			return st, fmt.Errorf("log counter: %w: %w", data.ErrUnavailable, err)
		}
		st.next = c.Last + 1
	}
	return st, nil
}

// writeLog queues the counter and the entry of a state readLog returned.
func (s *Store) writeLog(tx *fs.Transaction, st logState, e data.LogEntry) error {
	e = data.CoarseLogEntry(e)
	e.Seq = st.next
	fields, err := encode(e, map[string]any{"order_key": logOrderKey(e)})
	if err != nil {
		return err
	}
	if err := tx.Set(s.doc(colLogSeq, logSeqDoc), map[string]any{"last": st.next}); err != nil {
		return fmt.Errorf("write log counter: %w", err)
	}
	if err := tx.Set(st.ref, fields); err != nil {
		return fmt.Errorf("write log entry: %w", err)
	}
	return nil
}

// Record appends a log entry. The same ID with identical content is a no-op;
// with different content ErrConflict.
func (s *Store) Record(ctx context.Context, e data.LogEntry) error {
	if e.ID == "" {
		return fmt.Errorf("record log entry: empty id: %w", data.ErrInvalid)
	}
	err := s.client.RunTransaction(ctx, func(_ context.Context, tx *fs.Transaction) error {
		st, err := s.readLog(tx, e)
		if err != nil || st.exists {
			return err
		}
		return s.writeLog(tx, st, e)
	})
	if err != nil {
		return mapErr("record", err)
	}
	return nil
}

// ApplyLogged writes the change and its log entry in one transaction: both
// are applied or neither. Replaying an identical entry is a no-op.
func (s *Store) ApplyLogged(ctx context.Context, c data.Change, e data.LogEntry) error {
	if err := validateChange(c); err != nil {
		return fmt.Errorf("apply logged: %w", err)
	}
	if strings.TrimSpace(e.ID) == "" {
		return fmt.Errorf("apply logged: empty log id: %w", data.ErrInvalid)
	}
	ref, fields, err := s.changeDoc(c)
	if err != nil {
		return fmt.Errorf("apply logged: %w", err)
	}
	err = s.client.RunTransaction(ctx, func(_ context.Context, tx *fs.Transaction) error {
		return s.applyTx(tx, c, e, ref, fields)
	})
	if err != nil {
		return mapErr("apply logged", err)
	}
	return nil
}

// applyTx is the body of the ApplyLogged transaction: reads first, then the
// change, then the log entry, so a failed log write discards the change.
func (s *Store) applyTx(
	tx *fs.Transaction, c data.Change, e data.LogEntry, ref *fs.DocumentRef, fields map[string]any,
) error {
	st, err := s.readLog(tx, e)
	if err != nil || st.exists {
		return err
	}
	if c.Policy != nil {
		if err := checkPolicy(tx, ref, c.Policy); err != nil {
			return err
		}
	}
	if err := tx.Set(ref, fields); err != nil {
		return fmt.Errorf("write change: %w", err)
	}
	if s.failLog != nil {
		if err := s.failLog(); err != nil {
			return err
		}
	}
	return s.writeLog(tx, st, e)
}
