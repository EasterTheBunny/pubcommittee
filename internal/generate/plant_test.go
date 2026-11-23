package generate_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/generate"
)

// testContext is the input context of the change explanation fixtures.
func testContext() data.GenContext {
	return data.GenContext{
		Lines: map[string]string{
			"s1-l1": `Section 1395 of the Social Security Act is amended by striking "$5,000" and inserting "$7,500".`,
			"s1-l2": "The Secretary shall pay 30 days after receipt.",
			"s2-l1": "Unrelated text about reports.",
		},
		Provisions: map[string]string{
			"usc/42/1395": "Payment shall not exceed $5,000.",
		},
	}
}

// testKey is the key shared by the fixtures.
func testKey(task string) data.GenKey {
	return data.GenKey{
		Task: task,
		Version: data.VersionRef{
			Bill:    data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: 1234},
			Version: "ih",
		},
		PromptVersion: "test",
		Model:         "fake-model",
	}
}

// goodOutput is a change explanation that passes every check.
func goodOutput() data.GeneratedOutput {
	return data.GeneratedOutput{
		Key:    testKey(data.TaskChangeExplanation),
		Status: data.GenReady,
		Statements: []data.Statement{
			{
				Role:         data.RoleNow,
				Text:         "Payment shall not exceed $5,000.",
				Anchors:      []string{"s1-l1"},
				ProvisionIDs: []string{"usc/42/1395"},
			},
			{
				Role:         data.RoleChanges,
				Text:         `Replaces "$5,000" with "$7,500" in section 1395; payment is due 30 days after receipt.`,
				Anchors:      []string{"s1-l1", "s1-l2"},
				ProvisionIDs: []string{"usc/42/1395"},
			},
		},
		ReviewState: data.ReviewUnreviewed,
	}
}

// reversedOutput is goodOutput with the change written as a strike and insert instruction.
func reversedOutput() data.GeneratedOutput {
	o := goodOutput()
	o.Statements[1].Text = `Section 1395 is amended by striking "$5,000" and inserting "$7,500"; payment is due 30 days after receipt.`
	return o
}

// billOutput is a bill summary whose one statement cites the second line of section 1.
func billOutput() data.GeneratedOutput {
	return data.GeneratedOutput{
		Key:    testKey(data.TaskBillSummary),
		Status: data.GenReady,
		Statements: []data.Statement{
			{Text: "The Secretary shall pay 30 days after receipt.", Anchors: []string{"s1-l2"}},
		},
		ReviewState: data.ReviewUnreviewed,
	}
}

// inputFor returns the fixture a kind is planted in.
func inputFor(kind string) data.GeneratedOutput {
	switch kind {
	case generate.PlantReversedEffect:
		return reversedOutput()
	case generate.PlantSwappedSection:
		return billOutput()
	default:
		return goodOutput()
	}
}

func TestPlantKinds(t *testing.T) {
	t.Parallel()
	want := []string{
		"changed-number", "swapped-section", "reversed-effect",
		"evaluative-wording", "invented-number", "nonexistent-anchor",
	}
	if got := generate.PlantKinds(); !slices.Equal(got, want) {
		t.Fatalf("PlantKinds() = %q, want %q", got, want)
	}
}

func TestGoodOutputPassesValidate(t *testing.T) {
	t.Parallel()
	gc := testContext()
	for name, o := range map[string]data.GeneratedOutput{"good": goodOutput(), "reversed": reversedOutput()} {
		findings := generate.Validate(data.TaskChangeExplanation, generate.ModelOutput{Statements: o.Statements}, gc)
		if len(findings) != 0 {
			t.Errorf("%s output: Validate findings = %q, want none", name, findings)
		}
	}
}

func TestPlantCaughtByCheck(t *testing.T) {
	t.Parallel()
	gc := testContext()
	for _, kind := range generate.PlantKinds() {
		check := generate.CheckOf(kind)
		if check == "" {
			continue
		}
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			out, ok := generate.Plant(kind, inputFor(kind), gc)
			if !ok {
				t.Fatalf("Plant(%q) ok = false, want true", kind)
			}
			if out.Status != data.GenReady || out.Findings != nil {
				t.Errorf("Plant(%q) status = %q, findings = %q; want ready and nil", kind, out.Status, out.Findings)
			}
			findings := generate.Validate(data.TaskChangeExplanation, generate.ModelOutput{Statements: out.Statements}, gc)
			if len(findings) == 0 {
				t.Fatalf("Plant(%q): Validate findings empty, want a finding", kind)
			}
			if !generate.Caught(check, findings) {
				t.Errorf("Plant(%q): Caught(%q) = false; findings = %q", kind, check, findings)
			}
		})
	}
}

