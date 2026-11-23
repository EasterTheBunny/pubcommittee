//go:build emulator

package main

import (
	"github.com/easterthebunny/pubcommittee/internal/data"
)

// wire has no cloud target: the accuracy report is a development tool. It must never
// import a local-only adapter (depguard, test/ci/deps.sh).
func wire(config) (data.PlatformPorts, data.Generator, error) {
	return data.PlatformPorts{}, nil, errNoCloud
}
