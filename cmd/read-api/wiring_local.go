//go:build local

package main

import (
	"errors"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

// wire is a placeholder until M1-03a adds the filesystem adapters fsobject
// and fsdocs; it imports no adapter yet.
func wire(_ *telemetry.Telemetry) (data.PublicReadPorts, error) {
	return data.PublicReadPorts{}, errors.New("local adapters arrive with M1-03a")
}
