package main

import (
	"errors"
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

// The report calls no model unless the live switch is set and the claude
// generator is named.
func TestLiveGeneratorNeedsSwitch(t *testing.T) {
	t.Parallel()
	if _, err := parseArgs(nil, env(nil)); err != nil {
		t.Fatalf("default args: %v", err)
	}
	if _, err := parseArgs([]string{"--generator", "claude"}, env(nil)); !errors.Is(err, errLive) {
		t.Fatalf("err = %v, want errLive", err)
	}
}

func TestGraderMustDifferFromGenerator(t *testing.T) {
	t.Parallel()
	if _, err := parseArgs([]string{"--model", "m", "--grader-model", "m"}, env(nil)); !errors.Is(err, errSameModel) {
		t.Fatalf("err = %v, want errSameModel", err)
	}
}
