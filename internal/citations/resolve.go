package citations

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/uslm"
)

// Reasons recorded on entries that are not resolved. They are stable text:
// stewards and the public list show them.
const (
	reasonNoSection  = "the section is not in the stored release point"
	reasonNoSubdiv   = "the section is stored but the cited subdivision is not found in it"
	reasonEtSeq      = "\"et seq.\" names a range of sections; only the first is linked"
	reasonChapter    = "chapter-level provisions are not stored"
	reasonPublicLaw  = "public law text is not a stored source, so no provision is linked"
	reasonOther      = "this kind of reference is not resolved"
	reasonNoActTable = "no official mapping from this Act's sections to the US Code is stored"
	reasonActMulti   = "the Act section is classified to more than one US Code section"
	reasonActLearned = "mapped from the bill's own citation of the same Act section"
	reasonActDirect  = "named code whose section numbers equal the US Code title's"
	reasonDuplicate  = "more than one section of the title has this number"
	reasonBadCite    = "the citation could not be read"
)

// titleIRC is the US Code title that is the Internal Revenue Code.
const titleIRC = 26

// directCodeTitle returns the US Code title of a named code whose sections
// are the sections of that title, so "section 45 of the Internal Revenue Code
// of 1986" is 26 U.S.C. 45.
func directCodeTitle(name string) (int, bool) {
	switch strings.ToLower(name) {
	case "internal revenue code of 1986", "internal revenue code":
		return titleIRC, true
	default:
		return 0, false
	}
}

// ActTable maps a section of a named Act to US Code sections. It starts from
// the direct codes; Learn adds what a bill's own markup says.
type ActTable struct {
	byKey map[string][]string // key: lower-case Act + "|" + lower-case section; values: "title/section"
}

// NewActTable returns an empty table.
func NewActTable() *ActTable { return &ActTable{byKey: map[string][]string{}} }

func actKey(act, section string) string {
	return strings.ToLower(collapse(act)) + "|" + strings.ToLower(NormalizeSection(section))
}

// Learn records that section of act is classified to title/section.
func (t *ActTable) Learn(act, section string, title int, usc string) {
	k := actKey(act, section)
	v := fmt.Sprintf("%d/%s", title, usc)
	if !slices.Contains(t.byKey[k], v) {
		t.byKey[k] = append(t.byKey[k], v)
	}
}

// Lookup returns the "title/section" targets of an Act section.
func (t *ActTable) Lookup(act, section string) []string {
	return t.byKey[actKey(act, section)]
}

// Resolver resolves raw citations against one stored release point.
type Resolver struct {
	Law     data.LawReader
	Release data.Release
	Acts    *ActTable
	cache   map[data.ProvisionRef]sectionResult
}

type sectionResult struct {
	sec data.LawSection
	ok  bool
}

// NewResolver returns a resolver for the release. acts may be nil.
func NewResolver(law data.LawReader, rel data.Release, acts *ActTable) *Resolver {
	if acts == nil {
		acts = NewActTable()
	}
	return &Resolver{Law: law, Release: rel, Acts: acts, cache: map[data.ProvisionRef]sectionResult{}}
}

func validRef(ref data.ProvisionRef) bool { return ref.Validate() == nil }

func (r *Resolver) section(ctx context.Context, title int, sec string) (data.LawSection, bool, error) {
	ref := data.ProvisionRef{Release: r.Release.ID, Title: title, Section: sec}
	if !validRef(ref) {
		return data.LawSection{}, false, nil // a section number that cannot be stored is absent
	}
	if c, ok := r.cache[ref]; ok {
		return c.sec, c.ok, nil
	}
	s, err := r.Law.GetLawSection(ctx, ref)
	switch {
	case errors.Is(err, data.ErrNotFound):
		r.cache[ref] = sectionResult{}
		return data.LawSection{}, false, nil
	case err != nil:
		return data.LawSection{}, false, fmt.Errorf("resolve %s: %w", ref, err)
	}
	r.cache[ref] = sectionResult{s, true}
	return s, true, nil
}

