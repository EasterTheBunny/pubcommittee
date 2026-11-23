package generate

import (
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// strikeInstruction is one strike instruction of an amendment: the text removed and, when the
// instruction also inserts, the text inserted in its place (empty for a strike only).
type strikeInstruction struct {
	removed  string
	inserted string
}

// AmendmentConsistency parses the amendment instructions in cited (the text of the cited bill lines, joined by
// newlines) and checks that explanation, the statement text of the explanation, describes the same target, removed
// text and inserted text. It returns one finding per inconsistency; nil when cited holds no amendment instruction or
// every instruction is described consistently. provisionIDs are the provision ids the statement cites (for example
// "usc/42/1395").
func AmendmentConsistency(explanation, cited string, provisionIDs []string) []string {
	text := normText(cited)
	expl := norm(explanation)
	strikes := strikeInstructions(text)
	adds := addRE().MatchString(text)
	repeals := repealRE().MatchString(text)

	return slices.Concat(
		checkStrikes(expl, strikes),
		checkAdd(expl, adds, repeals, len(strikes) > 0),
		checkRepeal(expl, repeals, adds, len(strikes) > 0),
		checkTargets(expl, text, provisionIDs),
	)
}

// strikeRE matches a strike instruction; the second group is the inserted text when the instruction also inserts.
func strikeRE() *regexp.Regexp {
	return regexp.MustCompile(`(?i)striking\s+"([^"]+)"(?:\s+and\s+inserting\s+"([^"]+)")?`)
}

// addRE matches an instruction that adds text without removing any.
func addRE() *regexp.Regexp {
	return regexp.MustCompile(`(?i)\b(?:adding at the end|inserting after)\b`)
}

// repealRE matches a repeal instruction.
func repealRE() *regexp.Regexp {
	return regexp.MustCompile(`(?i)\brepeal(?:s|ed)?\b`)
}

// targetRE matches the section an instruction amends; the group is the section number.
func targetRE() *regexp.Regexp {
	return regexp.MustCompile(`(?i)section\s+(\d+[A-Za-z0-9-]*)[^.]{0,120}?(?:is|are)\s+amended`)
}

// normText maps curly and doubled quotes to straight double quotes and collapses whitespace.
func normText(s string) string {
	quotes := strings.NewReplacer("“", `"`, "”", `"`, "‘", `"`, "’", `"`, "``", `"`, "''", `"`)
	return strings.Join(strings.Fields(quotes.Replace(s)), " ")
}

// norm is normText in lower case: the form the explanation and the quoted texts are compared in.
func norm(s string) string {
	return strings.ToLower(normText(s))
}

// strikeInstructions returns every strike instruction in the normalized cited text.
func strikeInstructions(text string) []strikeInstruction {
	matches := strikeRE().FindAllStringSubmatch(text, -1)
	out := make([]strikeInstruction, 0, len(matches))
	for _, m := range matches {
		out = append(out, strikeInstruction{removed: m[1], inserted: m[2]})
	}
	return out
}

// checkStrikes checks every strike instruction against the explanation.
func checkStrikes(expl string, strikes []strikeInstruction) []string {
	var findings []string
	for _, s := range strikes {
		findings = slices.Concat(findings, checkStrike(expl, s))
	}
	return findings
}

// checkStrike checks that the explanation names the removed text, and the inserted text when there is one, and
// names the removed text first. A text is named when the explanation quotes it or, failing that, describes it in
// other words (see paraphrases); the order is checked only for quoted texts.
func checkStrike(expl string, s strikeInstruction) []string {
	removedAt := strings.Index(expl, norm(s.removed))
	removedNamed := removedAt >= 0 || paraphrases(expl, s.removed)
	var findings []string
	if !removedNamed {
		findings = append(findings, `removed text "`+s.removed+`" not named`)
	}
	if s.inserted == "" {
		return findings
	}
	insertedAt := strings.Index(expl, norm(s.inserted))
	if insertedAt < 0 && !paraphrases(expl, s.inserted) {
		return append(findings, `inserted text "`+s.inserted+`" not named`)
	}
	if removedAt >= 0 && insertedAt >= 0 && removedAt > insertedAt {
		findings = append(findings, "inserted text named before removed text")
	}
	return findings
}

// Thresholds of paraphrases: the share of the quoted text's words one sentence of the explanation must use, and the
// least number of them. Chosen on the stored live outputs: a sentence that renames the amended wording keeps half of
// its words or more, while one that only repeats the section number and the name of the act keeps fewer. The words
// must come from one sentence: words spread over a long explanation match by chance.
const (
	paraphraseShare = 0.5
	paraphraseWords = 2
)

// wordRE matches the words and the lettered subsection marks ("(b)") of a text.
var wordRE = regexp.MustCompile(`[a-z0-9]+`)

// sentenceRE splits an explanation into sentences and list items.
var sentenceRE = regexp.MustCompile(`[.!?;]\s|\n`)

// isFiller reports whether w carries no meaning of its own in a quoted amendment text: articles, linking words and
// the names of the parts of a statute.
func isFiller(w string) bool {
	return slices.Contains([]string{
		"the", "of", "and", "in", "to", "a", "an", "as", "is", "are", "be", "by", "for", "has", "have", "had", "that",
		"such", "this", "with", "or", "not", "does", "do", "on", "at", "from", "under", "made", "s", "insofar", "it",
		"its", "which", "section", "subsection", "paragraph", "subparagraph", "clause", "act", "title", "chapter",
	}, w)
}

// paraphrases reports whether a sentence of the explanation (normalized) describes the quoted text in other words:
// it uses at least paraphraseWords of the quoted text's meaningful words and at least paraphraseShare of them.
// Plurals and possessives count as the same word.
func paraphrases(expl, quoted string) bool {
	var words []string
	seen := map[string]bool{}
	for _, w := range wordRE.FindAllString(norm(quoted), -1) {
		if st := stem(w); !isFiller(w) && !seen[st] {
			seen[st] = true
			words = append(words, st)
		}
	}
	for _, sentence := range sentenceRE.Split(expl, -1) {
		have := map[string]bool{}
		for _, w := range wordRE.FindAllString(sentence, -1) {
			have[stem(w)] = true
		}
		matched := 0
		for _, w := range words {
			if have[w] {
				matched++
			}
		}
		if matched >= paraphraseWords && float64(matched) >= paraphraseShare*float64(len(words)) {
			return true
		}
	}
	return false
}

// stem folds a plural to its singular: "studies" and "study", "hearings" and "hearing".
func stem(w string) string {
	switch {
	case strings.HasSuffix(w, "ies") && len(w) > 4:
		return w[:len(w)-3] + "y"
	case strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss") && len(w) > 3:
		return w[:len(w)-1]
	}
	return w
}

// checkAdd reports an explanation that describes a removal for an instruction that only adds text.
func checkAdd(expl string, adds, repeals, strikes bool) []string {
	if !adds || repeals || strikes {
		return nil
	}
	if !hasWord(expl, "repeal", "repeals", "repealed", "eliminate", "eliminates", "removes", "abolish", "abolishes") {
		return nil
	}
	return []string{"explanation describes removal but the instruction adds text"}
}

// checkRepeal checks that an instruction which only repeals is described as a removal and not as an addition.
func checkRepeal(expl string, repeals, adds, strikes bool) []string {
	if !repeals || adds || strikes {
		return nil
	}
	var findings []string
	if !hasWord(expl, "repeal", "repeals", "repealed", "eliminate", "eliminates", "remove", "removes", "removed",
		"strike", "strikes", "abolish", "abolishes") {
		findings = append(findings, "repeal not described")
	}
	if hasWord(expl, "add", "adds", "added", "establish", "establishes", "create", "creates", "expand", "expands") {
		findings = append(findings, "explanation describes an addition but the instruction repeals")
	}
	return findings
}

// checkTargets checks that every section the cited text amends is named by the explanation or a provision id.
func checkTargets(expl, text string, provisionIDs []string) []string {
	var findings []string
	for _, m := range targetRE().FindAllStringSubmatch(text, -1) {
		if !namedTarget(expl, provisionIDs, norm(m[1])) {
			findings = slices.Concat(findings, []string{"target section " + m[1] + " not named"})
		}
	}
	return findings
}

// namedTarget reports whether the normalized section number n appears in the explanation or in a provision id.
func namedTarget(expl string, provisionIDs []string, n string) bool {
	if strings.Contains(expl, n) {
		return true
	}
	for _, id := range provisionIDs {
		if strings.Contains(norm(id), n) {
			return true
		}
	}
	return false
}

// hasWord reports whether the lower-case text contains one of words as a whole word.
func hasWord(text string, words ...string) bool {
	fields := strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) })
	for _, f := range fields {
		if slices.Contains(words, f) {
			return true
		}
	}
	return false
}
