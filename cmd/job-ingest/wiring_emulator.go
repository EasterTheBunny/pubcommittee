//go:build emulator

package main

import (
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

// wire is a placeholder until M1-03b adds the cloud adapters. It must never
// import a local-only adapter (depguard, test/ci/deps.sh).
func wire(_ *telemetry.Telemetry, _ config) (data.PlatformPorts, data.BillFetcher, error) {
	return data.PlatformPorts{}, nil, errNoCloud
}