// Resolve turns raw citations into entries: every raw citation yields exactly
// one entry with a state. The one exception is an Act-section phrase
// directly followed by a markup citation of the same provision: the markup
// citation is the entry, and the phrase teaches the act table first, so the
// order of the input does not matter.
func (r *Resolver) Resolve(ctx context.Context, v data.VersionRef, raws []Raw) ([]data.CitationEntry, error) {
	r.learn(raws)
	out := make([]data.CitationEntry, 0, len(raws))
	for _, raw := range raws {
		if raw.Kind == data.CiteActSection && raw.PairedWith >= 0 {
			// The markup citation that follows it is the entry; the phrase
			// only taught the table.
			continue
		}
		e, err := r.resolveOne(ctx, v, raw)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// learn teaches the act table from Act-section phrases paired with a markup
// citation.
func (r *Resolver) learn(raws []Raw) {
	for _, raw := range raws {
		if raw.Kind != data.CiteActSection || raw.PairedWith < 0 || raw.PairedWith >= len(raws) {
			continue
		}
		if p := raws[raw.PairedWith]; p.Kind == data.CiteUSC && p.Title > 0 && p.USCSect != "" {
			r.Acts.Learn(raw.Act, raw.ActSection, p.Title, p.USCSect)
		}
	}
}

func (r *Resolver) resolveOne(ctx context.Context, v data.VersionRef, raw Raw) (data.CitationEntry, error) {
	e := data.CitationEntry{
		Version: v, Section: raw.Section, Line: raw.Line, Text: raw.Text,
		ProvisionIDs: []string{}, State: data.CitationUnresolved,
		Kind: raw.Kind, Source: raw.Source,
	}
	switch raw.Kind {
	case data.CiteUSC:
		return r.resolveUSC(ctx, e, raw.Title, raw.USCSect, raw.Path, raw.EtSeq, "")
	case data.CiteUSCChapter:
		e.Target = fmt.Sprintf("usc-chapter/%d/%s", raw.Title, raw.Chapter)
		e.Reason = reasonChapter
	case data.CitePublicLaw:
		e.Target = "pl/" + strings.Replace(raw.PublicLaw, "-", "/", 1)
		e.Reason = reasonPublicLaw
	case data.CiteActSection:
		return r.resolveAct(ctx, e, raw)
	default:
		e.Reason = reasonOther
	}
	return e, nil
}

func (r *Resolver) resolveAct(ctx context.Context, e data.CitationEntry, raw Raw) (data.CitationEntry, error) {
	e.Target = "act/" + strings.ToLower(raw.Act) + "/" + raw.ActSection
	if title, ok := directCodeTitle(raw.Act); ok {
		return r.resolveUSC(ctx, e, title, raw.ActSection, raw.Path, false, reasonActDirect)
	}
	targets := r.Acts.Lookup(raw.Act, raw.ActSection)
	if len(targets) == 0 {
		e.Reason = reasonNoActTable
		return e, nil
	}
	if len(targets) == 1 {
		title, usc, ok := splitTarget(targets[0])
		if !ok {
			e.Reason = reasonBadCite
			return e, nil
		}
		return r.resolveUSC(ctx, e, title, usc, raw.Path, false, reasonActLearned)
	}
	e.State, e.Reason = data.CitationAmbiguous, reasonActMulti
	for _, t := range targets {
		if title, usc, ok := splitTarget(t); ok {
			e.ProvisionIDs = append(e.ProvisionIDs, data.ProvisionID(title, usc))
		}
	}
	return e, nil
}

// splitTarget splits "42/1395cc-7" into title and section.
func splitTarget(t string) (title int, sec string, ok bool) {
	ts, sec, found := strings.Cut(t, "/")
	title, err := strconv.Atoi(ts)
	return title, sec, found && err == nil
}

// resolveUSC links a US Code citation. A found section with a found
// subdivision is resolved; a found section with a missing subdivision, or an
// "et seq." cite, is ambiguous and links the deepest provision found; a
// missing section is unresolved. note, when set, is recorded as the reason of
// a resolved entry.
func (r *Resolver) resolveUSC(
	ctx context.Context, e data.CitationEntry, title int, sec string, path []string, etSeq bool, note string,
) (data.CitationEntry, error) {
	if title <= 0 || sec == "" {
		e.Reason = reasonBadCite
		return e, nil
	}
	e.Target = data.ProvisionID(title, sec, path...)
	s, ok, err := r.section(ctx, title, sec)
	if err != nil {
		return e, err
	}
	if !ok {
		e.Reason = reasonNoSection
		return e, nil
	}
	// A number shared by two sections is ambiguous: link both.
	if dup, ok, err := r.section(ctx, title, sec+uslm.DuplicateMarker+"2"); err != nil {
		return e, err
	} else if ok {
		e.State, e.Reason = data.CitationAmbiguous, reasonDuplicate
		e.ProvisionIDs = []string{s.Provisions[0].ID, dup.Provisions[0].ID}
		return e, nil
	}
	// Deepest stored provision along the path.
	found, depth := s.Provisions[0].ID, 0
	for d := len(path); d >= 0; d-- {
		id := data.ProvisionID(title, sec, path[:d]...)
		if _, ok := s.Find(id); ok {
			found, depth = id, d
			break
		}
	}
	e.ProvisionIDs = []string{found}
	switch {
	case depth < len(path):
		e.State, e.Reason = data.CitationAmbiguous, reasonNoSubdiv
	case etSeq:
		e.State, e.Reason = data.CitationAmbiguous, reasonEtSeq
	default:
		e.State, e.Reason = data.CitationResolved, note
	}
	return e, nil
}
