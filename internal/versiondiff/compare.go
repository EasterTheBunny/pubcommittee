package versiondiff

import (
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/easterthebunny/pubcommittee/internal/billxml"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

// AlgorithmVersion is part of every comparison's method. Any change to the
// rules in the package comment needs a new value.
const AlgorithmVersion = "versiondiff/1"

// ChangedThreshold is the least word similarity (Dice coefficient, 0..1) at
// which an old and a new line with different text count as one changed line.
// Below it the old line is removed.
const ChangedThreshold = 0.5

const (
	maxLCSCells    = 40_000_000 // larger section pairs skip the in-order step
	maxTokenSpread = 64         // words in more new lines than this are not used to find candidates
)

// Method returns the method string stored in a ComparisonRef: the parser
// version of the stored text plus this package's algorithm version.
func Method(parserVersion string) string { return parserVersion + "+" + AlgorithmVersion }

// Compare maps from onto to. parserVersion is the version of the parser that
// produced both texts (billxml.ParserVersion for M1).
func Compare(from, to data.OfficialVersion, parserVersion string) data.Comparison {
	secs := CompareSections(from.Sections, to.Sections)
	return data.Comparison{
		Ref: data.ComparisonRef{
			Bill: from.Ref.Bill, From: from.Ref.Version, To: to.Ref.Version, Method: Method(parserVersion),
		},
		FromSHA256:   from.SourceSHA256,
		ToSHA256:     to.SourceSHA256,
		Sections:     secs,
		LineCountOld: countLines(from.Sections),
		LineCountNew: countLines(to.Sections),
		Meta: data.Meta{
			Version: 1, AsOf: to.PublishedAt.UTC(),
			Provenance: data.ProvenanceAuto, ContentClass: data.ContentPlatform,
		},
	}
}

// CompareDocuments is the extension point for ingest: it compares two parsed
// documents and returns the per-section mappings.
func CompareDocuments(prev, next *billxml.Document) []data.SectionComparison {
	return CompareSections(prev.DataSections(), next.DataSections())
}

func countLines(secs []data.Section) int {
	n := 0
	for _, s := range secs {
		n += len(s.Lines)
	}
	return n
}

type flatLine struct {
	sec      int
	pos      int // index in the section's Lines
	l        data.Line
	norm     string
	stripped string
	words    []string
}

type side struct {
	secs  []data.Section
	lines []flatLine
	first []int // flat index of each section's first line
}

var enumerator = regexp.MustCompile(`(?i)^(?:(?:\([a-z0-9]+\)\s*)+|(?:sec\.|section)\s*\d+[a-z]*\.?\s*|\d+\.\s+)`)

// normalize collapses white space; wording, case and punctuation are kept.
func normalize(s string) string { return strings.Join(strings.Fields(s), " ") }

func flatten(secs []data.Section) side {
	sd := side{secs: secs, first: make([]int, len(secs))}
	for si, s := range secs {
		sd.first[si] = len(sd.lines)
		for pi, l := range s.Lines {
			n := normalize(l.Text)
			fl := flatLine{sec: si, pos: pi, l: l, norm: n, words: strings.Fields(n)}
			if loc := enumerator.FindStringIndex(n); loc != nil {
				fl.stripped = strings.TrimSpace(n[loc[1]:])
			}
			sd.lines = append(sd.lines, fl)
		}
	}
	return sd
}

// pairSections pairs old and new sections one to one.
func pairSections(o, n side) (oldTo, newFrom []int) {
	oldTo = fill(len(o.secs))
	newFrom = fill(len(n.secs))
	pass := func(key func(data.Section) (string, bool)) {
		queues := map[string][]int{}
		for j, s := range n.secs {
			if newFrom[j] >= 0 {
				continue
			}
			if k, ok := key(s); ok {
				queues[k] = append(queues[k], j)
			}
		}
		for i, s := range o.secs {
			if oldTo[i] >= 0 {
				continue
			}
			k, ok := key(s)
			if !ok || len(queues[k]) == 0 {
				continue
			}
			j := queues[k][0]
			queues[k] = queues[k][1:]
			oldTo[i], newFrom[j] = j, i
		}
	}
	pass(func(s data.Section) (string, bool) { return s.SourceID, s.SourceID != "" })
	pass(func(s data.Section) (string, bool) {
		return normalize(s.Label) + "|" + normalize(s.Heading), true
	})
	pass(func(s data.Section) (string, bool) { return normalize(s.Heading), normalize(s.Heading) != "" })
	return oldTo, newFrom
}

func fill(n int) []int {
	s := make([]int, n)
	for i := range s {
		s[i] = -1
	}
	return s
}

// matcher holds the line matching under construction.
type matcher struct {
	o, n    side
	oldTo   []int // flat old -> flat new
	newFrom []int
	class   []data.LineClass // per old flat line
}

func (m *matcher) link(i, j int, c data.LineClass) {
	m.oldTo[i], m.newFrom[j], m.class[i] = j, i, c
}

// CompareSections maps the old sections onto the new ones.
func CompareSections(from, to []data.Section) []data.SectionComparison {
	m := &matcher{o: flatten(from), n: flatten(to)}
	m.oldTo = fill(len(m.o.lines))
	m.newFrom = fill(len(m.n.lines))
	m.class = make([]data.LineClass, len(m.o.lines))
	secOld, secNew := pairSections(m.o, m.n)

	m.matchInOrder(secOld)
	m.matchExact(secOld, func(l flatLine) string { return l.norm })
	m.matchExact(secOld, func(l flatLine) string { return l.stripped })
	m.matchSimilar(secOld)
	for i := range m.class {
		if m.oldTo[i] < 0 {
			m.class[i] = data.LineRemoved
		}
	}
	return m.build(secOld, secNew)
}

// matchInOrder pairs identical lines in order inside each pair of sections.
func (m *matcher) matchInOrder(secOld []int) {
	for si, sj := range secOld {
		if sj < 0 {
			continue
		}
		moved := normalize(m.o.secs[si].Label) != normalize(m.n.secs[sj].Label)
		oi, ni := m.sectionRange(m.o, si), m.sectionRange(m.n, sj)
		class := data.LineUnchanged
		if moved {
			class = data.LineMoved
		}
		for _, p := range lcs(m.o.lines[oi[0]:oi[1]], m.n.lines[ni[0]:ni[1]]) {
			m.link(oi[0]+p[0], ni[0]+p[1], class)
		}
	}
}

func (m *matcher) sectionRange(s side, si int) [2]int {
	end := len(s.lines)
	if si+1 < len(s.first) {
		end = s.first[si+1]
	}
	return [2]int{s.first[si], end}
}

// lcs returns index pairs of a longest common subsequence of identical
// normalised lines, after trimming a common prefix and suffix.
func lcs(a, b []flatLine) [][2]int {
	var out [][2]int
	lo := 0
	for lo < len(a) && lo < len(b) && a[lo].norm == b[lo].norm {
		out = append(out, [2]int{lo, lo})
		lo++
	}
	ea, eb := len(a), len(b)
	var tail [][2]int
	for ea > lo && eb > lo && a[ea-1].norm == b[eb-1].norm {
		ea--
		eb--
		tail = append(tail, [2]int{ea, eb})
	}
	slices.Reverse(tail)
	if w, h := ea-lo, eb-lo; w > 0 && h > 0 && w*h <= maxLCSCells {
		out = append(out, middleLCS(a[lo:ea], b[lo:eb], lo)...)
	}
	return append(out, tail...)
}

// middleLCS runs the dynamic program on the part left after trimming; the
// returned indices are shifted by off.
func middleLCS(a, b []flatLine, off int) [][2]int {
	w, h := len(a), len(b)
	// dp[i][j] = LCS length of a[i:] and b[j:].
	dp := make([]int32, (w+1)*(h+1))
	at := func(i, j int) int { return i*(h+1) + j }
	for i := w - 1; i >= 0; i-- {
		for j := h - 1; j >= 0; j-- {
			if a[i].norm == b[j].norm {
				dp[at(i, j)] = dp[at(i+1, j+1)] + 1
			} else {
				dp[at(i, j)] = max(dp[at(i+1, j)], dp[at(i, j+1)])
			}
		}
	}
	var out [][2]int
	for i, j := 0, 0; i < w && j < h; {
		switch {
		case a[i].norm == b[j].norm:
			out = append(out, [2]int{off + i, off + j})
			i++
			j++
		case dp[at(i+1, j)] >= dp[at(i, j+1)]:
			i++
		default:
			j++
		}
	}
	return out
}

// matchExact pairs still-unmatched lines whose key is equal and not empty,
// preferring a counterpart in the paired section, then the first in order.
func (m *matcher) matchExact(secOld []int, key func(flatLine) string) {
	index := map[string][]int{}
	for j, l := range m.n.lines {
		if m.newFrom[j] < 0 {
			if k := key(l); k != "" {
				index[k] = append(index[k], j)
			}
		}
	}
	for i, l := range m.o.lines {
		if m.oldTo[i] >= 0 {
			continue
		}
		k := key(l)
		cands := index[k]
		if k == "" || len(cands) == 0 {
			continue
		}
		pick := 0
		for c, j := range cands {
			if m.n.lines[j].sec == secOld[l.sec] {
				pick = c
				break
			}
		}
		m.link(i, cands[pick], data.LineMoved)
		index[k] = slices.Delete(slices.Clone(cands), pick, pick+1)
	}
}

type candidate struct {
	i, j  int
	score float64
}

// matchSimilar pairs the remaining lines by word similarity, best first.
func (m *matcher) matchSimilar(secOld []int) {
	cands := m.candidates(secOld)
	sort.Slice(cands, func(a, b int) bool {
		x, y := cands[a], cands[b]
		if x.score != y.score {
			return x.score > y.score
		}
		if x.i != y.i {
			return x.i < y.i
		}
		return x.j < y.j
	})
	for _, c := range cands {
		if m.oldTo[c.i] < 0 && m.newFrom[c.j] < 0 {
			m.link(c.i, c.j, data.LineChanged)
		}
	}
}

// candidates scores every unmatched old line against the unmatched new lines
// that share a word with it, keeping scores at or above ChangedThreshold.
func (m *matcher) candidates(secOld []int) []candidate {
	byWord := map[string][]int{}
	for j, l := range m.n.lines {
		if m.newFrom[j] >= 0 {
			continue
		}
		for _, w := range dedupe(l.words) {
			byWord[w] = append(byWord[w], j)
		}
	}
	var cands []candidate
	for i, l := range m.o.lines {
		if m.oldTo[i] >= 0 || len(l.words) == 0 {
			continue
		}
		cands = append(cands, m.scoreLine(i, secOld[l.sec], byWord)...)
	}
	return cands
}

// scoreLine scores old line i against the new lines that share a word with it.
func (m *matcher) scoreLine(i, pairedSec int, byWord map[string][]int) []candidate {
	l := m.o.lines[i]
	shared := map[int]bool{}
	for _, w := range dedupe(l.words) {
		if js := byWord[w]; len(js) <= maxTokenSpread {
			for _, j := range js {
				shared[j] = true
			}
		}
	}
	var out []candidate
	for j := range shared {
		s := dice(l.words, m.n.lines[j].words)
		if s < ChangedThreshold {
			continue
		}
		if m.n.lines[j].sec == pairedSec {
			s += 0.001 // prefer the paired section on ties
		}
		out = append(out, candidate{i, j, s})
	}
	return out
}

func dedupe(ws []string) []string {
	out := slices.Clone(ws)
	slices.Sort(out)
	return slices.Compact(out)
}

// dice is the Dice coefficient on word multisets.
func dice(a, b []string) float64 {
	count := map[string]int{}
	for _, w := range a {
		count[w]++
	}
	common := 0
	for _, w := range b {
		if count[w] > 0 {
			count[w]--
			common++
		}
	}
	return 2 * float64(common) / float64(len(a)+len(b))
}

func (m *matcher) loc(j int) *data.LineLoc {
	l := m.n.lines[j]
	return &data.LineLoc{Section: slices.Clone(m.n.secs[l.sec].Path), N: l.l.N}
}

// build assembles the per-section output.
func (m *matcher) build(secOld, secNew []int) []data.SectionComparison {
	out := make([]data.SectionComparison, 0, len(m.o.secs))
	for si := range m.o.secs {
		out = append(out, m.oldSection(si, secOld[si]))
	}
	for sj, s := range m.n.secs {
		if secNew[sj] >= 0 {
			continue
		}
		changes := m.addedRuns(sj)
		if len(changes) == 0 && len(s.Lines) > 0 {
			continue // every line came from elsewhere in the old version
		}
		out = append(out, data.SectionComparison{
			Path: slices.Clone(s.Path), Status: data.SectionAdded, Lines: []data.LineMapping{}, Changes: changes,
		})
	}
	return out
}

// oldSection describes old section si; pair is its paired new section or -1.
func (m *matcher) oldSection(si, pair int) data.SectionComparison {
	sc := data.SectionComparison{
		Path: slices.Clone(m.o.secs[si].Path), Lines: []data.LineMapping{}, Changes: []data.ComparisonChange{},
	}
	rng := m.sectionRange(m.o, si)
	votes := map[int]int{}
	for i := rng[0]; i < rng[1]; i++ {
		lm := data.LineMapping{N: m.o.lines[i].l.N, Class: m.class[i]}
		if j := m.oldTo[i]; j >= 0 {
			lm.New = m.loc(j)
			votes[m.n.lines[j].sec]++
		}
		sc.Lines = append(sc.Lines, lm)
	}
	newSec := pair
	if newSec < 0 {
		newSec = majority(votes)
	}
	if newSec >= 0 {
		sc.NewPath = slices.Clone(m.n.secs[newSec].Path)
	}
	sc.Changes = m.runs(rng, pair)
	sc.Status = m.status(sc.Lines, pair)
	return sc
}

// status is the status of an old section with these line mappings and pair.
func (m *matcher) status(lines []data.LineMapping, pair int) string {
	only := func(c data.LineClass) bool {
		return !slices.ContainsFunc(lines, func(l data.LineMapping) bool { return l.Class != c })
	}
	switch {
	case pair < 0 && (len(lines) == 0 || only(data.LineRemoved)):
		return data.SectionRemoved
	case pair >= 0 && only(data.LineUnchanged) && !m.hasAdded(pair):
		return data.SectionUnchanged
	default:
		return data.SectionChanged
	}
}

func majority(votes map[int]int) int {
	best, bestN := -1, 0
	for sec, n := range votes {
		if n > bestN || (n == bestN && sec < best) {
			best, bestN = sec, n
		}
	}
	return best
}

func (m *matcher) hasAdded(sj int) bool {
	r := m.sectionRange(m.n, sj)
	for j := r[0]; j < r[1]; j++ {
		if m.newFrom[j] < 0 {
			return true
		}
	}
	return false
}

// runs lists the changes of one old section: runs of removed, changed, and
// moved lines, plus the added runs of its paired new section.
func (m *matcher) runs(rng [2]int, pair int) []data.ComparisonChange {
	changes := []data.ComparisonChange{}
	for i := rng[0]; i < rng[1]; {
		c := m.class[i]
		if c == data.LineUnchanged {
			i++
			continue
		}
		k := i
		for k+1 < rng[1] && m.class[k+1] == c && m.adjacent(c, k, k+1) {
			k++
		}
		ch := data.ComparisonChange{
			BeforeSection: slices.Clone(m.o.secs[m.o.lines[i].sec].Path),
			BeforeFrom:    m.o.lines[i].l.N, BeforeTo: m.o.lines[k].l.N,
		}
		ch.Kind = data.ChangeRemoved
		if c != data.LineRemoved {
			ch.Kind = data.ChangeModified
			if c == data.LineMoved {
				ch.Kind = data.ChangeMoved
			}
			a, b := m.n.lines[m.oldTo[i]], m.n.lines[m.oldTo[k]]
			ch.AfterSection = slices.Clone(m.n.secs[a.sec].Path)
			ch.AfterFrom, ch.AfterTo = a.l.N, b.l.N
		}
		changes = append(changes, ch)
		i = k + 1
	}
	if pair >= 0 {
		changes = append(changes, m.addedRuns(pair)...)
	}
	return changes
}

// adjacent reports whether old lines a and b (consecutive) continue one run.
func (m *matcher) adjacent(c data.LineClass, a, b int) bool {
	if c == data.LineRemoved {
		return true
	}
	ja, jb := m.oldTo[a], m.oldTo[b]
	return jb == ja+1 && m.n.lines[ja].sec == m.n.lines[jb].sec
}

func (m *matcher) addedRuns(sj int) []data.ComparisonChange {
	var out []data.ComparisonChange
	r := m.sectionRange(m.n, sj)
	for j := r[0]; j < r[1]; {
		if m.newFrom[j] >= 0 {
			j++
			continue
		}
		k := j
		for k+1 < r[1] && m.newFrom[k+1] < 0 {
			k++
		}
		out = append(out, data.ComparisonChange{
			Kind: data.ChangeAdded, AfterSection: slices.Clone(m.n.secs[sj].Path),
			AfterFrom: m.n.lines[j].l.N, AfterTo: m.n.lines[k].l.N,
		})
		j = k + 1
	}
	return out
}
