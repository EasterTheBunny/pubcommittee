package firestore

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	fs "cloud.google.com/go/firestore"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

const (
	defaultLimit = 50
	// maxLimit caps one page: a larger Limit is served as maxLimit.
	maxLimit = 1000
)

// docID escapes s into a document id: no slash, and not "." / ".." or
// "__x__" (which Firestore reserves).
func docID(parts ...string) string {
	escaper := strings.NewReplacer("%", "%25", "/", "%2F", "|", "%7C")
	esc := make([]string, len(parts))
	for i, p := range parts {
		esc[i] = escaper.Replace(p)
	}
	id := strings.Join(esc, "|")
	switch {
	case id == "":
		return "%00"
	case id[0] == '.':
		return "%2E" + id[1:]
	case id[0] == '_':
		return "%5F" + id[1:]
	default:
		return id
	}
}

// billID is the id of a bill document.
func billID(b data.BillRef) string {
	return docID(b.Jurisdiction, b.Session, b.Type, fmt.Sprintf("%010d", b.Number))
}

func policyID(v int) string { return fmt.Sprintf("%08d", v) }

// explanationID starts with the version key, so one version's explanations
// share an id prefix; the tail makes (section, provision) unique.
func explanationID(x data.ChangeExplanation) string {
	return docID(x.Version.String(), strings.Join(x.Section, "/"), x.ProvisionID)
}

func (s *Store) doc(col, id string) *fs.DocumentRef { return s.client.Collection(col).Doc(id) }

// encode returns the stored fields of a record: its JSON body plus extra.
func encode(v any, extra map[string]any) (map[string]any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encode: %w: %w", data.ErrInvalid, err)
	}
	fields := map[string]any{fieldBody: string(b)}
	maps.Copy(fields, extra)
	return fields, nil
}

// decode reads the body of a snapshot into v.
func decode(snap *fs.DocumentSnapshot, v any) error {
	raw, err := snap.DataAt(fieldBody)
	if err != nil {
		return fmt.Errorf("%s: no body: %w: %w", snap.Ref.Path, data.ErrUnavailable, err)
	}
	body, ok := raw.(string)
	if !ok {
		return fmt.Errorf("%s: body is not text: %w", snap.Ref.Path, data.ErrUnavailable)
	}
	if err := json.Unmarshal([]byte(body), v); err != nil {
		return fmt.Errorf("%s: decode: %w: %w", snap.Ref.Path, data.ErrUnavailable, err)
	}
	return nil
}

// read decodes the document into v; a missing document is ErrNotFound.
func (s *Store) read(ctx context.Context, col, id string, v any) error {
	snap, err := s.doc(col, id).Get(ctx)
	if err != nil {
		return mapErr(col+"/"+id, err)
	}
	return decode(snap, v)
}

// same reports whether a and b encode to the same JSON.
func same(a, b any) bool {
	x, err1 := json.Marshal(a)
	y, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && string(x) == string(y)
}

