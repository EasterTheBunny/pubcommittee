//go:build gcp

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/easterthebunny/pubcommittee/internal/cloudports"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

// wire builds the public read ports on the cloud adapters (Firestore and Cloud
// Storage; their emulators on the emulator target). It never imports a
// local-only adapter (depguard, test/ci/deps.sh).
func wire(tel *telemetry.Telemetry) (data.PublicReadPorts, error) {
	p, err := cloudports.Open(context.Background(), cloudports.GCP, os.Getenv)
	if err != nil {
		return data.PublicReadPorts{}, fmt.Errorf("cloud ports: %w", err)
	}
	return tel.WrapPublicRead("gcp", p.Public()), nil
}
