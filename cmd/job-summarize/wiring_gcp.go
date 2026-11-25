//go:build gcp

package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/easterthebunny/pubcommittee/internal/adapters/claude"
	"github.com/easterthebunny/pubcommittee/internal/cloudports"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

// errNoOfflineGenerator: the gcp build has no fake generator (local-only).
var errNoOfflineGenerator = errors.New(
	"the gcp build has no offline generator: use --dry-run, or --generator claude with " + liveSwitch + "=1")

// wire builds the platform ports on the cloud adapters and the generator.
// With --generator claude (which parseArgs allows only with the live-run
// switch): Claude Models on Google Cloud with the job's service account
// (Application Default Credentials), no API key. Otherwise a generator that
// refuses every call, so only a dry run works and no model is ever reached.
// It never imports a local-only adapter (depguard, test/ci/deps.sh).
func wire(tel *telemetry.Telemetry, cfg config) (data.PlatformPorts, data.Generator, error) {
	ctx := context.Background()
	gen, err := gcpGenerator(ctx, cfg, os.Getenv, claude.NewGoogleCloud)
	if err != nil {
		return data.PlatformPorts{}, nil, err
	}
	p, err := cloudports.Open(ctx, cloudports.GCP, os.Getenv)
	if err != nil {
		return data.PlatformPorts{}, nil, fmt.Errorf("cloud ports: %w", err)
	}
	return tel.WrapPlatform("gcp", p.Platform()), tel.WrapGenerator(cfg.Generator, gen), nil
}

// newClaudeFunc builds the Claude client; a parameter so tests never build one.
type newClaudeFunc func(context.Context, claude.GoogleCloudConfig) (*claude.Gen, error)

// gcpGenerator picks the generator by --generator. The project is GCP_PROJECT;
// the workspace comes from ANTHROPIC_GOOGLE_CLOUD_WORKSPACE_ID (read by the SDK).
func gcpGenerator(
	ctx context.Context, cfg config, getenv func(string) string, newClaude newClaudeFunc,
) (data.Generator, error) {
	if cfg.Generator != generatorClaude || !cfg.Live {
		return refusingGenerator{}, nil
	}
	g, err := newClaude(ctx, claude.GoogleCloudConfig{Project: getenv("GCP_PROJECT"), Model: cfg.Model})
	if err != nil {
		return nil, fmt.Errorf("claude: %w", err)
	}
	return g, nil
}

// refusingGenerator answers every call with errNoOfflineGenerator.
type refusingGenerator struct{}

func (refusingGenerator) Submit(context.Context, []data.GenRequest) (string, error) {
	return "", errNoOfflineGenerator
}

func (refusingGenerator) Collect(context.Context, string) ([]data.GenResult, bool, error) {
	return nil, false, errNoOfflineGenerator
}

func (refusingGenerator) Generate(context.Context, data.GenRequest) (data.GenResult, error) {
	return data.GenResult{}, errNoOfflineGenerator
}