func TestPlantRules(t *testing.T) {
	t.Parallel()
	gc := testContext()
	tests := []struct {
		name     string
		kind     string
		input    data.GeneratedOutput
		wantIdx  int
		wantText string
	}{
		{
			name:     "changed-number adds seven to the first digit run",
			kind:     generate.PlantChangedNumber,
			input:    goodOutput(),
			wantIdx:  0,
			wantText: "Payment shall not exceed $5007.",
		},
		{
			name:     "evaluative wording is appended to the first statement",
			kind:     generate.PlantEvaluative,
			input:    goodOutput(),
			wantIdx:  0,
			wantText: "Payment shall not exceed $5,000. This is a much-needed and sensible reform.",
		},
		{
			name:     "invented number is appended to the first statement",
			kind:     generate.PlantInventedNumber,
			input:    goodOutput(),
			wantIdx:  0,
			wantText: "Payment shall not exceed $5,000. It provides $987,654,321 in funding.",
		},
		{
			name:     "reversed effect swaps the quoted texts",
			kind:     generate.PlantReversedEffect,
			input:    reversedOutput(),
			wantIdx:  1,
			wantText: `Section 1395 is amended by striking "$7,500" and inserting "$5,000"; payment is due 30 days after receipt.`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, ok := generate.Plant(tt.kind, tt.input, gc)
			if !ok {
				t.Fatalf("Plant(%q) ok = false, want true", tt.kind)
			}
			if got := out.Statements[tt.wantIdx].Text; got != tt.wantText {
				t.Errorf("statement %d text = %q, want %q", tt.wantIdx, got, tt.wantText)
			}
		})
	}
}

func TestPlantBadAnchorAppendsAnchor(t *testing.T) {
	t.Parallel()
	out, ok := generate.Plant(generate.PlantBadAnchor, goodOutput(), testContext())
	if !ok {
		t.Fatal("Plant(nonexistent-anchor) ok = false, want true")
	}
	want := []string{"s1-l1", "s99-l99"}
	if got := out.Statements[0].Anchors; !slices.Equal(got, want) {
		t.Errorf("first statement anchors = %q, want %q", got, want)
	}
}

func TestPlantSwappedSection(t *testing.T) {
	t.Parallel()
	out, ok := generate.Plant(generate.PlantSwappedSection, billOutput(), testContext())
	if !ok {
		t.Fatal("Plant(swapped-section) ok = false, want true")
	}
	if got, want := out.Statements[0].Anchors, []string{"s2-l1"}; !slices.Equal(got, want) {
		t.Errorf("anchors = %q, want %q", got, want)
	}
	if len(out.Statements[0].ProvisionIDs) != 0 {
		t.Errorf("provision ids = %q, want none", out.Statements[0].ProvisionIDs)
	}
}

func TestPlantNotApplicable(t *testing.T) {
	t.Parallel()
	gc := testContext()
	noDigits := data.GeneratedOutput{Statements: []data.Statement{
		{Role: data.RoleNow, Text: "Payment is due promptly.", Anchors: []string{"s1-l2"}},
	}}
	noAnchor := data.GeneratedOutput{Statements: []data.Statement{
		{Text: "The Secretary shall pay 30 days after receipt."},
	}}
	noStatements := data.GeneratedOutput{}
	tests := []struct {
		name string
		kind string
		o    data.GeneratedOutput
	}{
		{"changed-number without digits", generate.PlantChangedNumber, noDigits},
		{"swapped-section without anchor", generate.PlantSwappedSection, noAnchor},
		{"reversed-effect without amendment", generate.PlantReversedEffect, goodOutput()},
		{"evaluative without statements", generate.PlantEvaluative, noStatements},
		{"unknown kind", "no-such-kind", goodOutput()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, ok := generate.Plant(tt.kind, tt.o, gc); ok {
				t.Errorf("Plant(%q) ok = true, want false", tt.kind)
			}
		})
	}
}

func TestPlantDoesNotMutateInput(t *testing.T) {
	t.Parallel()
	gc := testContext()
	for _, kind := range generate.PlantKinds() {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			in := inputFor(kind)
			orig := inputFor(kind)
			out, _ := generate.Plant(kind, in, gc)
			if !reflect.DeepEqual(in, orig) {
				t.Fatalf("Plant(%q) mutated its input", kind)
			}
			// The copy shares no slice or statement with the input.
			out.Statements[0].Text = "changed"
			out.Statements[0].Anchors = append(out.Statements[0].Anchors, "x")
			if !reflect.DeepEqual(in, orig) {
				t.Fatalf("Plant(%q) copy shares memory with its input", kind)
			}
		})
	}
}

func TestCaughtUnknownCheck(t *testing.T) {
	t.Parallel()
	findings := []string{"statement 1: fact not in cited lines: number 5007"}
	if generate.Caught("no-such-check", findings) {
		t.Error("Caught(unknown check) = true, want false")
	}
	if generate.Caught("anchors", findings) {
		t.Error("Caught(anchors) matched a fact-fidelity finding")
	}
	if !generate.Caught("fact-fidelity", findings) {
		t.Error("Caught(fact-fidelity) = false, want true")
	}
	if generate.CheckOf(generate.PlantSwappedSection) != "" {
		t.Errorf("CheckOf(swapped-section) = %q, want empty", generate.CheckOf(generate.PlantSwappedSection))
	}
	if strings.TrimSpace(generate.CheckOf(generate.PlantBadAnchor)) != "anchors" {
		t.Errorf("CheckOf(nonexistent-anchor) = %q, want anchors", generate.CheckOf(generate.PlantBadAnchor))
	}
}
