//go:build gcp

package main

import (
	"context"
	"fmt"

	"github.com/easterthebunny/pubcommittee/internal/adapters/claude"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

// wire is a placeholder until M1-03b adds the cloud storage adapters. The
// generator side is real: Claude Models on Google Cloud with the job's
// service account (Application Default Credentials), no API key. It must
// never import a local-only adapter (depguard, test/ci/deps.sh).
func wire(_ *telemetry.Telemetry, _ config) (data.PlatformPorts, data.Generator, error) {
	if _, err := claude.NewGoogleCloud(context.Background(), claude.GoogleCloudConfig{}); err != nil {
		return data.PlatformPorts{}, nil, fmt.Errorf("claude: %w", err)
	}
	return data.PlatformPorts{}, nil, errNoCloud
}
