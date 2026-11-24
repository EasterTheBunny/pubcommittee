package main

import (
	"errors"
	"os"
	"path/filepath"
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

func TestRegradeContradictedFlag(t *testing.T) {
	t.Parallel()
	cfg, err := parseArgs([]string{"--regrade-contradicted"}, func(string) string { return "" })
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if !cfg.Regrade {
		t.Error("--regrade-contradicted did not set Regrade")
	}
	if _, err := parseArgs([]string{"--regrade-contradicted", "--generator", "claude"},
		func(string) string { return "" }); err == nil {
		t.Error("a live regrade without the live switch was accepted")
	}
}

// --report-only writes report.md and leaves the operator sample files alone.
func TestReportOnlyKeepsSample(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tl := &tool{cfg: config{OutDir: dir, ReportOnly: true}}
	if err := tl.writeSample([]byte("[]"), nil); err != nil {
		t.Fatalf("writeSample: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "operator-sample.json")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("operator-sample.json written with --report-only: %v", err)
	}
}
