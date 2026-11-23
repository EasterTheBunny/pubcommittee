//go:build memory

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/readapi/seed"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

// wire builds the public read ports from process memory. Nothing persists.
// SEED_FIXTURES=1 loads the invented fixture bills so contract tests have
// data to read; without it the store starts empty.
func wire(tel *telemetry.Telemetry) (data.PublicReadPorts, error) {
	store := memory.New()
	if os.Getenv("SEED_FIXTURES") == "1" {
		if err := seed.Apply(context.Background(), store.PlatformPorts()); err != nil {
			return data.PublicReadPorts{}, fmt.Errorf("seed fixtures: %w", err)
		}
	}
	return tel.WrapPublicRead("memory", store.PublicReadPorts()), nil
}
