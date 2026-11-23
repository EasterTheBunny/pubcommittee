package generate

import (
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// Kinds of planted errors (architecture § Accuracy checks).
const (
	PlantChangedNumber  = "changed-number"     // a number in a statement is changed
	PlantSwappedSection = "swapped-section"    // a statement cites lines of an unrelated section
	PlantReversedEffect = "reversed-effect"    // the effect of an amendment is reversed
	PlantEvaluative     = "evaluative-wording" // an evaluative sentence is added
	PlantInventedNumber = "invented-number"    // an amount that is in no cited line is added
	PlantBadAnchor      = "nonexistent-anchor" // a statement cites an anchor that is not in the input
)

// Texts added by the wording and number kinds.
const (
	evaluativeSentence = " This is a much-needed and sensible reform."
	inventedSentence   = " It provides $987,654,321 in funding."
	badAnchor          = "s99-l99"
	changedNumberDelta = 7
)

// PlantKinds lists every kind in the order above.
func PlantKinds() []string {
	return []string{
		PlantChangedNumber,
		PlantSwappedSection,
		PlantReversedEffect,
		PlantEvaluative,
		PlantInventedNumber,
		PlantBadAnchor,
	}
}

// Plant returns a deep copy of o with one error of kind injected into one
// statement, and ok=false when o has no statement the error can be planted in.
// gc is the input context of o.
func Plant(kind string, o data.GeneratedOutput, gc data.GenContext) (data.GeneratedOutput, bool) {
	out := cloneOutput(o)
	out.Status = data.GenReady
	out.Findings = nil
	var ok bool
	switch kind {
	case PlantChangedNumber:
		ok = plantChangedNumber(out.Statements)
	case PlantSwappedSection:
		ok = plantSwappedSection(out.Statements, gc)
	case PlantReversedEffect:
		ok = plantReversedEffect(out.Statements)
	case PlantEvaluative:
		ok = appendToFirst(out.Statements, evaluativeSentence)
	case PlantInventedNumber:
		ok = appendToFirst(out.Statements, inventedSentence)
	case PlantBadAnchor:
		ok = appendAnchor(out.Statements, badAnchor)
	}
	return out, ok
}

// CheckOf names the deterministic check that should catch the kind:
// "fact-fidelity", "amendment-consistency", "neutral-wording" or "anchors";
// "" for PlantSwappedSection, which only the grounding grader can catch.
func CheckOf(kind string) string {
	switch kind {
	case PlantChangedNumber, PlantInventedNumber:
		return "fact-fidelity"
	case PlantReversedEffect:
		return "amendment-consistency"
	case PlantEvaluative:
		return "neutral-wording"
	case PlantBadAnchor:
		return "anchors"
	default:
		return ""
	}
}

// Caught reports whether findings (as returned by Validate) contain a finding of the named check.
func Caught(check string, findings []string) bool {
	var needles []string
	switch check {
	case "fact-fidelity":
		needles = []string{"fact not in cited lines"}
	case "amendment-consistency":
		needles = []string{"amendment consistency"}
	case "neutral-wording":
		needles = []string{"evaluative wording"}
	case "anchors":
		needles = []string{"anchor", "not in the input"}
	default:
		return false
	}
	return slices.ContainsFunc(findings, func(f string) bool {
		return slices.ContainsFunc(needles, func(n string) bool { return strings.Contains(f, n) })
	})
}

// cloneOutput copies o so that the copy shares no slice with it.
func cloneOutput(o data.GeneratedOutput) data.GeneratedOutput {
	c := o
	c.Findings = slices.Clone(o.Findings)
	c.Section = slices.Clone(o.Section)
	c.Statements = make([]data.Statement, len(o.Statements))
	for i, s := range o.Statements {
		c.Statements[i] = s
		c.Statements[i].Anchors = slices.Clone(s.Anchors)
		c.Statements[i].ProvisionIDs = slices.Clone(s.ProvisionIDs)
	}
	return c
}

// eligibleChange reports whether a statement may carry the effect of an amendment.
func eligibleChange(s data.Statement) bool {
	return s.Role == data.RoleChanges || s.Role == ""
}

// plantChangedNumber replaces the first digit run of the first statement that
// has one by its value plus changedNumberDelta.
func plantChangedNumber(stmts []data.Statement) bool {
	re := regexp.MustCompile(`\d+(?:,\d{3})*`)
	for i, s := range stmts {
		loc := re.FindStringIndex(s.Text)
		if loc == nil {
			continue
		}
		n, err := strconv.Atoi(strings.ReplaceAll(s.Text[loc[0]:loc[1]], ",", ""))
		if err != nil {
			continue
		}
		stmts[i].Text = s.Text[:loc[0]] + strconv.Itoa(n+changedNumberDelta) + s.Text[loc[1]:]
		return true
	}
	return false
}

// sectionOf returns the section part of an anchor: everything before "-l".
func sectionOf(anchor string) string {
	before, _, found := strings.CutLast(anchor, "-l")
	if !found {
		return anchor
	}
	return before
}

// plantSwappedSection makes the first statement cite the first line, in key order, of another section.
func plantSwappedSection(stmts []data.Statement, gc data.GenContext) bool {
	if len(stmts) == 0 || len(stmts[0].Anchors) == 0 {
		return false
	}
	own := sectionOf(stmts[0].Anchors[0])
	for _, k := range slices.Sorted(maps.Keys(gc.Lines)) {
		if strings.HasPrefix(k, PrevPrefix) || sectionOf(k) == own {
			continue
		}
		stmts[0].Anchors = []string{k}
		stmts[0].ProvisionIDs = nil
		return true
	}
	return false
}

// plantReversedEffect reverses the effect of the first amendment in the statements, in this order of preference:
// a strike and insert whose quoted texts are swapped, "repealed" changed to "added", "adding" changed to "repealing".
func plantReversedEffect(stmts []data.Statement) bool {
	return plantStrikeSwap(stmts) ||
		plantWord(stmts, `\brepealed\b`, "added") ||
		plantWord(stmts, `\badding\b`, "repealing")
}

// plantStrikeSwap swaps the two quoted texts of the first striking and inserting instruction.
func plantStrikeSwap(stmts []data.Statement) bool {
	re := regexp.MustCompile(`(?i)striking\s+(["“'\x60]+)(.+?)(["”']+)\s+and\s+inserting\s+(["“'\x60]+)(.+?)(["”']+)`)
	for i, s := range stmts {
		if !eligibleChange(s) {
			continue
		}
		idx := re.FindStringSubmatchIndex(s.Text)
		if idx == nil {
			continue
		}
		// Groups 2 (removed text) and 5 (inserted text) trade places.
		t := s.Text
		stmts[i].Text = t[:idx[4]] + t[idx[10]:idx[11]] + t[idx[5]:idx[10]] + t[idx[4]:idx[5]] + t[idx[11]:]
		return true
	}
	return false
}

// plantWord replaces the first match of pattern in the first eligible statement that has one by repl.
func plantWord(stmts []data.Statement, pattern, repl string) bool {
	re := regexp.MustCompile(pattern)
	for i, s := range stmts {
		if !eligibleChange(s) {
			continue
		}
		loc := re.FindStringIndex(s.Text)
		if loc == nil {
			continue
		}
		stmts[i].Text = s.Text[:loc[0]] + repl + s.Text[loc[1]:]
		return true
	}
	return false
}

// appendToFirst appends suffix to the text of the first statement.
func appendToFirst(stmts []data.Statement, suffix string) bool {
	if len(stmts) == 0 {
		return false
	}
	stmts[0].Text += suffix
	return true
}

// appendAnchor appends anchor to the anchors of the first statement.
func appendAnchor(stmts []data.Statement, anchor string) bool {
	if len(stmts) == 0 {
		return false
	}
	stmts[0].Anchors = append(stmts[0].Anchors, anchor)
	return true
}
