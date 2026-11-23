package data

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Licences of exports. Official bill text and US Code provisions are US
// government works in the public domain; platform-generated content is CC0.
const (
	LicencePublicDomain = "public-domain"
	LicenceCC0          = "CC0-1.0"
)

// Export kinds (the spec's ExportEntry.kind).
const (
	ExportBillText           = "bill-text"
	ExportExistingLaw        = "existing-law"
	ExportChangeExplanations = "change-explanations"
	ExportOther              = "other"
)

// ExportPrefix is the storage prefix of export files; ExportIndexPath is the
// replaceable index. The index name starts with "_", which no export id can.
const (
	ExportPrefix    = "exports/"
	ExportIndexPath = ExportPrefix + "_index.json"
)

var exportIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// maxExportID is the longest export id the spec allows.
const maxExportID = 128

// ExportEntry is one line of the export index: a published file, its as-of
// time (Meta.AsOf), licence, and hash. The file states the same facts in its
// own header.
type ExportEntry struct {
	ID          string `json:"id"` // file name under exports/
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	ContentType string `json:"content_type"`
	SHA256      string `json:"sha256"`
	SizeBytes   int    `json:"size_bytes"`
	Licence     string `json:"licence"`
	Meta        Meta   `json:"meta"`
}

// ObjectPath returns the storage path of the export file.
func (e ExportEntry) ObjectPath() string { return ExportPrefix + e.ID }

// ValidExportID reports whether id is a legal export id.
func ValidExportID(id string) bool {
	return len(id) <= maxExportID && exportIDPattern.MatchString(id)
}

// Validate reports ErrInvalid for a malformed entry.
func (e ExportEntry) Validate() error {
	switch {
	case !ValidExportID(e.ID):
		return fmt.Errorf("export id %q: %w", e.ID, ErrInvalid)
	case !slices.Contains([]string{ExportBillText, ExportExistingLaw, ExportChangeExplanations, ExportOther}, e.Kind):
		return fmt.Errorf("export %s: kind %q: %w", e.ID, e.Kind, ErrInvalid)
	case e.Licence != LicencePublicDomain && e.Licence != LicenceCC0:
		return fmt.Errorf("export %s: licence %q: %w", e.ID, e.Licence, ErrInvalid)
	case e.ContentType == "" || e.Title == "" || e.Meta.AsOf.IsZero():
		return fmt.Errorf("export %s: missing content type, title or as-of: %w", e.ID, ErrInvalid)
	}
	return nil
}

// ExportReader reads the published exports.
type ExportReader interface {
	// ListExports pages through the index ordered by id. No index yet is an
	// empty page; an unreadable cursor is ErrInvalid.
	ListExports(ctx context.Context, p Page) (Paged[ExportEntry], error)
	// GetExport returns the indexed entry and the file; ErrNotFound for an
	// id the index does not list.
	GetExport(ctx context.Context, id string) (ExportEntry, []byte, error)
}

// ExportWriter publishes exports. Exports are rebuilt from source; an old
// file is replaced whole, never edited in place.
type ExportWriter interface {
	// PutExport stores the file and returns the entry with SHA256 and
	// SizeBytes filled from body. With replace false the path is write-once:
	// identical content is a no-op, different content is ErrConflict. With
	// replace true a later call may replace it. An invalid entry is ErrInvalid.
	PutExport(ctx context.Context, e ExportEntry, body []byte, replace bool) (ExportEntry, error)
	// PutExportIndex replaces the index with these entries, ordered by id.
	// Duplicate ids are ErrInvalid.
	PutExportIndex(ctx context.Context, entries []ExportEntry) error
}

// PutExportFile implements ExportWriter.PutExport over a Publisher.
func PutExportFile(ctx context.Context, p Publisher, e ExportEntry, body []byte, replace bool) (ExportEntry, error) {
	sum := sha256.Sum256(body)
	e.SHA256, e.SizeBytes = hex.EncodeToString(sum[:]), len(body)
	if err := e.Validate(); err != nil {
		return ExportEntry{}, err
	}
	var err error
	if replace {
		_, err = p.PutReplaceable(ctx, e.ObjectPath(), body)
	} else {
		err = p.PutImmutable(ctx, e.ObjectPath(), body)
	}
	if err != nil {
		return ExportEntry{}, fmt.Errorf("put export %s: %w", e.ID, err)
	}
	return e, nil
}

