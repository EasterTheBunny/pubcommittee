package data

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// LineClass says what became of one old line in a newer version (design/02
// § New official version handling).
type LineClass string

// Line classes.
const (
	LineUnchanged LineClass = "unchanged" // same words, same place
	LineMoved     LineClass = "moved"     // same words, elsewhere or renumbered
	LineChanged   LineClass = "changed"   // words altered; a counterpart exists
	LineRemoved   LineClass = "removed"   // no counterpart
)

// Section statuses of SectionComparison.Status.
const (
	SectionUnchanged = "unchanged"
	SectionChanged   = "changed"
	SectionRemoved   = "removed"
	SectionAdded     = "added"
)

// Change kinds of ComparisonChange.Kind.
const (
	ChangeAdded    = "added"
	ChangeRemoved  = "removed"
	ChangeModified = "modified"
	ChangeMoved    = "moved"
)

// ComparisonRef identifies one stored comparison: version codes of the base
// (From) and the compared version (To), and the method that produced it. The
// record is write-once per ref.
type ComparisonRef struct {
	Bill   BillRef `json:"bill"`
	From   string  `json:"from"`
	To     string  `json:"to"`
	Method string  `json:"method"` // parser version plus comparison algorithm version
}

// String returns the canonical key, for example
// "us/119/hr/1234/ih..rh/billxml/1+versiondiff/1".
func (r ComparisonRef) String() string {
	return r.Bill.String() + "/" + r.From + ".." + r.To + "/" + r.Method
}

// Validate reports ErrInvalid for an empty or malformed reference.
func (r ComparisonRef) Validate() error {
	if err := r.Bill.Validate(); err != nil {
		return err
	}
	for _, s := range []string{r.From, r.To} {
		if s == "" || strings.ContainsAny(s, "/.") {
			return fmt.Errorf("comparison %s: version code %q: %w", r, s, ErrInvalid)
		}
	}
	if r.Method == "" {
		return fmt.Errorf("comparison %s: empty method: %w", r, ErrInvalid)
	}
	return nil
}

// ObjectPath returns the storage path of the comparison: bucket-relative,
// "/" separated. "/" inside the method is written as "_".
func (r ComparisonRef) ObjectPath() string {
	return "bills/" + r.Bill.String() + "/compare/" + r.From + ".." + r.To + "/" +
		strings.ReplaceAll(r.Method, "/", "_") + ".json"
}

// LineLoc is a line in a version: section path and line number N.
type LineLoc struct {
	Section []string `json:"section"`
	N       int      `json:"n"`
}

// LineMapping is the fate of one old line. New is nil when Class is removed.
type LineMapping struct {
	N     int       `json:"n"`
	Class LineClass `json:"class"`
	New   *LineLoc  `json:"new,omitempty"`
}

// ComparisonChange is one run of difference: line spans in the old (before) and new
// (after) version. Added has only After, removed only Before.
type ComparisonChange struct {
	Kind          string   `json:"kind"`
	BeforeSection []string `json:"before_section,omitempty"`
	BeforeFrom    int      `json:"before_from,omitempty"`
	BeforeTo      int      `json:"before_to,omitempty"`
	AfterSection  []string `json:"after_section,omitempty"`
	AfterFrom     int      `json:"after_from,omitempty"`
	AfterTo       int      `json:"after_to,omitempty"`
}

// SectionComparison covers one section: an old section (Path set, Lines
// holds one mapping per old line) or an added one (Status added, Path is the
// new path, Lines empty).
type SectionComparison struct {
	Path    []string           `json:"path"`
	NewPath []string           `json:"new_path,omitempty"` // counterpart section, when one exists
	Status  string             `json:"status"`
	Lines   []LineMapping      `json:"lines"`
	Changes []ComparisonChange `json:"changes"`
}

