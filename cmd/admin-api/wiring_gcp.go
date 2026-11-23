//go:build gcp

package main

import (
	"github.com/easterthebunny/pubcommittee/internal/adminapi"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

// wire is a placeholder until M1-03b adds the cloud adapters and the Google
// ID token verifier. It must never import a local-only adapter (depguard,
// test/ci/deps.sh).
func wire(_ *telemetry.Telemetry, _ func(string) string) (adminapi.Deps, error) {
	return adminapi.Deps{}, errNoCloud
}
