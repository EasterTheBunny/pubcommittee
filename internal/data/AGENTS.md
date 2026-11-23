# AGENTS.md — internal/data/

Purpose: domain types, typed errors, port interfaces, and the conformance suites every adapter must pass.
Skills: `tool-go`, `tool-golangci-lint`, `tool-docker`.
Owns: `internal/data` and `internal/data/conformance`. Does not own: any storage technology, SDK, or adapter.

## Rules
- The eight interface rules in [architecture § Interface rules](../../architecture/README.md#interface-rules) apply to every port: domain types only, domain operations, typed errors, `context.Context` first, opaque cursors.
- Record types that carry text have a `Meta` (`version`, `as_of`, `provenance`, `content_class`). Names follow [design/README.md § Shared vocabulary](../../design/README.md#shared-vocabulary).
- Never import a vendor SDK here (`depguard` enforces it).
- A new port needs: interface, group entry in `groups.go`, a conformance suite, a `memory` implementation, and a wrapper in `internal/telemetry`.
- Suites are exported functions taking a factory (`conformance.Docs(t, factory)`); they test behavior any adapter can show, and list what they cannot check.
- Write-once paths: identical content is a no-op, different content is `ErrConflict`.

## Commands
- `docker exec pc-<id>-dev go test -tags memory ./internal/data/... ./internal/adapters/memory/...`
- `docker exec pc-<id>-dev go tool golangci-lint run ./internal/data/...`

## Gotchas
- Cursors are opaque to callers and owned by the adapter; suites must not assume their format.