// Comparison maps the lines of version From onto version To. It is derived
// platform content computed from official text, never edited.
type Comparison struct {
	Ref          ComparisonRef       `json:"ref"`
	FromSHA256   string              `json:"from_sha256"` // source hashes of the compared versions
	ToSHA256     string              `json:"to_sha256"`
	Sections     []SectionComparison `json:"sections"`
	Meta         Meta                `json:"meta"`
	LineCountOld int                 `json:"line_count_old"`
	LineCountNew int                 `json:"line_count_new"`
}

// RangeResult is the classification of an old line range.
type RangeResult struct {
	Class LineClass `json:"class"`
	// New is the best new location of the range when one exists: the section
	// and span of the counterpart lines. Nil when the range was removed.
	New *Anchorless `json:"new,omitempty"`
}

// Anchorless is a section and line span without a version.
type Anchorless struct {
	Section []string `json:"section"`
	From    int      `json:"from"`
	To      int      `json:"to"`
}

// Lookup classifies old lines from..to of section, for "changed since"
// notices. A range is unchanged only if every line is; moved if every line
// is unchanged or moved and their counterparts form one contiguous run in
// order within one section; removed if every line is removed; changed
// otherwise. ErrNotFound when the section or a line is not in the comparison.
func (c Comparison) Lookup(section []string, from, to int) (RangeResult, error) {
	if from < 1 || to < from {
		return RangeResult{}, fmt.Errorf("lookup %v %d-%d: %w", section, from, to, ErrInvalid)
	}
	i := slices.IndexFunc(c.Sections, func(sc SectionComparison) bool {
		return sc.Status != SectionAdded && slices.Equal(sc.Path, section)
	})
	if i < 0 {
		return RangeResult{}, fmt.Errorf("lookup %v: %w", section, ErrNotFound)
	}
	var ms []LineMapping
	for _, m := range c.Sections[i].Lines {
		if m.N >= from && m.N <= to {
			ms = append(ms, m)
		}
	}
	if len(ms) != to-from+1 {
		return RangeResult{}, fmt.Errorf("lookup %v %d-%d: line missing: %w", section, from, to, ErrNotFound)
	}
	return classifyRange(ms), nil
}

func classifyRange(ms []LineMapping) RangeResult {
	counts := map[LineClass]int{}
	for _, m := range ms {
		counts[m.Class]++
	}
	span, contiguous := newSpan(ms)
	switch {
	case counts[LineRemoved] == len(ms):
		return RangeResult{Class: LineRemoved}
	case counts[LineUnchanged] == len(ms):
		return RangeResult{Class: LineUnchanged, New: span}
	case counts[LineMoved] > 0 && counts[LineMoved]+counts[LineUnchanged] == len(ms) && contiguous:
		return RangeResult{Class: LineMoved, New: span}
	default:
		return RangeResult{Class: LineChanged, New: span}
	}
}

// newSpan returns the span of the counterparts in the section of the first
// one, and whether all counterparts are consecutive lines of that section.
func newSpan(ms []LineMapping) (span *Anchorless, contiguous bool) {
	contiguous = true
	var last *LineLoc
	for _, m := range ms {
		if m.New == nil {
			continue
		}
		switch {
		case span == nil:
			span = &Anchorless{Section: slices.Clone(m.New.Section), From: m.New.N, To: m.New.N}
		case slices.Equal(m.New.Section, span.Section):
			span.From, span.To = min(span.From, m.New.N), max(span.To, m.New.N)
			contiguous = contiguous && m.New.N == last.N+1
		default:
			contiguous = false
		}
		last = m.New
	}
	return span, contiguous
}

// ComparisonReader reads stored comparisons. Not found: ErrNotFound.
type ComparisonReader interface {
	// GetComparison returns the comparison with exactly this ref.
	GetComparison(ctx context.Context, ref ComparisonRef) (Comparison, error)
}

// ComparisonWriter stores comparisons.
type ComparisonWriter interface {
	// PutComparison stores a comparison at a write-once ref. Identical
	// content is a no-op; different content is ErrConflict; an invalid ref
	// is ErrInvalid.
	PutComparison(ctx context.Context, c Comparison) error
}
