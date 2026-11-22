//go:build memory

package main

import (
	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

// wire builds the public read ports from process memory. Nothing persists.
func wire(tel *telemetry.Telemetry) (data.PublicReadPorts, error) {
	return tel.WrapPublicRead("memory", memory.New().PublicReadPorts()), nil
}
