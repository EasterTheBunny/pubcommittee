//go:build local || memory

package main

import (
	"fmt"
	"os"

	"github.com/easterthebunny/pubcommittee/internal/adapters/claude"
	"github.com/easterthebunny/pubcommittee/internal/adapters/fakegen"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

// newGenerator builds the generator named by --generator: fakegen (the
// default; no network), or the first-party Claude client with the operator's
// key from ANTHROPIC_API_KEY (live, stage L3; parseArgs already required the
// live-run switch). files holds fakegen's batches between the two phases.
func newGenerator(cfg config, files data.FileStore) (data.Generator, error) {
	if cfg.Generator == generatorClaude {
		g, err := claude.New(claude.Config{APIKey: os.Getenv("ANTHROPIC_API_KEY"), Model: cfg.Model})
		if err != nil {
			return nil, fmt.Errorf("claude: %w", err)
		}
		return g, nil
	}
	return fakegen.New(fakegen.Config{Files: files}), nil
}
