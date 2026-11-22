//go:build emulator

package main

import (
	"errors"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

// wire is a placeholder until M1-03b adds the emulator-backed cloud adapters.
func wire(_ *telemetry.Telemetry) (data.PublicReadPorts, error) {
	return data.PublicReadPorts{}, errors.New("emulator adapters arrive with M1-03b")
}
