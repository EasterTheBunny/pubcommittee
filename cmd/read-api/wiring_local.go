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

// wire builds the public read ports on the local data directory named by
// DATA_DIR, the only storage setting. Other processes may write to it.
func wire(tel *telemetry.Telemetry) (data.PublicReadPorts, error) {
	dir := os.Getenv("DATA_DIR")
	if dir == "" {
		return data.PublicReadPorts{}, fmt.Errorf("DATA_DIR is required: %w", data.ErrInvalid)
	}
	objects, err := fsobject.New(fsobject.Config{Root: dir})
	if err != nil {
		return data.PublicReadPorts{}, err //nolint:wrapcheck // already names the adapter
	}
	docs, err := fsdocs.New(fsdocs.Config{Root: dir})
	if err != nil {
		return data.PublicReadPorts{}, err //nolint:wrapcheck // already names the adapter
	}
	ports := data.PublicReadPorts{
		BillText: objects, PublicDocs: docs, Comparisons: objects, Citations: objects, Law: objects,
		Generated: data.NewGeneratedStore(objects),
	}
	return tel.WrapPublicRead("local", ports), nil
}
