//go:build local

package main

import (
	"fmt"

	"github.com/easterthebunny/pubcommittee/internal/adapters/fsdocs"
	"github.com/easterthebunny/pubcommittee/internal/adapters/fsobject"
	"github.com/easterthebunny/pubcommittee/internal/adapters/localauth"
	"github.com/easterthebunny/pubcommittee/internal/adminapi"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

// wire builds admin-api on the local data directory named by DATA_DIR, with
// localauth for operators. Other processes (read-api, job-ingest) share it.
func wire(tel *telemetry.Telemetry, getenv func(string) string) (adminapi.Deps, error) {
	dir := getenv("DATA_DIR")
	if dir == "" {
		return adminapi.Deps{}, fmt.Errorf("DATA_DIR is required: %w", data.ErrInvalid)
	}
	objects, err := fsobject.New(fsobject.Config{Root: dir})
	if err != nil {
		return adminapi.Deps{}, fmt.Errorf("objects: %w", err)
	}
	docs, err := fsdocs.New(fsdocs.Config{Root: dir})
	if err != nil {
		return adminapi.Deps{}, fmt.Errorf("docs: %w", err)
	}
	cfg, err := devConfig(getenv)
	if err != nil {
		return adminapi.Deps{}, err
	}
	fetcher, source, err := newFetcher(getenv)
	if err != nil {
		return adminapi.Deps{}, err
	}
	var ports data.PlatformPorts
	ports.BillText, ports.PublicDocs = objects, docs
	ports.PlatformDocs, ports.Moderation, ports.Publisher = docs, docs, objects
	return adminapi.Deps{
		Ports: tel.WrapPlatform("local", ports), Fetcher: fetcher, Source: source,
		Auth: devAuth{fixed: localauth.New()}, Config: cfg,
	}, nil
}
