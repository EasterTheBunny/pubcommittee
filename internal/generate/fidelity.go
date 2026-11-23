package generate

import (
	"cmp"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Multipliers for the scale words that may follow a number.
const (
	thousandScale = 1e3
	millionScale  = 1e6
	billionScale  = 1e9
	trillionScale = 1e12
)

// Description prefixes, one per fact kind.
const (
	numberFactPrefix  = "number "
	monthFactPrefix   = "month "
	spelledFactPrefix = "word-number "
)

// fact is one checkable statement found in the text.
type fact struct {
	offset      int
	description string
	supported   bool
}

// numberToken is a number as written, with the scale word that follows it.
type numberToken struct {
	start int
	raw   string
	scale string
}

// fidelityPatterns holds the regular expressions used by one FactFidelity call.
type fidelityPatterns struct {
	number  *regexp.Regexp
	scale   *regexp.Regexp
	month   *regexp.Regexp
	spelled *regexp.Regexp
	letters *regexp.Regexp
	commas  *regexp.Regexp
}

// FactFidelity returns the facts of text that source does not support, one
// short description each ("number 5000", "month january", "word-number ten"); nil when every fact is supported.
func FactFidelity(text, source string) []string {
	p := compilePatterns()
	src := strings.ToLower(p.stripThousandsCommas(source))
	plain, all := p.sourceNumbers(src)
	words := p.sourceWords(src)

	facts := slices.Concat(
		p.numberFacts(text, all),
		p.monthFacts(text, words),
		p.spelledFacts(text, words, plain),
	)
	slices.SortStableFunc(facts, func(a, b fact) int { return cmp.Compare(a.offset, b.offset) })

	var out []string
	seen := make(map[string]bool, len(facts))
	for _, f := range facts {
		if seen[f.description] {
			continue
		}
		seen[f.description] = true
		if !f.supported {
			out = append(out, f.description)
		}
	}
	return out
}

func compilePatterns() fidelityPatterns {
	months := []string{
		"january", "february", "march", "april", "may", "june", "july",
		"august", "september", "october", "november", "december",
	}
	spelled := slices.Sorted(maps.Keys(spelledNumbers()))
	return fidelityPatterns{
		number:  regexp.MustCompile(`\d[\d,]*(\.\d+)?`),
		scale:   regexp.MustCompile(`^\s+(?i:(thousand|million|billion|trillion))\b`),
		month:   regexp.MustCompile(`(?i)\b(` + strings.Join(months, "|") + `)\b`),
		spelled: regexp.MustCompile(`(?i)\b(` + strings.Join(spelled, "|") + `)\b`),
		letters: regexp.MustCompile(`[a-z]+`),
		commas:  regexp.MustCompile(`(\d),(\d{3})\b`),
	}
}

// stripThousandsCommas removes commas that separate thousands groups.
// Repeats because one pass cannot reuse a digit consumed by the previous match.
func (p fidelityPatterns) stripThousandsCommas(s string) string {
	for {
		next := p.commas.ReplaceAllString(s, "$1$2")
		if next == s {
			return s
		}
		s = next
	}
}

func (p fidelityPatterns) numberTokens(s string) []numberToken {
	matches := p.number.FindAllStringIndex(s, -1)
	out := make([]numberToken, 0, len(matches))
	for _, loc := range matches {
		tok := numberToken{start: loc[0], raw: strings.ReplaceAll(s[loc[0]:loc[1]], ",", "")}
		if m := p.scale.FindStringSubmatch(s[loc[1]:]); m != nil {
			tok.scale = strings.ToLower(m[1])
		}
		out = append(out, tok)
	}
	return out
}

// sourceNumbers returns the canonical values of plain numbers in src, and of
// plain numbers plus every number and scale-word combination.
func (p fidelityPatterns) sourceNumbers(src string) (plain, all map[string]bool) {
	plain = make(map[string]bool)
	all = make(map[string]bool)
	for _, tok := range p.numberTokens(src) {
		v, err := strconv.ParseFloat(tok.raw, 64)
		if err != nil {
			continue
		}
		plain[canonical(v)] = true
		all[canonical(v)] = true
		if tok.scale != "" {
			all[canonical(v*scaleMultiplier(tok.scale))] = true
		}
	}
	return plain, all
}

// sourceWords returns the whole alphabetic words of the lowercased src.
func (p fidelityPatterns) sourceWords(src string) map[string]bool {
	set := make(map[string]bool)
	for _, w := range p.letters.FindAllString(src, -1) {
		set[w] = true
	}
	return set
}

func (p fidelityPatterns) numberFacts(text string, all map[string]bool) []fact {
	tokens := p.numberTokens(text)
	out := make([]fact, 0, len(tokens))
	for _, tok := range tokens {
		v, err := strconv.ParseFloat(tok.raw, 64)
		if err != nil {
			out = append(out, fact{offset: tok.start, description: numberFactPrefix + tok.raw})
			continue
		}
		canon := canonical(v * scaleMultiplier(tok.scale))
		out = append(out, fact{offset: tok.start, description: numberFactPrefix + canon, supported: all[canon]})
	}
	return out
}

func (p fidelityPatterns) monthFacts(text string, words map[string]bool) []fact {
	matches := p.month.FindAllStringIndex(text, -1)
	out := make([]fact, 0, len(matches))
	for _, loc := range matches {
		name := strings.ToLower(text[loc[0]:loc[1]])
		// "may" is a month only when a day number follows ("May 5"); it is
		// otherwise the modal verb.
		if name == "may" && !startsWithDay(text[loc[1]:]) {
			continue
		}
		out = append(out, fact{offset: loc[0], description: monthFactPrefix + name, supported: words[name]})
	}
	return out
}

func (p fidelityPatterns) spelledFacts(text string, words, plain map[string]bool) []fact {
	values := spelledNumbers()
	matches := p.spelled.FindAllStringIndex(text, -1)
	out := make([]fact, 0, len(matches))
	for _, loc := range matches {
		word := strings.ToLower(text[loc[0]:loc[1]])
		supported := words[word] || plain[canonical(values[word])]
		out = append(out, fact{offset: loc[0], description: spelledFactPrefix + word, supported: supported})
	}
	return out
}

// Values of the spelled-out numbers FactFidelity checks. "one" is excluded on purpose.
const (
	two       = 2
	three     = 3
	four      = 4
	five      = 5
	six       = 6
	seven     = 7
	eight     = 8
	nine      = 9
	ten       = 10
	eleven    = 11
	twelve    = 12
	thirteen  = 13
	fourteen  = 14
	fifteen   = 15
	sixteen   = 16
	seventeen = 17
	eighteen  = 18
	nineteen  = 19
	twenty    = 20
	thirty    = 30
	forty     = 40
	fifty     = 50
	sixty     = 60
	seventy   = 70
	eighty    = 80
	ninety    = 90
	hundred   = 100
)

// spelledNumbers maps each spelled-out number to its value.
func spelledNumbers() map[string]float64 {
	return map[string]float64{
		"two": two, "three": three, "four": four, "five": five, "six": six, "seven": seven,
		"eight": eight, "nine": nine, "ten": ten, "eleven": eleven, "twelve": twelve,
		"thirteen": thirteen, "fourteen": fourteen, "fifteen": fifteen, "sixteen": sixteen,
		"seventeen": seventeen, "eighteen": eighteen, "nineteen": nineteen, "twenty": twenty,
		"thirty": thirty, "forty": forty, "fifty": fifty, "sixty": sixty, "seventy": seventy,
		"eighty": eighty, "ninety": ninety, "hundred": hundred,
	}
}

// scaleMultiplier returns the value a scale word adds to a number, 1 when there is none.
func scaleMultiplier(word string) float64 {
	switch strings.ToLower(word) {
	case "thousand":
		return thousandScale
	case "million":
		return millionScale
	case "billion":
		return billionScale
	case "trillion":
		return trillionScale
	default:
		return 1
	}
}

// canonical is the string form used to compare numbers.
func canonical(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// startsWithDay reports whether s begins with spaces and then a digit.
func startsWithDay(s string) bool {
	t := strings.TrimLeft(s, " ")
	return len(t) < len(s) && t != "" && t[0] >= '0' && t[0] <= '9'
}
