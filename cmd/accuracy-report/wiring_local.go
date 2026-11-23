//go:build local

package main

import (
	"fmt"
	"os"

	"github.com/easterthebunny/pubcommittee/internal/adapters/fsdocs"
	"github.com/easterthebunny/pubcommittee/internal/adapters/fsobject"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

// wire builds the ports on the local data directory named by DATA_DIR and
// the generator named by --generator.
func wire(cfg config) (data.PlatformPorts, data.Generator, error) {
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
	gen, err := newGenerator(cfg, objects)
	if err != nil {
		return data.PlatformPorts{}, nil, err
	}
	var ports data.PlatformPorts
	ports.BillText, ports.PublicDocs = objects, docs
	ports.Comparisons, ports.Citations, ports.Law = objects, objects, objects
	store := data.NewGeneratedStore(objects)
	ports.Generated, ports.GeneratedStore = store, store
	ports.PlatformDocs, ports.Moderation, ports.Publisher = docs, docs, objects
	return ports, gen, nil
}
