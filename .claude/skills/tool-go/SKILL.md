---
name: tool-go
description: Go conventions for the PubCommittee backend: module layout, ports and adapters, interface rules, typed errors, build tags and wiring, conformance suites, net/http and slog, tests, and the commands to build and test inside the task container. Load for any change under cmd/, internal/, or go.mod.
---

# Go backend

Source of truth: [architecture § Data access layer](../../../architecture/README.md#data-access-layer) and [§ Service packaging](../../../architecture/README.md#service-packaging). Read the nearest `AGENTS.md` for the package you edit.

## Layout

- `cmd/<binary>/`: `main.go` plus one wiring file per build target.
- `internal/data/`: ports, domain types, typed errors. `internal/data/conformance/`: one exported suite per port.
- `internal/adapters/<name>/`: one technology per package; vendor SDK imports only here.
- `internal/httpx/`, `internal/telemetry/`: shared bootstrap.

## Rules that reviews and lint check

1. Ports take and return `internal/data` types only. No vendor types, rows, or SDK errors cross a port.
2. Domain operations, not generic queries (`SignalLedger.Set`, never `Query(sql)`).
3. Errors: wrap adapter failures in `data.ErrNotFound`, `ErrConflict`, `ErrUnavailable`, `ErrRateLimited`, `ErrInvalid`; callers use `errors.Is`. Wrap with `fmt.Errorf("...: %w", err)`.
4. `ctx context.Context` is the first parameter of every port method and I/O function.
5. Cursors are opaque strings owned by the adapter.
6. Config from environment only, validated at startup with no network calls; fail fast with a clear message.
7. Build tags pick adapters: `//go:build gcp`, `local`, `emulator`, `memory`. Wiring files import adapters; services never do. `wiring_none.go` must fail to compile.
8. Standard library first: `net/http` with method and path patterns, `log/slog` JSON handler, `encoding/json`, `encoding/xml`. A new module needs a reason in your report.
9. Logs never include client IPs, cookies, bodies, account ids, or bill/thread ids as metric attributes.

## Tests

- Table-driven tests with `t.Run`; `t.Parallel()` where safe; `t.TempDir()` for files.
- An adapter's tests call the conformance suite for every port it implements: `conformance.BillText(t, factory)`.
- Golden files under `testdata/`; regenerate only with an explicit flag (`-update`), never by default.
- No network in unit tests; fixtures come from `fixturesrc` or `testdata/`.

## Commands (inside the container)

```sh
docker exec <container> go build -tags memory ./...
docker exec <container> go test -tags memory -race -count=1 ./internal/...
docker exec <container> go vet -tags memory ./...
docker exec <container> go list -deps -tags gcp ./cmd/read-api   # dependency rule checks
docker exec <container> go mod tidy -diff                         # module file is tidy
```

Run tests for every build tag your change affects.
