//go:build gcp

package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/easterthebunny/pubcommittee/internal/adapters/govinfo"
	"github.com/easterthebunny/pubcommittee/internal/adminapi"
	"github.com/easterthebunny/pubcommittee/internal/cloudports"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

// Environment variables of the gcp target. Their values (the operator list,
// the deploy principal) are never logged.
const (
	envOperators       = "OPERATORS_JSON"
	envDeployPrincipal = "DEPLOY_PRINCIPAL"
	envAudience        = "ADMIN_API_AUDIENCE"
	envClientAudiences = "ADMIN_API_CLIENT_AUDIENCES"
)

// gcpSource is the ingest_state key prefix of the govinfo bill source.
const gcpSource = "govinfo"

var errEnv = errors.New("missing or invalid environment")

// wire builds admin-api on the cloud ports, the govinfo bill source, and the
// Google ID token authenticator. The operator list and the deploy principal
// come from the environment only. Generator is nil: the admin-api service
// account holds no model role in M1, so regenerate answers as unavailable.
// It never imports a local-only adapter (depguard, test/ci/deps.sh).
func wire(tel *telemetry.Telemetry, getenv func(string) string) (adminapi.Deps, error) {
	audience := strings.TrimSpace(getenv(envAudience))
	if audience == "" {
		return adminapi.Deps{}, fmt.Errorf("%s is required: %w", envAudience, errEnv)
	}
	cfg, err := gcpConfig(getenv)
	if err != nil {
		return adminapi.Deps{}, err
	}
	p, err := cloudports.Open(context.Background(), cloudports.GCP, getenv)
	if err != nil {
		return adminapi.Deps{}, fmt.Errorf("cloud ports: %w", err)
	}
	src, err := govinfo.New(govinfo.Config{})
	if err != nil {
		return adminapi.Deps{}, errors.Join(fmt.Errorf("govinfo: %w", err), p.Close())
	}
	return adminapi.Deps{
		Ports:   tel.WrapPlatform("gcp", p.Platform()),
		Fetcher: src,
		Source:  gcpSource,
		Auth:    newIDTokenAuth(audience, strings.Split(getenv(envClientAudiences), ","), googleValidator),
		Config:  cfg,
	}, nil
}

// gcpConfig reads the operator list from OPERATORS_JSON (the
// adminapi.LoadConfig format) and the deploy principal from DEPLOY_PRINCIPAL.
// Either missing or invalid is an error. Error text never carries a principal.
func gcpConfig(getenv func(string) string) (adminapi.Config, error) {
	raw := strings.TrimSpace(getenv(envOperators))
	deploy := strings.TrimSpace(getenv(envDeployPrincipal))
	if raw == "" || deploy == "" {
		return adminapi.Config{}, fmt.Errorf("%s and %s are required: %w", envOperators, envDeployPrincipal, errEnv)
	}
	cfg, err := adminapi.LoadConfig(strings.NewReader(raw))
	if err != nil {
		return adminapi.Config{}, fmt.Errorf("%s: %w", envOperators, err)
	}
	if cfg.DeployPrincipal != "" && cfg.DeployPrincipal != deploy {
		return adminapi.Config{}, fmt.Errorf("%s disagrees with %s: %w", envOperators, envDeployPrincipal, errEnv)
	}
	cfg.DeployPrincipal = deploy
	if err := cfg.Validate(); err != nil {
		return adminapi.Config{}, fmt.Errorf("%s: %w", envOperators, err)
	}
	return cfg, nil
}
