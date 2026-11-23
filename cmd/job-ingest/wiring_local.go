//go:build local

package main

import (
	"fmt"
	"os"

	"github.com/easterthebunny/pubcommittee/internal/adapters/fsdocs"
	"github.com/easterthebunny/pubcommittee/internal/adapters/fsobject"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

// wire builds the platform ports on the local data directory named by
// DATA_DIR, the only storage setting, and the bill source named by --source.
func wire(tel *telemetry.Telemetry, cfg config) (data.PlatformPorts, data.BillFetcher, error) {
	dir := os.Getenv("DATA_DIR")
	if dir == "" {
		return data.PlatformPorts{}, nil, fmt.Errorf("DATA_DIR is required: %w", data.ErrInvalid)
	}
	objects, err := fsobject.New(fsobject.Config{Root: dir})
	if err != nil {
		return data.PlatformPorts{}, nil, fmt.Errorf("objects: %w", err)
	}
	docs, err := fsdocs.New(fsdocs.Config{Root: dir})
	if err != nil {
		return data.PlatformPorts{}, nil, fmt.Errorf("docs: %w", err)
	}
	fetcher, err := newFetcher(cfg)
	if err != nil {
		return data.PlatformPorts{}, nil, err
	}
	var ports data.PlatformPorts
	ports.BillText, ports.PublicDocs = objects, docs
	ports.Comparisons, ports.Citations, ports.Law = objects, objects, objects
	ports.ComparisonStore, ports.CitationStore, ports.LawStore = objects, objects, objects
	ports.PlatformDocs, ports.Moderation, ports.Publisher = docs, docs, objects
	return tel.WrapPlatform("local", ports), fetcher, nil
}
