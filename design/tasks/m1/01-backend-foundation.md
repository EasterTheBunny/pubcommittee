# M1-01 — Backend Foundation

## Goal

Create the Go backend skeleton every other M1 task builds on: one module, domain types and port interfaces, an in-memory adapter for every M1 port, conformance test suites, shared HTTP and telemetry bootstrap, build-tag wiring, container packaging, a local run, and backend CI. After this task, a new service or job is a `cmd/` directory plus a wiring file.

## Read first

- [architecture § Data access layer](../../../architecture/README.md#data-access-layer) (ports, interface rules, build-time wiring, adapter packages, conformance tests, repository layout)
- [architecture § Service packaging](../../../architecture/README.md#service-packaging) and [§ Observability](../../../architecture/README.md#observability)
- [architecture § Milestone fit](../../../architecture/README.md#milestone-fit) (M1 row)
- [01 § Domain concepts](../../01-bill-ingestion-and-display.md#domain-concepts) for vocabulary
- `app/src/lib/types.ts` for the names the UI already uses

## Scope

**In**
- `go.mod` at the repository root; layout under `cmd/` and `internal/` as in the architecture.
- `internal/data`: domain types, typed errors, and the M1 ports from [architecture § Ports](../../../architecture/README.md#ports): `BillText`, `PublicDocs`, `Moderation`, `PlatformDocs` (including `ApplyLogged`), `Publisher`, `BillSource`, `LawSource`, `Generator`, and the `PlatformPorts` group.
- `internal/data/conformance`: exported suite per port.
- `internal/adapters/memory`: every M1 port, seedable from fixtures.
- `internal/httpx`: listen on `$PORT`, `/healthz`, `/readyz`, `/metrics` on `$METRICS_PORT`, graceful shutdown within 10 s, problem-details error writer.
- `internal/telemetry`: OpenTelemetry setup, standard instruments, attribute allowlist, port-wrapper that records `pubcommittee.data.operation.duration`, OTLP push for jobs.
- Build tags `gcp`, `local` (filesystem adapters from [M1-03a](03a-local-storage-adapters.md)), `memory`, and `emulator` (reserved for [M1-03b](03b-cloud-storage-adapters.md)), and the failing `wiring_none.go` pattern; a placeholder `cmd/read-api` that builds with `memory`.
- `docker/Dockerfile` (multi-stage, `SERVICE` and `TARGET` args, distroless static pinned by digest, `linux/amd64` and `linux/arm64`, labels, SBOM).
- `compose.yaml` with an OpenTelemetry Collector and a shared data volume for the `local` target; no emulators and no Postgres in M1. An `emulator` profile is added in the cloud phase by [M1-03b](03b-cloud-storage-adapters.md).
- Backend CI: `go vet`, lint with `depguard`, tests, `go list -deps` check that `read-api` links no SQL or key adapter, binary size budget (record the first measured sizes and set the budget from them).

**Out**
- Storage adapters ([M1-03a](03a-local-storage-adapters.md) local, [M1-03b](03b-cloud-storage-adapters.md) cloud), sources (M1-05, M1-07), generation (M1-09), the OpenAPI contract (M1-02).
- SQL, keys, sessions, signals: M2.

## Deliverables

- `go.mod`, `cmd/`, `internal/data`, `internal/data/conformance`, `internal/adapters/memory`, `internal/httpx`, `internal/telemetry`
- `docker/Dockerfile`, `compose.yaml`
- `docker/dev.Dockerfile`: the development image every build, test, and lint runs in (Go toolchain at the pinned version, pinned tools, and Node at the version `app/` uses), used by agents and CI alike
- `.golangci.yml` (strict; see Requirements)
- CI workflow for the Go module
- Package documentation explaining how to add a port, an adapter, and a service
- `AGENTS.md` files for `cmd/`, `internal/data/`, and `internal/adapters/`, in the pattern of the root [`AGENTS.md`](../../../AGENTS.md#directory-agentsmd-files)

## Requirements

- Interface rules 1–8 in [architecture § Interface rules](../../../architecture/README.md#interface-rules): domain types only, domain operations, typed errors, `context.Context` first, opaque cursors, adapters own schema, config validated at startup without network, no runtime plugin loading.
- Domain types carry jurisdiction and session; anchors carry `(bill, version, section path, line or line range)` and the source XML id where one exists ([MILESTONES § Prior work](../../MILESTONES.md#m1--real-bill-text-read-only-1)).
- Every content record type has `version`, `as_of`, `provenance`, and `content_class` (`official`, `platform`, `user`) fields where it carries text ([10 § Instruction injection](../../10-machine-access.md#instruction-injection)).
- Telemetry never carries ids, paths, or IPs ([architecture § Privacy rules for telemetry](../../../architecture/README.md#privacy-rules-for-telemetry)); the allowlist is enforced by metric views and covered by a test.
- Config from environment only; non-zero exit with a clear message on missing values.
- **Build context holds no private files.** A `.dockerignore` excludes `deploy/` (the private deployment repository checkout), `contacts/`, `.local-data/`, and `.env*`, and the Dockerfile copies only the paths it builds from (`go.mod`, `go.sum`, `api/`, `cmd/`, `internal/`), so a build from the repository root can never put private values into an image.
- Standard library first; any third-party module needs a stated reason.
- **Strict lint.** `.golangci.yml` (golangci-lint v2) enables all linters (`linters.default: all`); any disabled linter carries a written reason in the file; no issue limits (`max-issues-per-linter: 0`, `max-same-issues: 0`); `nolintlint` requires a named linter and an explanation; `depguard` carries the `read-api` and local-adapter rules. CI fails on any finding.
- **Containers have limits.** Every `compose.yaml` service declares `deploy.resources.limits` (`cpus`, `memory`) and `mem_reservation`, and the stack has an `acceptance` service that runs `test/acceptance/*/run.sh` against it.
- Tooling as decided in [architecture § Tooling](../../../architecture/README.md#tooling): Go 1.27, `tool` directive for `golangci-lint` and `oapi-codegen`, cross-compiled multi-arch builds, BuildKit SBOM, GitHub Actions.

## Suggested breakdown

1. Module, directory layout, lint config, CI skeleton.
2. Domain types and typed errors in `internal/data` (bill, catalog entry, official version, section, line, block, anchor, provision, citation link, change explanation, generation record, selection policy, log entry, feed entry).
3. Port interfaces for M1, grouped as in the architecture (public read, public write, publishing, ingestion, generation).
4. Conformance suites per port (not found, idempotent writes, write-once paths refuse overwrite, cursor stability, typed errors).
5. Memory adapter passing every suite.
6. `internal/httpx` with tests for health, readiness, shutdown, problem details.
7. `internal/telemetry` with allowlist views, port wrapper, Prometheus exporter, OTLP push for jobs, `job.last_run_timestamp`.
8. Wiring pattern and a placeholder `read-api` and job binary to prove it.
9. Dockerfile, multi-arch build, SBOM, labels; `compose.yaml`.
10. CI checks: dependency rule for `read-api`, size budget, tests on every push.

## Acceptance criteria

- `go build -tags memory ./...` and `go test ./...` pass; a build with no target tag fails to compile.
- Memory adapter passes every conformance suite.
- `docker compose up` starts the placeholder `read-api` and the OpenTelemetry Collector with no cloud service or emulator; `/healthz`, `/readyz`, and `/metrics` respond.
- CI fails if a `gcp` build links `localkeys` or any other local-only adapter.
- CI fails if `read-api` links `adapters/postgres`, `adapters/kmsgcp`, or their SDKs (demonstrated by a deliberate failing branch or a test of the check).
- Image is distroless, non-root, multi-arch, labelled, with an SBOM.

## Open questions

- Does `BillSource` declare its "list changed bills since a watermark" operation in M1, when bill pulls are manual ([M1-05](05-manual-bill-pull-and-storage.md))?
  - **Answered (M1-01):** yes. `ListChanged(ctx, since, page)` is declared and the memory adapter and conformance suite cover it; manual pulls pass an empty watermark. Recorded in [architecture § Ports](../../../architecture/README.md#ports).
- What binary size budget applies to each service? The architecture sets it from the first M1 builds; record it in [architecture § Keeping each service small](../../../architecture/README.md#keeping-each-service-small).
  - **Answered (M1-01):** `read-api` measured 16.8 MB; budget 20 MB in `test/ci/size-budget.txt`, enforced in CI. Recorded in the architecture section.
