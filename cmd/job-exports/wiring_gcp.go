//go:build gcp

package main

import (
	"errors"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

var errNoCloud = errors.New("cloud adapters arrive with M1-03b")

// wire is a placeholder until M1-03b adds the cloud adapters. It must never
// import a local-only adapter (depguard, test/ci/deps.sh).
func wire(_ *telemetry.Telemetry) (data.PlatformPorts, error) {
	return data.PlatformPorts{}, errNoCloud
}