type exportIndex struct {
	Items []ExportEntry `json:"items"`
}

// PutExportIndexFile implements ExportWriter.PutExportIndex over a Publisher.
func PutExportIndexFile(ctx context.Context, p Publisher, entries []ExportEntry) error {
	items := slices.Clone(entries)
	slices.SortFunc(items, func(a, b ExportEntry) int { return strings.Compare(a.ID, b.ID) })
	for i, e := range items {
		if err := e.Validate(); err != nil {
			return err
		}
		if i > 0 && items[i-1].ID == e.ID {
			return fmt.Errorf("export index: duplicate id %s: %w", e.ID, ErrInvalid)
		}
	}
	if items == nil {
		items = []ExportEntry{}
	}
	body, err := json.Marshal(exportIndex{Items: items})
	if err != nil {
		return fmt.Errorf("export index: encode: %w: %w", ErrInvalid, err)
	}
	if _, err := p.PutReplaceable(ctx, ExportIndexPath, body); err != nil {
		return fmt.Errorf("export index: %w", err)
	}
	return nil
}

func readExportIndex(ctx context.Context, p Publisher) (exportIndex, error) {
	body, err := p.GetFile(ctx, ExportIndexPath)
	if errors.Is(err, ErrNotFound) {
		return exportIndex{}, nil
	}
	if err != nil {
		return exportIndex{}, fmt.Errorf("read export index: %w", err)
	}
	var idx exportIndex
	if err := json.Unmarshal(body, &idx); err != nil {
		return exportIndex{}, fmt.Errorf("read export index: decode: %w: %w", ErrInvalid, err)
	}
	return idx, nil
}

// ListExportFiles implements ExportReader.ListExports over a Publisher. The
// cursor is the base64 of the last id of the previous page.
func ListExportFiles(ctx context.Context, p Publisher, page Page) (Paged[ExportEntry], error) {
	after := ""
	if page.Cursor != "" {
		b, err := base64.RawURLEncoding.DecodeString(page.Cursor)
		if err != nil || !ValidExportID(string(b)) {
			return Paged[ExportEntry]{}, fmt.Errorf("export cursor: %w", ErrInvalid)
		}
		after = string(b)
	}
	limit := page.Limit
	if limit <= 0 {
		limit = DefaultPageLimit
	}
	idx, err := readExportIndex(ctx, p)
	if err != nil {
		return Paged[ExportEntry]{}, err
	}
	var out Paged[ExportEntry]
	for _, e := range idx.Items {
		if e.ID <= after {
			continue
		}
		if len(out.Items) == limit {
			out.Next = base64.RawURLEncoding.EncodeToString([]byte(out.Items[len(out.Items)-1].ID))
			break
		}
		out.Items = append(out.Items, e)
	}
	return out, nil
}

// GetExportFile implements ExportReader.GetExport over a Publisher.
func GetExportFile(ctx context.Context, p Publisher, id string) (ExportEntry, []byte, error) {
	if !ValidExportID(id) {
		return ExportEntry{}, nil, fmt.Errorf("export %q: %w", id, ErrNotFound)
	}
	idx, err := readExportIndex(ctx, p)
	if err != nil {
		return ExportEntry{}, nil, err
	}
	i := slices.IndexFunc(idx.Items, func(e ExportEntry) bool { return e.ID == id })
	if i < 0 {
		return ExportEntry{}, nil, fmt.Errorf("export %q: %w", id, ErrNotFound)
	}
	body, err := p.GetFile(ctx, idx.Items[i].ObjectPath())
	if err != nil {
		return ExportEntry{}, nil, fmt.Errorf("get export %s: %w", id, err)
	}
	return idx.Items[i], body, nil
}
