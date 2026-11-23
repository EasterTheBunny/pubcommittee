//go:build gcp || emulator

package main

import (
	"errors"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

var errNoCloud = errors.New("cloud adapters arrive with M1-03b")

// localTarget is false on the cloud targets; see wiring_sources.go.
const localTarget = false

// newLawSource has no cloud adapter yet.
func newLawSource(config, data.BillFetcher) (data.LawSource, error) { return nil, errNoCloud }
