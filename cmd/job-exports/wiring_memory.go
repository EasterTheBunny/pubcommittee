//go:build memory

package main

import (
	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

// wire builds the platform ports from process memory. Nothing persists, so a
// run only shows what a build would do.
func wire(tel *telemetry.Telemetry) (data.PlatformPorts, error) {
	return tel.WrapPlatform("memory", memory.New().PlatformPorts()), nil
}
