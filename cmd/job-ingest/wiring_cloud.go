//go:build gcp || emulator

package main

import "errors"

var errNoCloud = errors.New("cloud adapters arrive with M1-03b")

// localTarget is false on the cloud targets; see wiring_sources.go.
const localTarget = false
