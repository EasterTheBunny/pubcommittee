package generate

import (
	"regexp"
	"slices"
	"strings"
)

// neutralWords returns the evaluative words that generated text must not use.
// The list lives in a function, not a package variable, so it cannot be changed
// at run time; extend it here.
func neutralWords() []string {
	return []string{
		"unfortunately", "fortunately", "wasteful", "harmful", "beneficial",
		"dangerous", "radical", "extreme", "excellent", "terrible", "good",
		"bad", "better", "worse", "best", "worst", "should", "ought", "urge",
		"urges", "recommend", "recommends", "recommended", "commendable",
		"controversial", "misguided", "overdue", "crucial", "vital", "essential",
		"sensible", "reckless", "irresponsible", "welcome", "regrettable", "smart",
		"foolish", "unwise", "wise", "long-overdue", "much-needed", "common-sense",
		"commonsense", "sweeping", "draconian", "landmark",
	}
}

// neutralWordPattern matches one word, where a hyphenated form such as
// "long-overdue" is a single word. Matching whole tokens keeps "overdue" from
// matching inside "long-overdue" and "bad" from matching inside "badge".
func neutralWordPattern() *regexp.Regexp {
	return regexp.MustCompile(`[A-Za-z]+(?:-[A-Za-z]+)*`)
}

// Neutrality returns the evaluative words found in text (lowercase, each once,
// in order of first appearance); nil when the wording is descriptive.
// Generation must never rank, recommend or take a position (design principles
// 2 and 3).
func Neutrality(text string) []string {
	words := neutralWords()
	var found []string
	for _, tok := range neutralWordPattern().FindAllString(text, -1) {
		word := strings.ToLower(tok)
		if slices.Contains(words, word) && !slices.Contains(found, word) {
			found = append(found, word)
		}
	}
	return found
}
