//go:build emulator

package main

import (
	"errors"

	"github.com/easterthebunny/pubcommittee/internal/adminapi"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

// errNoCloud is returned by the emulator target, which has no Google ID token
// verifier; the gcp target wires the real one.
var errNoCloud = errors.New("the emulator target has no Google ID token verifier; use gcp")

// wire is a placeholder on the emulator target. It must never import a
// local-only adapter (depguard, test/ci/deps.sh).
func wire(_ *telemetry.Telemetry, _ func(string) string) (adminapi.Deps, error) {
	return adminapi.Deps{}, errNoCloud
}
