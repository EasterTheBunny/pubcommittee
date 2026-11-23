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
// DATA_DIR, the only storage setting.
func wire(tel *telemetry.Telemetry) (data.PlatformPorts, error) {
	dir := os.Getenv("DATA_DIR")
	if dir == "" {
		return data.PlatformPorts{}, fmt.Errorf("DATA_DIR is required: %w", data.ErrInvalid)
	}
	objects, err := fsobject.New(fsobject.Config{Root: dir})
	if err != nil {
		return data.PlatformPorts{}, fmt.Errorf("objects: %w", err)
	}
	docs, err := fsdocs.New(fsdocs.Config{Root: dir})
	if err != nil {
		return data.PlatformPorts{}, fmt.Errorf("docs: %w", err)
	}
	var ports data.PlatformPorts
	ports.BillText, ports.PublicDocs, ports.Publisher = objects, docs, objects
	ports.Citations, ports.Law = objects, objects
	ports.FeedStore, ports.ExportStore = objects, objects
	return tel.WrapPlatform("local", ports), nil
}
