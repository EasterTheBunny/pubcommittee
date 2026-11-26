// Command read-api is the public, read-only JSON service. It serves the
// routes of api/openapi.yaml from the read ports its build target wires.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/httpx"
	"github.com/easterthebunny/pubcommittee/internal/readapi"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

const (
	serviceName        = "read-api"
	healthcheckTimeout = 2 * time.Second
)

var errUnhealthy = errors.New("healthcheck: unexpected status")

// version is set at build time with -ldflags "-X main.version=<commit>".
var version = "dev"

func main() {
	if err := run(os.Args[1:], os.Getenv); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", serviceName, err)
		os.Exit(1)
	}
}

func run(args []string, getenv func(string) string) error {
	started := time.Now()
	cfg, err := httpx.ConfigFromEnv(getenv)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	// The container has no shell or curl; the image's health check runs the binary.
	if len(args) == 1 && args[0] == "-healthcheck" {
		return healthcheck(cfg.Port)
	}

	tel, err := telemetry.Setup(telemetry.Options{
		Kind:        telemetry.KindService,
		ServiceName: serviceName,
		Version:     version,
		Environment: envOr(getenv, "DEPLOYMENT_ENVIRONMENT", "dev"),
	})
	if err != nil {
		return fmt.Errorf("telemetry: %w", err)
	}
	ports, err := wire(tel)
	if err != nil {
		return fmt.Errorf("wiring: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	limits, err := readapi.ParseRateLimits(getenv("RATE_LIMITS"))
	if err != nil {
		return fmt.Errorf("config: RATE_LIMITS: %w", err)
	}
	handler, err := readapi.Handler(readapi.New(ports), readapi.Options{
		RateLimits: limits, ClientIPHeader: getenv("CLIENT_IP_HEADER"), Logger: logger, UIURL: getenv("PUBLIC_UI_URL"),
		ClientIPDebug: getenv("RATE_LIMIT_DEBUG") == "1",
	})
	if err != nil {
		return fmt.Errorf("handler: %w", err)
	}

	srv := httpx.New(cfg, tel.Instruments.HTTP(handler), tel.MetricsHandler(),
		func(ctx context.Context) error {
			_, err := ports.BillText.ListVersions(ctx, data.BillRef{Jurisdiction: "us", Session: "0", Type: "hr", Number: 1})
			if err != nil {
				return fmt.Errorf("bill text: %w", err)
			}
			return nil
		})
	tel.Instruments.RecordStartup(context.Background(), time.Since(started))
	logger.Info("starting", "service", serviceName, "port", cfg.Port)
	if err := srv.RunSignals(); err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}

func envOr(getenv func(string) string, key, def string) string {
	if v := getenv(key); v != "" {
		return v
	}
	return def
}

func healthcheck(port string) error {
	ctx, cancel := context.WithTimeout(context.Background(), healthcheckTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+port+"/healthz", nil)
	if err != nil {
		return fmt.Errorf("healthcheck: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("healthcheck: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %d", errUnhealthy, resp.StatusCode)
	}
	return nil
}
