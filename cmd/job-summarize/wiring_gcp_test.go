//go:build gcp

package main

import (
	"context"
	"errors"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/adapters/claude"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

var errBuilt = errors.New("claude client built")

// TestGCPGeneratorNeedsLiveClaude: only --generator claude with the live-run
// switch builds the Claude client; anything else refuses every call.
func TestGCPGeneratorNeedsLiveClaude(t *testing.T) {
	t.Parallel()
	var got claude.GoogleCloudConfig
	build := func(_ context.Context, c claude.GoogleCloudConfig) (*claude.Gen, error) {
		got = c
		return nil, errBuilt
	}
	env := func(k string) string {
		if k == "GCP_PROJECT" {
			return "p"
		}
		return ""
	}
	for _, cfg := range []config{
		{Generator: generatorFake},
		{Generator: generatorFake, Live: true},
		{Generator: generatorClaude},
	} {
		g, err := gcpGenerator(t.Context(), cfg, env, build)
		if err != nil {
			t.Fatalf("%+v: %v", cfg, err)
		}
		if _, err := g.Submit(t.Context(), nil); !errors.Is(err, errNoOfflineGenerator) {
			t.Errorf("%+v: Submit err = %v, want errNoOfflineGenerator", cfg, err)
		}
		if _, _, err := g.Collect(t.Context(), "b"); !errors.Is(err, errNoOfflineGenerator) {
			t.Errorf("%+v: Collect err = %v", cfg, err)
		}
		if _, err := g.Generate(t.Context(), data.GenRequest{}); !errors.Is(err, errNoOfflineGenerator) {
			t.Errorf("%+v: Generate err = %v", cfg, err)
		}
	}
	_, err := gcpGenerator(t.Context(), config{Generator: generatorClaude, Live: true, Model: "m"}, env, build)
	if !errors.Is(err, errBuilt) {
		t.Fatalf("live claude: err = %v, want the client to be built", err)
	}
	if got.Project != "p" || got.Model != "m" {
		t.Errorf("config = %+v", got)
	}
}
