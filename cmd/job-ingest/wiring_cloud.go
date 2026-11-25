//go:build gcp || emulator

package main

import (
	"fmt"

	"github.com/easterthebunny/pubcommittee/internal/adapters/uscode"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

// localTarget is false on the cloud targets; see wiring_sources.go.
const localTarget = false

// defaultSource is govinfo on the cloud targets: the fixtures are local-only,
// so a Cloud Run job needs no --source flag.
const defaultSource = sourceGovinfo

// checkSource refuses --source fixtures: the fixture source is local-only and
// is not linked into a cloud build.
func checkSource(cfg config) error {
	if cfg.Source != sourceGovinfo {
		return fmt.Errorf("--source %q is not available on this target, only %q: %w",
			cfg.Source, sourceGovinfo, data.ErrInvalid)
	}
	return nil
}

// newLawSource builds the live Law Revision Counsel source (one request at a
// time, politeness rules of the adapter). The emulator target uses the live
// source too: it has no fixture source, and --law only fetches when a new
// release point is stored.
func newLawSource(config, data.BillFetcher) (data.LawSource, error) {
	s, err := uscode.New(uscode.Config{UserAgent: userAgent})
	if err != nil {
		return nil, fmt.Errorf("uscode: %w", err)
	}
	return s, nil
}
