//go:build gcp

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/easterthebunny/pubcommittee/internal/adapters/claude"
	"github.com/easterthebunny/pubcommittee/internal/cloudports"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

// wire builds the platform ports on the cloud adapters and the generator:
// Claude Models on Google Cloud with the job's service account (Application
// Default Credentials), no API key. It never imports a local-only adapter
// (depguard, test/ci/deps.sh).
func wire(tel *telemetry.Telemetry, cfg config) (data.PlatformPorts, data.Generator, error) {
	ctx := context.Background()
	gen, err := claude.NewGoogleCloud(ctx, claude.GoogleCloudConfig{Model: cfg.Model})
	if err != nil {
		return data.PlatformPorts{}, nil, fmt.Errorf("claude: %w", err)
	}
	p, err := cloudports.Open(ctx, cloudports.GCP, os.Getenv)
	if err != nil {
		return data.PlatformPorts{}, nil, fmt.Errorf("cloud ports: %w", err)
	}
	return tel.WrapPlatform("gcp", p.Platform()), tel.WrapGenerator(cfg.Generator, gen), nil
}
