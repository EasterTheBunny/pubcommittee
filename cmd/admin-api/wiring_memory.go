//go:build memory

package main

import (
	"github.com/easterthebunny/pubcommittee/internal/adapters/localauth"
	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/adminapi"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

// wire builds admin-api from process memory with localauth. Nothing persists.
func wire(tel *telemetry.Telemetry, getenv func(string) string) (adminapi.Deps, error) {
	cfg, err := devConfig(getenv)
	if err != nil {
		return adminapi.Deps{}, err
	}
	fetcher, source, err := newFetcher(getenv)
	if err != nil {
		return adminapi.Deps{}, err
	}
	return adminapi.Deps{
		Ports: tel.WrapPlatform("memory", memory.New().PlatformPorts()), Fetcher: fetcher, Source: source,
		Auth: devAuth{fixed: localauth.New()}, Config: cfg,
	}, nil
}
