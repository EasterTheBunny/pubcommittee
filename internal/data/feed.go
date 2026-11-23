package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"
)

// Kinds of FeedEntry: the M1 events of the change feed (design/10 § Change feed).
const (
	FeedCatalogBill          = "catalog-bill"          // a bill entered the catalog
	FeedBillTracked          = "bill-tracked"          // a bill became tracked
	FeedVersion              = "version"               // a new official version was stored
	FeedExplanation          = "explanation"           // a new change explanation
	FeedExplanationCorrected = "explanation-corrected" // a revised change explanation
	FeedPolicy               = "policy"                // a selection policy version
	FeedLogEntry             = "log-entry"             // an administrator log entry
)

// FeedPath is the storage path of the published feed file.
const FeedPath = "feed/entries.json"

// FeedEntry is one typed record of a public change. Updated is the UTC day
// the entry joined the feed, and never carries a time of day. Address is the
// canonical API path of the changed item. Titles are written by the platform
// from templates, never from source or user text.
type FeedEntry struct {
	ID      string    `json:"id"`      // stable and unique; the same event never gets two ids
	Seq     int       `json:"seq"`     // position in the feed, from 1; assigned by AppendFeed
	Updated time.Time `json:"updated"` // UTC midnight of the day the entry joined the feed
	Title   string    `json:"title"`
	Kind    string    `json:"kind"`
	Address string    `json:"address"`
	Meta    Meta      `json:"meta"`
}

// Validate reports ErrInvalid for an entry without id, kind, title or address.
func (e FeedEntry) Validate() error {
	if e.ID == "" || e.Kind == "" || e.Title == "" || e.Address == "" {
		return fmt.Errorf("feed entry %q: missing field: %w", e.ID, ErrInvalid)
	}
	return nil
}

// FeedReader reads the change feed.
type FeedReader interface {
	// ListFeed returns entries newest first. A non-zero since keeps entries
	// whose day is on or after the day of since, so a client that polled
	// mid-day misses nothing; it may see the day's earlier entries again and
	// de-duplicates by id. An unreadable cursor is ErrInvalid; an empty feed
	// is an empty page.
	ListFeed(ctx context.Context, since time.Time, p Page) (Paged[FeedEntry], error)
}

// FeedWriter appends to the change feed.
type FeedWriter interface {
	// AppendFeed adds the entries whose id is not in the feed yet, in order,
	// assigning Seq, and returns how many it added. A repeat call with the
	// same entries adds nothing. An invalid entry is ErrInvalid and nothing
	// is added.
	AppendFeed(ctx context.Context, entries []FeedEntry) (added int, err error)
}

// feedFile is the stored form of the feed: all entries, oldest first. One
// file is enough for M1's volume; a later store may shard it behind the same
// ports.
type feedFile struct {
	Entries []FeedEntry `json:"entries"`
}

func readFeedFile(ctx context.Context, p Publisher) (feedFile, error) {
	body, err := p.GetFile(ctx, FeedPath)
	if errors.Is(err, ErrNotFound) {
		return feedFile{}, nil
	}
	if err != nil {
		return feedFile{}, fmt.Errorf("read feed: %w", err)
	}
	var f feedFile
	if err := json.Unmarshal(body, &f); err != nil {
		return feedFile{}, fmt.Errorf("read feed: decode: %w: %w", ErrInvalid, err)
	}
	return f, nil
}

// AppendFeedFile implements FeedWriter over a Publisher. Adapters call it.
// Two overlapping runs may each read the same file; both then write the same
// ids, so the loser of the race only loses its Seq order for that run.
func AppendFeedFile(ctx context.Context, p Publisher, entries []FeedEntry) (int, error) {
	for _, e := range entries {
		if err := e.Validate(); err != nil {
			return 0, err
		}
	}
	f, err := readFeedFile(ctx, p)
	if err != nil {
		return 0, err
	}
	have := make(map[string]bool, len(f.Entries))
	for _, e := range f.Entries {
		have[e.ID] = true
	}
	added := 0
	for _, e := range entries {
		if have[e.ID] {
			continue
		}
		have[e.ID] = true
		e.Seq = len(f.Entries) + 1
		e.Updated = e.Updated.UTC().Truncate(Day)
		f.Entries = append(f.Entries, e)
		added++
	}
	if added == 0 {
		return 0, nil
	}
	body, err := json.Marshal(f)
	if err != nil {
		return 0, fmt.Errorf("append feed: encode: %w: %w", ErrInvalid, err)
	}
	if _, err := p.PutReplaceable(ctx, FeedPath, body); err != nil {
		return 0, fmt.Errorf("append feed: %w", err)
	}
	return added, nil
}

// Day is the granularity of every public timestamp.
const Day = 24 * time.Hour

// feedCursor decodes the cursor of ListFeedFile: the Seq of the last entry of
// the previous page, or 0 for the first page.
func feedCursor(c string) (int, error) {
	if c == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(c)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("feed cursor: %w", ErrInvalid)
	}
	return n, nil
}

// ListFeedFile implements FeedReader over a Publisher. The cursor is the Seq
// of the last entry of the previous page.
func ListFeedFile(ctx context.Context, p Publisher, since time.Time, page Page) (Paged[FeedEntry], error) {
	before, err := feedCursor(page.Cursor)
	if err != nil {
		return Paged[FeedEntry]{}, err
	}
	limit := page.Limit
	if limit <= 0 {
		limit = DefaultPageLimit
	}
	f, err := readFeedFile(ctx, p)
	if err != nil {
		return Paged[FeedEntry]{}, err
	}
	day := since.UTC().Truncate(Day)
	var out Paged[FeedEntry]
	for _, e := range slices.Backward(f.Entries) {
		if (before > 0 && e.Seq >= before) || (!since.IsZero() && e.Updated.Before(day)) {
			continue
		}
		if len(out.Items) == limit {
			out.Next = strconv.Itoa(out.Items[len(out.Items)-1].Seq)
			break
		}
		out.Items = append(out.Items, e)
	}
	return out, nil
}

// DefaultPageLimit is the page size a Page with Limit 0 gets from the
// publisher-backed feed and export lists.
const DefaultPageLimit = 50
