//go:build emulator

package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/easterthebunny/pubcommittee/internal/adapters/govinfo"
	"github.com/easterthebunny/pubcommittee/internal/cloudports"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

// wire builds the platform ports on the cloud adapters and the govinfo bill
// source (the fixture source is local-only). It never imports a local-only
// adapter (depguard, test/ci/deps.sh).
func wire(tel *telemetry.Telemetry, _ config) (data.PlatformPorts, data.BillFetcher, error) {
	p, err := cloudports.Open(context.Background(), cloudports.Emulator, os.Getenv)
	if err != nil {
		return data.PlatformPorts{}, nil, fmt.Errorf("cloud ports: %w", err)
	}
	src, err := govinfo.New(govinfo.Config{})
	if err != nil {
		return data.PlatformPorts{}, nil, errors.Join(fmt.Errorf("govinfo: %w", err), p.Close())
	}
	return tel.WrapPlatform("emulator", p.Platform()), src, nil
}
