package main

import (
	"errors"
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

// The default generator is fakegen, so a run with no flags for it never
// reaches a model.
func TestDefaultGeneratorIsFake(t *testing.T) {
	t.Parallel()
	cfg, _, err := parseArgs([]string{"--collect"}, env(nil))
	if err != nil || cfg.Generator != generatorFake {
		t.Fatalf("parseArgs = %+v, %v", cfg, err)
	}
}

// A live model run needs the explicit switch; CI never sets it.
func TestLiveGeneratorNeedsSwitch(t *testing.T) {
	t.Parallel()
	_, _, err := parseArgs([]string{"--collect", "--generator", "claude"}, env(nil))
	if !errors.Is(err, errLive) {
		t.Fatalf("err = %v, want errLive", err)
	}
	if _, _, err := parseArgs([]string{"--collect", "--generator", "claude"}, env(map[string]string{liveSwitch: "1"})); err != nil {
		t.Fatalf("with the switch: %v", err)
	}
}

func TestModeAndBillRules(t *testing.T) {
	t.Parallel()
	for name, args := range map[string][]string{
		"no mode":                 {},
		"both modes":              {"--submit", "--collect"},
		"no bill":                 {"--submit"},
		"bad bill":                {"--submit", "--bill", "nope"},
		"bad backend":             {"--collect", "--generator", "x"},
		"revalidate with submit":  {"--revalidate", "--submit", "--bill", "us-119-hr-1"},
		"revalidate with no bill": {"--revalidate"},
	} {
		if _, _, err := parseArgs(args, env(nil)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// Revalidation and retry are modes of the same job; neither needs the live switch.
func TestRevalidateAndRetryFlags(t *testing.T) {
	t.Parallel()
	for name, args := range map[string][]string{
		"revalidate": {"--revalidate", "--bill", "us-119-hr-4313"},
		"retry":      {"--submit", "--retry", "--dry-run", "--bill", "us-119-hr-4313"},
	} {
		if _, _, err := parseArgs(args, env(nil)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
