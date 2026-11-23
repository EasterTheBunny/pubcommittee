//go:build local

// Command seed writes the invented fixture bills into the local data
// directory named by DATA_DIR. It stands in for a platform job running in
// another container, so the acceptance script can show that read-api serves
// documents written by a different process.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/easterthebunny/pubcommittee/internal/adapters/fsdocs"
	"github.com/easterthebunny/pubcommittee/internal/adapters/fsobject"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/readapi/seed"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "seed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	dir := os.Getenv("DATA_DIR")
	if dir == "" {
		return fmt.Errorf("DATA_DIR is required: %w", data.ErrInvalid)
	}
	objects, err := fsobject.New(fsobject.Config{Root: dir})
	if err != nil {
		return fmt.Errorf("objects: %w", err)
	}
	docs, err := fsdocs.New(fsdocs.Config{Root: dir})
	if err != nil {
		return fmt.Errorf("docs: %w", err)
	}
	ports := data.PlatformPorts{PlatformDocs: docs, Moderation: docs, Publisher: objects}
	ports.BillText, ports.PublicDocs = objects, docs
	if err := seed.Apply(context.Background(), ports); err != nil {
		return fmt.Errorf("apply: %w", err)
	}
	slog.Info("seeded", "dir", dir)
	return nil
}
