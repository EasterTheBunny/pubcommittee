package citations

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// ExcerptLimit is the most characters of a provision stored as its excerpt.
const ExcerptLimit = 600

// LaterLaw is a public law enacted after the release point, and the US Code
// sections it cites (section-level ids such as "usc/42/1395cc-7"). A cited
// provision in a bill is flagged when a later law cites its section: the law
// may have changed it.
type LaterLaw struct {
	PublicLaw string
	EnactedAt time.Time
	Sections  []string
}

// LaterLawFromSet builds a LaterLaw from the citation set of the enacted
// text of a public law: every US Code section it cites.
func LaterLawFromSet(publicLaw string, enactedAt time.Time, s data.CitationSet) LaterLaw {
	l := LaterLaw{PublicLaw: publicLaw, EnactedAt: enactedAt}
	for _, e := range s.Links {
		if e.Kind != data.CiteUSC {
			continue
		}
		if id := sectionLevel(e.Target); id != "" && !slices.Contains(l.Sections, id) {
			l.Sections = append(l.Sections, id)
		}
	}
	slices.Sort(l.Sections)
	return l
}

// sectionLevel cuts a provision id down to "usc/<title>/<section>".
func sectionLevel(id string) string {
	parts := strings.Split(id, "/")
	if len(parts) < minParts || parts[0] != uscPrefix {
		return ""
	}
	return strings.Join(parts[:minParts], "/")
}

// Excerpt returns the provision with its text cut at ExcerptLimit characters
// on a word boundary, and whether it was cut.
func Excerpt(p data.Provision, release string) data.CitedProvision {
	out := data.CitedProvision{Provision: p, Release: release}
	if utf8.RuneCountInString(p.Text) <= ExcerptLimit {
		return out
	}
	cut := []rune(p.Text)[:ExcerptLimit]
	text := string(cut)
	if i := strings.LastIndex(text, " "); i > ExcerptLimit/2 {
		text = text[:i]
	}
	out.Text, out.Truncated = text, true
	return out
}

// Build resolves the citations of one version against a release point and
// returns the set to store. Same version bytes, release, and rules give the
// same set.
func Build(
	ctx context.Context, law data.LawReader, rel data.Release, v data.OfficialVersion, src []byte,
	parserVersion string, later []LaterLaw,
) (data.CitationSet, error) {
	raws, err := Extract(v, src)
	if err != nil {
		return data.CitationSet{}, err
	}
	res := NewResolver(law, rel, nil)
	links, err := res.Resolve(ctx, v.Ref, raws)
	if err != nil {
		return data.CitationSet{}, err
	}
	set := data.CitationSet{
		Ref:     data.CitationRef{Version: v.Ref, Method: Method(parserVersion, rel.ID)},
		Release: rel.ID, ReleaseDate: rel.Date, Links: links,
		Provisions: []data.CitedProvision{}, Freshness: []data.FreshnessFlag{},
		Meta: data.Meta{Version: 1, AsOf: v.PublishedAt, Provenance: data.ProvenanceAuto, ContentClass: data.ContentPlatform},
	}
	ids := provisionIDs(links)
	for _, id := range ids {
		p, ok, err := res.Provision(ctx, id)
		if err != nil {
			return data.CitationSet{}, err
		}
		if ok {
			set.Provisions = append(set.Provisions, Excerpt(p, rel.ID))
		}
	}
	set.Freshness = freshness(ids, rel, later)
	return set, nil
}

// provisionIDs returns the distinct provision ids the links name, sorted.
func provisionIDs(links []data.CitationEntry) []string {
	var ids []string
	for _, l := range links {
		for _, id := range l.ProvisionIDs {
			if !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
	}
	slices.Sort(ids)
	return ids
}

// freshness flags each provision whose section a law enacted after the
// release date cites.
func freshness(ids []string, rel data.Release, later []LaterLaw) []data.FreshnessFlag {
	flags := []data.FreshnessFlag{}
	for _, id := range ids {
		for _, ll := range later {
			if ll.EnactedAt.After(rel.Date) && slices.Contains(ll.Sections, sectionLevel(id)) {
				flags = append(flags, data.FreshnessFlag{ProvisionID: id, PublicLaw: ll.PublicLaw, EnactedAt: ll.EnactedAt})
			}
		}
	}
	return flags
}

// Provision returns one stored provision by id; false when the id is not a
// US Code id or the provision is not stored.
func (r *Resolver) Provision(ctx context.Context, id string) (data.Provision, bool, error) {
	title, sec, _, perr := data.ParseProvisionID(id)
	if perr == nil {
		s, ok, err := r.section(ctx, title, sec)
		if err != nil {
			return data.Provision{}, false, err
		}
		if ok {
			p, found := s.Find(id)
			return p, found, nil
		}
	}
	return data.Provision{}, false, nil
}

// Step is the citation step that runs after a version is stored and a
// release point is stored: it resolves the citations of every stored version
// of the bill against the current release point and writes the sets. Writes
// are write-once, so repeating the step with the same release is a no-op. It
// returns the number of sets written or confirmed. Nothing runs it
// automatically in the local phase; a command or job calls it over stored
// data. later lists public laws enacted after the release point, for the
// freshness flags.
func Step(
	ctx context.Context, text data.BillText, law data.LawReader, store data.CitationWriter,
	bill data.BillRef, parserVersion string, later []LaterLaw,
) (int, error) {
	rel, err := law.CurrentRelease(ctx)
	if err != nil {
		return 0, fmt.Errorf("citations %s: current release: %w", bill, err)
	}
	vs, err := text.ListVersions(ctx, bill)
	if err != nil {
		return 0, fmt.Errorf("citations %s: list versions: %w", bill, err)
	}
	for _, s := range vs {
		v, err := text.GetVersion(ctx, s.Ref)
		if err != nil {
			return 0, fmt.Errorf("citations %s: get version %s: %w", bill, s.Ref, err)
		}
		src, err := text.GetSource(ctx, s.Ref)
		if err != nil {
			return 0, fmt.Errorf("citations %s: get source %s: %w", bill, s.Ref, err)
		}
		set, err := Build(ctx, law, rel, v, src, parserVersion, later)
		if err != nil {
			return 0, fmt.Errorf("citations %s: %w", s.Ref, err)
		}
		if err := store.PutCitations(ctx, set); err != nil {
			return 0, fmt.Errorf("citations %s: store: %w", s.Ref, err)
		}
	}
	return len(vs), nil
}
