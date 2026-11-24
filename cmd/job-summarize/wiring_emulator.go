//go:build emulator

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/easterthebunny/pubcommittee/internal/adapters/fakegen"
	"github.com/easterthebunny/pubcommittee/internal/cloudports"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

// wire builds the platform ports on the emulators and the offline generator:
// no model runs on the emulator target.
func wire(tel *telemetry.Telemetry, cfg config) (data.PlatformPorts, data.Generator, error) {
	p, err := cloudports.Open(context.Background(), cloudports.Emulator, os.Getenv)
	if err != nil {
		return data.PlatformPorts{}, nil, fmt.Errorf("cloud ports: %w", err)
	}
	gen := fakegen.New(fakegen.Config{Files: p.Objects})
	return tel.WrapPlatform("emulator", p.Platform()), tel.WrapGenerator(cfg.Generator, gen), nil
}
