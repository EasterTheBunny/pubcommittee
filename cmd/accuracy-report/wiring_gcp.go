//go:build gcp

package main

import (
	"github.com/easterthebunny/pubcommittee/internal/data"
)

// wire has no cloud target: the accuracy report is a development tool that
// runs over the local data directory only.
func wire(config) (data.PlatformPorts, data.Generator, error) {
	return data.PlatformPorts{}, nil, errNoCloud
}
