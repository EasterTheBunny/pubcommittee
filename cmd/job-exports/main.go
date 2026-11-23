// Command job-exports builds the bulk exports and the change feed from the
// stored data (M1: run on demand, never scheduled; nothing chains it to a
// pull). Each run rebuilds every export from source, writes the export index,
// and appends the new events to the feed. It prints one JSON report and exits
// non-zero when the run failed. See cmd/job-exports/README.md.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/exports"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

const serviceName = "job-exports"

// version is set at build time with -ldflags "-X main.version=<commit>".
var version = "dev"

func main() {
	if err := run(os.Args[1:], os.Getenv, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", serviceName, err)
		os.Exit(1)
	}
}

// parseArgs reads the flags.
func parseArgs(args []string) (exports.Options, error) {
	fs := flag.NewFlagSet(serviceName, flag.ContinueOnError)
	var opts exports.Options
	fs.BoolVar(&opts.DryRun, "dry-run", false, "build everything and report it; write nothing")
	if err := fs.Parse(args); err != nil {
		return opts, fmt.Errorf("flags: %w", err)
	}
	return opts, nil
}

// run parses the flags, wires the ports and runs one export build.
func run(args []string, getenv func(string) string, out io.Writer) error {
	started := time.Now()
	opts, err := parseArgs(args)
	if err != nil {
		return err
	}
	tel, err := telemetry.Setup(telemetry.Options{
		Kind: telemetry.KindJob, ServiceName: serviceName, Version: version,
		Environment: envOr(getenv, "DEPLOYMENT_ENVIRONMENT", "dev"),
	})
	if err != nil {
		return fmt.Errorf("telemetry: %w", err)
	}
	ports, err := wire(tel)
	if err != nil {
		return fmt.Errorf("wiring: %w", err)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	rep, runErr := exports.Run(context.Background(), exports.Deps{Ports: ports, Log: logger, Now: time.Now}, opts)
	items := len(rep.Written)
	if encErr := encodeReport(out, rep); encErr != nil && runErr == nil {
		runErr = encErr
	}
	tel.Instruments.RecordJob(context.Background(), serviceName, started, items, runErr)
	if err := tel.Shutdown(context.Background()); err != nil {
		logger.Warn("telemetry shutdown", "error", err.Error())
	}
	return runErr
}

func encodeReport(out io.Writer, rep exports.Report) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rep); err != nil {
		return fmt.Errorf("report: %w", err)
	}
	return nil
}

func envOr(getenv func(string) string, key, def string) string {
	if v := getenv(key); v != "" {
		return v
	}
	return def
}
