//go:build memory

package main

import (
	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

// wire builds the platform ports from process memory. Nothing persists, so a
// run only shows what a pull would do.
func wire(tel *telemetry.Telemetry, cfg config) (data.PlatformPorts, data.BillFetcher, error) {
	fetcher, err := newFetcher(cfg)
	if err != nil {
		return data.PlatformPorts{}, nil, err
	}
	return tel.WrapPlatform("memory", memory.New().PlatformPorts()), fetcher, nil
}
