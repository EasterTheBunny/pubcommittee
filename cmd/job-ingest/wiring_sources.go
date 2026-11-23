//go:build local || memory

package main

import (
	"fmt"
	"os"

	"github.com/easterthebunny/pubcommittee/internal/adapters/fixturesrc"
	"github.com/easterthebunny/pubcommittee/internal/adapters/govinfo"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

// newFetcher builds the bill source named by --source: the example data on
// disk, or the live govinfo service (one request at a time, 2 s apart).
func newFetcher(cfg config) (data.BillFetcher, error) {
	if cfg.Source == sourceGovinfo {
		s, err := govinfo.New(govinfo.Config{})
		if err != nil {
			return nil, fmt.Errorf("govinfo: %w", err)
		}
		return s, nil
	}
	s, err := fixturesrc.New(fixturesrc.Config{FS: os.DirFS(cfg.FixturesDir)})
	if err != nil {
		return nil, fmt.Errorf("fixturesrc: %w", err)
	}
	return s, nil
}
