# AGENTS.md — cmd/

Purpose: one directory per binary; each holds `main.go` and one wiring file per build target.
Skills: `tool-go`, `tool-golangci-lint`, `tool-docker`.
Owns: `main.go` and `wiring_*.go` of every service and job. Does not own: domain logic (`internal/`), adapters (`internal/adapters/`).

## Rules
- Wiring files: `wiring_gcp.go`, `wiring_local.go`, `wiring_emulator.go`, `wiring_memory.go`, each behind its build tag, and `wiring_none.go` (`//go:build !gcp && !local && !emulator && !memory`) which references an undefined identifier so an untagged build fails ([architecture § Build-time wiring](../architecture/README.md#build-time-wiring)).
- Each wiring file exports the same function (`wire`) so `main.go` has no build-tag logic. Only wiring files import adapter packages. `wire` may return more than ports (for example `job-ingest`: `(PlatformPorts, BillFetcher, error)`), and a service may split wiring by source (`wiring_sources.go` for `local || memory`, `wiring_cloud.go` for `gcp || emulator` with a static sentinel error).
- `read-api` wiring never imports `postgres`, `kmsgcp`, or `localkeys`; `wiring_gcp.go` never imports a local-only adapter (`fsobject`, `fsdocs`, `localkeys`, `localauth`). `depguard` and `test/ci/deps.sh` enforce both.
- Config from environment only; exit non-zero with a clear message on a missing value. No work at startup beyond reading config and constructing clients.
- HTTP services use `internal/httpx` and `internal/telemetry`; jobs run to completion, push metrics over OTLP, and support `--dry-run`.
- A new binary needs a line in `test/ci/size-budget.txt`.

## Commands
- `docker exec pc-<id>-dev go build -tags memory ./cmd/...`
- `docker exec pc-<id>-dev sh test/ci/tags.sh` (no tag fails; all four tags build)
- `docker exec pc-<id>-dev sh test/ci/deps.sh`

## Gotchas
- `go build ./...` without `-tags` fails by design under `cmd/`.
