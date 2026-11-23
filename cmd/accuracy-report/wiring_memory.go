//go:build memory

package main

import (
	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

// wire builds the ports from process memory: an empty store, so the report
// shows what an empty run looks like.
func wire(cfg config) (data.PlatformPorts, data.Generator, error) {
	store := memory.New()
	gen, err := newGenerator(cfg, store)
	if err != nil {
		return data.PlatformPorts{}, nil, err
	}
	return store.PlatformPorts(), gen, nil
}
