//go:build emulator

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/easterthebunny/pubcommittee/internal/cloudports"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

// wire builds the platform ports on the cloud adapters. It never imports a
// local-only adapter (depguard, test/ci/deps.sh).
func wire(tel *telemetry.Telemetry) (data.PlatformPorts, error) {
	p, err := cloudports.Open(context.Background(), cloudports.Emulator, os.Getenv)
	if err != nil {
		return data.PlatformPorts{}, fmt.Errorf("cloud ports: %w", err)
	}
	return tel.WrapPlatform("emulator", p.Platform()), nil
}
