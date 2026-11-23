//go:build local || memory

package main

import (
	"fmt"
	"os"

	"github.com/easterthebunny/pubcommittee/internal/adapters/fixturesrc"
	"github.com/easterthebunny/pubcommittee/internal/adapters/govinfo"
	"github.com/easterthebunny/pubcommittee/internal/adapters/uscode"
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

// localTarget is true where the data directory is local: --publish-policy
// only writes there. A cloud policy version is published by an operator.
const localTarget = true

// newLawSource builds the existing-law source: the example sections on disk
// (the fixtures bill source also holds them), or the live Law Revision Counsel
// service (one request at a time, politeness rules of the adapter).
func newLawSource(cfg config, fetcher data.BillFetcher) (data.LawSource, error) {
	if cfg.Source == sourceGovinfo {
		s, err := uscode.New(uscode.Config{UserAgent: "pubcommittee-dev/0 (civic bill reader)"})
		if err != nil {
			return nil, fmt.Errorf("uscode: %w", err)
		}
		return s, nil
	}
	s, ok := fetcher.(*fixturesrc.Source)
	if !ok {
		return nil, fmt.Errorf("law source for %q: %w", cfg.Source, data.ErrInvalid)
	}
	return s, nil
}
