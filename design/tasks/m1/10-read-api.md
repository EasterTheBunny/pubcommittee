# M1-10 — `read-api`

## Goal

Build the public, read-only JSON service that serves everything M1 publishes: catalog, tracked bills, immutable version text, line ranges by canonical address, citations, explanations, summaries, version comparisons, selection policy, the public log, and links to exports and the feed. Every response carries provenance and content class, caches correctly, and passes the M1-02 contract tests.

## Read first

- [architecture § API contract](../../../architecture/README.md#api-contract) (resources, conventions)
- [architecture § Overview](../../../architecture/README.md#overview) rule 1 (no SQL on the read path) and [§ Request paths](../../../architecture/README.md#request-paths)
- [10 § Read access](../../10-machine-access.md#read-access) (canonical addresses, machine-readable forms, provenance, caching and limits)
- `api/openapi.yaml` (M1-02)

## Scope

**In**
- `cmd/read-api` with `gcp`, `local`, and `memory` wiring; read ports only.
- Handlers generated from the spec for every M1 route.
- Canonical address resolution: bill, version, section path, line or range; version-less address resolves to latest and states it; old-version address returns "changed since" data from M1-08.
- Catalog listing and search with filters and cursors; tracked set listing.
- `ETag` on every `GET`, `304` on `If-None-Match`; `Cache-Control: immutable` with long max-age for official version text; short TTLs for catalog, explanations, policy, and log.
- CORS: any origin, no credentials.
- Anonymous rate limiting with published limits per route class; `429` with `Retry-After`; no raw IPs stored. The mechanism is an open question.
- `GET /v1` discovery linking to the spec.
- Request logging without client IPs, cookies, or bodies.

**Out**
- Any write route, session, or credentialed response.
- Snapshots, threads, suggestions, briefs: later milestones.

## Deliverables

- `cmd/read-api` service and image
- Contract tests passing in CI against `memory` and `local` builds
- Published rate-limit table

## Requirements

- **Links no SQL or key adapter** (CI check from M1-01).
- Every record carrying text has `version`, `as_of`, `provenance`, `content_class`; explanations carry the provenance flag; previews carry `unofficial`; the bill's review state is included.
- Untracked catalog entries never include bill text.
- Exceeding limits slows a client; it never bans an address range shared by real residents ([10 § Caching and limits](../../10-machine-access.md#caching-and-limits)).
- Problem details for every error, with no vendor errors or stack traces.
- Cold start dominated by image pull: no work at startup beyond config and client construction.

## Suggested breakdown

1. Service skeleton on `internal/httpx`, wiring files, generated server interface.
2. Catalog and bill routes; search and filters.
3. Version, section, and line-range routes; address parsing and validation; latest resolution.
4. Citations, explanations, summaries, and comparison routes.
5. Selection policy, log, feed and export index routes.
6. Caching headers and `ETag` strategy (content hash or object generation).
7. Rate limiting and published limits.
8. Contract tests and cache-header tests on the `local` target; in the cloud phase, a load smoke test through the API edge in `dev`.

## Acceptance criteria

- Contract tests pass for every M1 route.
- A repeated `GET` with `If-None-Match` returns `304`; official text responses are `immutable`.
- `curl` can fetch any line range of an exit bill by canonical address and get anchors, depth, text, and provenance.
- `go list -deps -tags gcp ./cmd/read-api` contains no SQL or key packages.
- Logs contain no client IPs.
- All of the above pass on the `local` target against the filesystem stores.
- **Cloud phase:** load smoke test through the API edge in `dev`; CDN honors `immutable` and short TTLs.

## Open questions

- Rate-limit mechanism: **Decided:** an in-memory limiter per instance, keyed on `HMAC(k_rate, ip ‖ day)` held only in memory; no storage port, no raw IPs. Limits are approximate across instances, which fits "slows, never bans".
- Published limits: **Decided:** per route class, as ranges only ([architecture § What stays private](../../../architecture/README.md#what-stays-private)).
- Do cache lifetimes differ from [architecture § Resources](../../../architecture/README.md#resources)? Record any difference there.
