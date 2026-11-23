//go:build local || memory

package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/easterthebunny/pubcommittee/internal/adapters/fixturesrc"
	"github.com/easterthebunny/pubcommittee/internal/adapters/govinfo"
	"github.com/easterthebunny/pubcommittee/internal/adapters/localauth"
	"github.com/easterthebunny/pubcommittee/internal/adminapi"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

// Bill sources of INGEST_SOURCE.
const (
	sourceFixtures = "fixtures"
	sourceGovinfo  = "govinfo"
)

var errSource = errors.New("INGEST_SOURCE must be fixtures or govinfo")

// principalHeader lets a local caller act as a fake principal, to try the
// refusal rules (a stranger, the deploy account) without Google IAM. It exists
// only in local and memory builds; localauth alone knows one fixed operator.
const principalHeader = "X-Local-Principal"

// localDeployPrincipal is the fake deploy service account of local builds.
const localDeployPrincipal = "local-deploy"

// devAuth authenticates as localauth's fixed operator, or as the fake
// principal named by principalHeader.
type devAuth struct{ fixed localauth.Authenticator }

func (a devAuth) Authenticate(r *http.Request) (string, error) {
	if p := r.Header.Get(principalHeader); p != "" {
		return p, nil
	}
	op, err := a.fixed.Authenticate(r.Context())
	if err != nil {
		return "", fmt.Errorf("localauth: %w", err)
	}
	return op.ID, nil
}

// devConfig reads the operator list from the JSON file OPERATORS_FILE
// (fake principals only); without one, the one local operator and the fake
// deploy account are the list.
func devConfig(getenv func(string) string) (adminapi.Config, error) {
	path := getenv("OPERATORS_FILE")
	if path == "" {
		return adminapi.Config{
			Operators:       []adminapi.Operator{{ID: "op-local", Name: "Local Operator", Principal: localauth.OperatorID}},
			DeployPrincipal: localDeployPrincipal,
		}, nil
	}
	f, err := os.Open(path) //nolint:gosec // an operator-chosen local path, local builds only
	if err != nil {
		return adminapi.Config{}, fmt.Errorf("OPERATORS_FILE: %w", err)
	}
	defer func() { _ = f.Close() }()
	cfg, err := adminapi.LoadConfig(f)
	if err != nil {
		return adminapi.Config{}, fmt.Errorf("OPERATORS_FILE: %w", err)
	}
	return cfg, nil
}

// newFetcher builds the bill source of INGEST_SOURCE: the example data on
// disk (FIXTURES_DIR) or the live govinfo service.
func newFetcher(getenv func(string) string) (data.BillFetcher, string, error) {
	switch envOr(getenv, "INGEST_SOURCE", sourceFixtures) {
	case sourceGovinfo:
		s, err := govinfo.New(govinfo.Config{})
		if err != nil {
			return nil, "", fmt.Errorf("govinfo: %w", err)
		}
		return s, sourceGovinfo, nil
	case sourceFixtures:
		s, err := fixturesrc.New(fixturesrc.Config{FS: os.DirFS(envOr(getenv, "FIXTURES_DIR", "fixtures"))})
		if err != nil {
			return nil, "", fmt.Errorf("fixturesrc: %w", err)
		}
		return s, sourceFixtures, nil
	}
	return nil, "", errSource
}
