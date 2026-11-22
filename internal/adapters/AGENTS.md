# AGENTS.md — internal/adapters/

Purpose: one package per storage or source technology, each implementing ports from `internal/data`.
Skills: `tool-go`, `tool-golangci-lint`, `tool-docker`.
Owns: adapter packages. Does not own: port definitions or suites (`internal/data`), wiring (`cmd/`).

## Rules
- An adapter implements ports and returns domain types only; vendor types, errors, and timestamps never cross the port. Wrap failures in `data.ErrNotFound`, `ErrConflict`, `ErrUnavailable`, `ErrRateLimited`, `ErrInvalid`.
- The constructor takes a typed config and fails fast on missing values without network calls. Credentials come from workload identity, never config files.
- Every adapter's tests call the conformance suites for the ports it implements. An adapter that fails a suite cannot be wired.
- Local-only adapters (`fsobject`, `fsdocs`, `localkeys`, `localauth`) are never imported from `wiring_gcp.go`.
- Vendor SDKs are imported only inside the adapter package that needs them.
- Adapters do not instrument themselves; `internal/telemetry` wraps ports.

## Commands
- `docker exec pc-<id>-dev go test -tags memory ./internal/adapters/...`
- `docker exec pc-<id>-dev go tool golangci-lint run ./internal/adapters/...`

## Gotchas
- `memory` is one `Store` behind every port, so text written through `Publisher` reads back through `BillText`; other adapters must keep that same visibility.
