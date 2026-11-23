//go:build memory

package main

import (
	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

// wire builds the platform ports from process memory. Nothing persists, so a
// run only shows what a submit would plan.
func wire(tel *telemetry.Telemetry, cfg config) (data.PlatformPorts, data.Generator, error) {
	store := memory.New()
	gen, err := newGenerator(cfg, store)
	if err != nil {
		return data.PlatformPorts{}, nil, err
	}
	return tel.WrapPlatform("memory", store.PlatformPorts()), tel.WrapGenerator(cfg.Generator, gen), nil
}
