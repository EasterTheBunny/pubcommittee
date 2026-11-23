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
// names the removed text first.
func checkStrike(expl string, s strikeInstruction) []string {
	removedAt := strings.Index(expl, norm(s.removed))
	var findings []string
	if removedAt < 0 {
		findings = append(findings, `removed text "`+s.removed+`" not named`)
	}
	if s.inserted == "" {
		return findings
	}
	insertedAt := strings.Index(expl, norm(s.inserted))
	if insertedAt < 0 {
		return append(findings, `inserted text "`+s.inserted+`" not named`)
	}
	if removedAt >= 0 && removedAt > insertedAt {
		findings = append(findings, "inserted text named before removed text")
	}
	return findings
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
