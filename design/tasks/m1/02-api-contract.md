# M1-02 — API Contract

## Goal

Write `api/openapi.yaml` for every M1 read route, generate server and client types from it, and add contract tests so the backend, the web UI, and outside machines all depend on one documented, versioned structure. This closes the M1 exit criterion "structure of machine responses documented and versioned".

## Read first

- [architecture § API contract](../../../architecture/README.md#api-contract) (resources, conventions, contract workflow)
- [10 § Read access](../../10-machine-access.md#read-access) and [§ Provenance in every response](../../10-machine-access.md#provenance-in-every-response)
- [10 § Instruction injection](../../10-machine-access.md#instruction-injection) (content classes)
- [06 § Link behavior](../../06-sharing.md#link-behavior) (pinned, changed since, latest)
- M1-01 domain types

## Scope

**In** (M1 read routes only)
- `GET /v1` discovery.
- `GET /v1/bills` (tracked set) and `?scope=catalog` with filters `status`, `chamber`, `subject`, `q`, and cursor ([architecture § Resources](../../../architecture/README.md#resources)). A committee-activity filter is an open question.
- `GET /v1/bills/{bill}`, `GET /v1/bills/{bill}/v/{n}`, `.../s/{section}`, `.../s/{section}/l/{range}`; version-less address resolving to latest and saying so.
- `GET /v1/bills/{bill}/v/{n}/explanations`.
- Resources for bill summaries and version change summaries (M1-09), citations with their resolution state and the unresolved list (M1-07), and version comparisons (M1-08). The architecture lists no routes for these; see open questions.
- `GET /v1/selection-policy`, `GET /v1/selection-policy/{version}`.
- `GET /v1/moderation-log` (administrator actions in M1).
- `GET /v1/feed` and `GET /v1/exports/...` index.
- Shared schemas: provenance block, content class on every text field, problem details, cursor collections, anchor and canonical address.
- Codegen: `oapi-codegen` for Go server types and request validation; `openapi-typescript` for `app/`.
- Contract tests that call a running service and validate responses against the spec.
- A human-readable "machine response structure" page generated from or linked to the spec, with the versioning and deprecation policy.

**Out**
- `write-api`, `/auth`, `/v1/me*`, signals, threads, suggestions, briefs, snapshots: later milestones. Leave them out of the M1 spec rather than stubbing them.
- `admin-api` routes: not part of the public contract (M1-12 documents them separately).

## Deliverables

- `api/openapi.yaml` (OpenAPI 3.1)
- Generated Go package (location agreed with M1-01) and generated TypeScript types in `app/`
- One command that regenerates both; CI check that generated code is current
- Contract test harness usable by M1-10
- Structure and versioning doc for machine clients

## Requirements

- Conventions in [architecture § Conventions](../../../architecture/README.md#conventions): `/v1` in path, `snake_case`, RFC 3339 UTC, `{ "items": [...], "next": "<cursor>" }`, opaque cursors, strong `ETag`, RFC 9457 problem types (`not-found`, `conflict`, `invalid`, `rate-limited`, `unavailable`), `429` with `Retry-After`.
- Every record carrying text has `version`, `as_of`, `provenance`, `content_class`. Summary fields hold only `official` or `platform` text.
- Explanations carry the provenance flag (`auto`, `steward-checked`, `bill-only`); as-amended previews carry `unofficial`.
- Line ranges carry each line's anchor, depth, and text, so a client can rebuild blocks without page scripts.
- Addresses in the API mirror page addresses ([10 § Canonical addresses](../../10-machine-access.md#canonical-addresses)).
- Catalog entries carry `official_links` and `markup_status` with exclusion reason; untracked entries carry no bill text.
- No endpoint lists or filters by author ([11](../../11-ai-security.md)).

## Suggested breakdown

1. Shared components: provenance, content class, anchor, canonical address, problem details, collection and cursor.
2. Bill, catalog entry, and selection policy schemas; catalog filters and search.
3. Version, section, and line-range schemas; latest-version resolution.
4. Explanation, summary, citation, and version-comparison schemas (coordinate with M1-07, M1-08, M1-09 owners).
5. Moderation log, feed entry, and export index schemas.
6. Codegen setup for Go and TypeScript; CI freshness check.
7. Contract test harness against the memory-wired `read-api`.
8. Machine-client structure and versioning doc.

## Acceptance criteria

- Spec validates as OpenAPI 3.1 in CI.
- Generated Go and TypeScript code compiles and is current.
- Contract tests run in CI against `read-api` built with `memory` and seeded fixtures.
- Every text field in every schema declares `content_class`.
- Versioning and deprecation policy is published beside the spec.

## Open questions

- **Decided: this task proposes** the routes and shapes for bill summaries, version change summaries, citations with the unresolved list, and version comparisons, following [architecture § API contract](../../../architecture/README.md#api-contract) (cut by freshness, addresses mirror pages). It reports them as a doc change for [architecture § Resources](../../../architecture/README.md#resources); M1-07, M1-08, and M1-09 build to them and propose changes if their data needs one.
- Committee-activity filter: **Decided:** yes, in M1 ([01 § Public catalog](../../01-bill-ingestion-and-display.md#public-catalog)); added to [architecture § Resources](../../../architecture/README.md#resources).
- Short-code link aliases: **Decided:** not in M1; human-readable canonical addresses only.
- What is the range syntax in paths, and how is a block addressed? It must match the UI fragment form `#s4-l12-13`.
  - **Answered (M1-02):** `{range}` is `12` or `12-13`, inclusive. A block has no separate syntax: each line carries `block_to`, and the block is the range `n-block_to`. Recorded in [architecture § Resources](../../../architecture/README.md#resources).
- Spec linter: **Decided:** this task chooses one and proposes it for [architecture § Tooling](../../../architecture/README.md#tooling).
  - **Answered (M1-02):** Redocly CLI, recorded in architecture § Tooling.
- Where the spec is published: **Decided:** `read-api` serves it at `GET /v1/openapi.yaml`, linked from `GET /v1`.
