// Package firestore implements PublicDocs, Moderation, and PlatformDocs on
// Cloud Firestore (native mode), for the M1 collections bills, catalog,
// selection_policy, explanations, generations, moderation_log, and
// ingest_state, plus the two counters moderation_log_seq and the log order key.
// Credentials come from workload identity (Application Default
// Credentials), or from FIRESTORE_EMULATOR_HOST for the emulator target, never
// from the configuration. No document stores an account id.
//
// Schema. Every document has a field body, the JSON of the domain record, so
// a record reads back exactly as written (times, empty versus absent fields),
// plus the few fields queries need next to it: catalog entries carry tracked,
// chamber, status, policy_area, markup_status, and search_terms; explanations
// carry version_key; policy versions carry version; log entries carry
// order_key. The document id of a catalog entry is its bill key with the number
// zero padded, so that a plain id order is the bill key order, and the cursor
// of a page is the id of its last entry. Equality filters plus that id order are
// served by merging single-field indexes; composite indexes for the keyword
// queries are in indexes/firestore.indexes.json, which also exempts the large
// body field from indexing.
//
// Transactions. ApplyLogged writes the change, the log entry, and the log
// sequence counter in one transaction, so either all of them or none are
// applied. Record increments the same counter in a transaction.
//
// Not modelled by the emulator (left to the dev database, stage C1): composite
// index requirements, IAM, the real transaction contention and size limits.
package firestore

import (
	"context"
	"errors"
	"fmt"
	"strings"

	fs "cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// Collections of the M1 schema.
const (
	colBills        = "bills"
	colCatalog      = "catalog"
	colPolicy       = "selection_policy"
	colExplanations = "explanations"
	colGenerations  = "generations"
	colLog          = "moderation_log"
	colLogSeq       = "moderation_log_seq" // one document: the last log sequence number
	colIngest       = "ingest_state"

	logSeqDoc = "counter"

	fieldBody = "body"
)

// Collections lists every collection the adapter owns.
func Collections() []string {
	return []string{colBills, colCatalog, colPolicy, colExplanations, colGenerations, colLog, colLogSeq, colIngest}
}

// Config configures a Store.
type Config struct {
	// Project is the Google Cloud project id.
	Project string
	// Database is the Firestore database id; empty means "(default)".
	Database string
}

// Store is the Firestore implementation of the document ports.
type Store struct {
	client *fs.Client
	// failLog, when set by tests, is called inside ApplyLogged after the change
	// is queued and before the log entry is; an error aborts the transaction.
	failLog func() error
}

var (
	_ data.PublicDocs   = (*Store)(nil)
	_ data.PlatformDocs = (*Store)(nil)
	_ data.Moderation   = (*Store)(nil)
)

// New validates cfg and returns a Store. It makes no network call.
func New(ctx context.Context, cfg Config) (*Store, error) {
	if cfg.Project == "" || strings.ContainsAny(cfg.Project, " /\\") {
		return nil, fmt.Errorf("firestore: project %q: %w", cfg.Project, data.ErrInvalid)
	}
	if strings.ContainsAny(cfg.Database, " /\\") {
		return nil, fmt.Errorf("firestore: database %q: %w", cfg.Database, data.ErrInvalid)
	}
	db := cfg.Database
	if db == "" {
		db = fs.DefaultDatabaseID
	}
	c, err := fs.NewClientWithDatabase(ctx, cfg.Project, db)
	if err != nil {
		return nil, mapErr("firestore: new client", err)
	}
	return &Store{client: c}, nil
}

// Close releases the client.
func (s *Store) Close() error {
	if err := s.client.Close(); err != nil {
		return mapErr("firestore: close", err)
	}
	return nil
}

// mapErr wraps a Firestore failure in the typed data errors: a missing
// document is ErrNotFound, an existing one on create or an aborted transaction
// ErrConflict, an oversized or malformed request ErrInvalid, a quota
// ErrRateLimited, and everything else, including permission and a missing
// index, ErrUnavailable. Context errors pass through.
func mapErr(op string, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, data.ErrNotFound), errors.Is(err, data.ErrConflict), errors.Is(err, data.ErrInvalid),
		errors.Is(err, data.ErrUnavailable), errors.Is(err, data.ErrRateLimited),
		errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("%s: %w", op, err)
	}
	switch status.Code(err) { //nolint:exhaustive // the default maps every other code to ErrUnavailable
	case codes.NotFound:
		return fmt.Errorf("%s: %w: %w", op, data.ErrNotFound, err)
	case codes.AlreadyExists, codes.Aborted:
		return fmt.Errorf("%s: %w: %w", op, data.ErrConflict, err)
	case codes.InvalidArgument, codes.OutOfRange:
		return fmt.Errorf("%s: %w: %w", op, data.ErrInvalid, err)
	case codes.ResourceExhausted:
		return fmt.Errorf("%s: %w: %w", op, data.ErrRateLimited, err)
	case codes.Canceled:
		return fmt.Errorf("%s: %w: %w", op, context.Canceled, err)
	case codes.DeadlineExceeded:
		return fmt.Errorf("%s: %w: %w", op, context.DeadlineExceeded, err)
	default:
		return fmt.Errorf("%s: %w: %w", op, data.ErrUnavailable, err)
	}
}
