//go:build gcp

package main

import (
	"errors"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

// wire is a placeholder until M1-03b adds the cloud adapters (firestore,
// gcs). It must never import a local-only adapter (depguard, test/ci/deps.sh).
func wire(_ *telemetry.Telemetry) (data.PublicReadPorts, error) {
	return data.PublicReadPorts{}, errors.New("gcp adapters arrive with M1-03b")
}