func limitOf(p data.Page) (int, error) {
	switch {
	case p.Limit < 0:
		return 0, fmt.Errorf("page limit %d: %w", p.Limit, data.ErrInvalid)
	case p.Limit == 0:
		return defaultLimit, nil
	default:
		return min(p.Limit, maxLimit), nil
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

// GetBill returns bill metadata; ErrNotFound when the bill is unknown.
func (s *Store) GetBill(ctx context.Context, bill data.BillRef) (data.Bill, error) {
	if err := bill.Validate(); err != nil {
		return data.Bill{}, fmt.Errorf("get bill: %w", err)
	}
	var b data.Bill
	if err := s.read(ctx, colBills, billID(bill), &b); err != nil {
		return data.Bill{}, fmt.Errorf("get bill: %w", err)
	}
	return b, nil
}

// GetSelectionPolicy returns the policy with the highest Version.
func (s *Store) GetSelectionPolicy(ctx context.Context) (data.SelectionPolicy, error) {
	snaps, err := s.client.Collection(colPolicy).OrderBy("version", fs.Desc).Limit(1).Documents(ctx).GetAll()
	if err != nil {
		return data.SelectionPolicy{}, mapErr("get selection policy", err)
	}
	if len(snaps) == 0 {
		return data.SelectionPolicy{}, fmt.Errorf("get selection policy: %w", data.ErrNotFound)
	}
	var p data.SelectionPolicy
	if err := decode(snaps[0], &p); err != nil {
		return data.SelectionPolicy{}, fmt.Errorf("get selection policy: %w", err)
	}
	return p, nil
}

// GetSelectionPolicyVersion returns one policy version; ErrNotFound when it
// does not exist.
func (s *Store) GetSelectionPolicyVersion(ctx context.Context, version int) (data.SelectionPolicy, error) {
	var p data.SelectionPolicy
	if err := s.read(ctx, colPolicy, policyID(version), &p); err != nil {
		return data.SelectionPolicy{}, fmt.Errorf("get selection policy version %d: %w", version, err)
	}
	return p, nil
}

// GetExplanations returns the explanations of one version ordered by section
// path, then provision ID.
func (s *Store) GetExplanations(ctx context.Context, ref data.VersionRef) ([]data.ChangeExplanation, error) {
	if err := ref.Validate(); err != nil {
		return nil, fmt.Errorf("get explanations: %w", err)
	}
	snaps, err := s.client.Collection(colExplanations).Where("version_key", "==", ref.String()).Documents(ctx).GetAll()
	if err != nil {
		return nil, mapErr("get explanations", err)
	}
	out := make([]data.ChangeExplanation, 0, len(snaps))
	for _, snap := range snaps {
		var x data.ChangeExplanation
		if err := decode(snap, &x); err != nil {
			return nil, fmt.Errorf("get explanations: %w", err)
		}
		out = append(out, x)
	}
	slices.SortFunc(out, func(a, b data.ChangeExplanation) int {
		if c := strings.Compare(strings.Join(a.Section, "/"), strings.Join(b.Section, "/")); c != 0 {
			return c
		}
		return strings.Compare(a.ProvisionID, b.ProvisionID)
	})
	return out, nil
}

// GetIngestState returns the state of a source; ErrNotFound when none.
func (s *Store) GetIngestState(ctx context.Context, source string) (data.IngestState, error) {
	if source == "" {
		return data.IngestState{}, fmt.Errorf("get ingest state: empty source: %w", data.ErrInvalid)
	}
	var st data.IngestState
	if err := s.read(ctx, colIngest, docID(source), &st); err != nil {
		return data.IngestState{}, fmt.Errorf("get ingest state: %w", err)
	}
	return st, nil
}

// ListLog pages through the public log, newest first.
func (s *Store) ListLog(ctx context.Context, p data.Page) (data.Paged[data.LogEntry], error) {
	empty := data.Paged[data.LogEntry]{}
	limit, err := limitOf(p)
	if err != nil {
		return empty, err
	}
	q := s.client.Collection(colLog).OrderBy("order_key", fs.Desc)
	if p.Cursor != "" {
		var key string
		if err := decodeCursor(p.Cursor, &key); err != nil {
			return empty, err
		}
		q = q.StartAfter(key)
	}
	snaps, err := q.Limit(limit + 1).Documents(ctx).GetAll()
	if err != nil {
		return empty, mapErr("list log", err)
	}
	out := data.Paged[data.LogEntry]{Items: make([]data.LogEntry, 0, min(len(snaps), limit))}
	for _, snap := range snaps[:min(len(snaps), limit)] {
		var e data.LogEntry
		if err := decode(snap, &e); err != nil {
			return empty, fmt.Errorf("list log: %w", err)
		}
		out.Items = append(out.Items, e)
	}
	if len(snaps) > limit {
		last, err := snaps[limit-1].DataAt("order_key")
		if err != nil {
			return empty, fmt.Errorf("list log: %w: %w", data.ErrUnavailable, err)
		}
		if out.Next, err = encodeCursor(last); err != nil {
			return empty, fmt.Errorf("list log: %w", err)
		}
	}
	return out, nil
}
