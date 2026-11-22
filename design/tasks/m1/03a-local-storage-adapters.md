# M1-03a — Local Storage Adapters

## Goal

Implement filesystem adapters for every M1 storage port, so every service and job runs durably on a local machine with no cloud service and no emulator. `adapters/fsobject` stands in for Cloud Storage and `adapters/fsdocs` for Firestore. Both pass the same conformance suites the cloud adapters will pass in [M1-03b](03b-cloud-storage-adapters.md). After this task, ingest, generation, exports, administration, and `read-api` run on the `local` build target against one shared data directory.

## Read first

- [architecture § Data placement](../../../architecture/README.md#data-placement) (object store and document store)
- [architecture § Data access layer](../../../architecture/README.md#data-access-layer) (interface rules, adapters own schema, conformance, build targets)
- [architecture § Bill source § Data recorded per ingest](../../../architecture/README.md#data-recorded-per-ingest)
- [M1-01](01-backend-foundation.md) port interfaces and conformance suites

## Scope

**In**
- `adapters/fsobject`: `BillText`, `Snapshots` (reads), `Publisher` (writes) on a data root, using the same object paths as the bucket (`bills/{bill}/v/{n}/source.xml`, `bills/{bill}/v/{n}/text.json`, `generated/{task}/{bill}/v/{n}/{key}.json`, exports, feed).
  - Write-once by exclusive create (`O_CREATE|O_EXCL`); an existing path returns `ErrConflict`.
  - Replaceable files by write-to-temp, `fsync`, and atomic rename, with a generation check so a stale writer gets `ErrConflict`.
  - Metadata (content type, cache control, source URL, retrieved-at, SHA-256) in a sidecar `.meta.json`; the content hash is the strong `ETag`.
  - Range reads for line slicing, if the port needs them.
- `adapters/fsdocs`: `PublicDocs` reads, `Moderation`, and `PlatformDocs` (with `ApplyLogged`) for the M1 collections (`bills`, `selection_policy`, `explanations`, `generations`, `moderation_log`, `ingest_state`), one JSON file per document at `<collection>/<id>.json`.
  - Writes by temp file and atomic rename, so readers in other processes never see a partial document.
  - The "change plus log entry" operation under an exclusive lock file: write the log entry first, then the change; on failure, remove the log entry and return the error, so neither is applied alone.
  - Catalog filters, `search_terms` matching, and stable opaque cursors from an index file the adapter rebuilds on write.
- `adapters/localauth`: operator authentication for `admin-api` returning a fixed fake operator id. `local` and `memory` targets only.
- `local` wiring for every service and job that exists, with `DATA_DIR` as the only storage setting.
- `compose.yaml`: services share one data volume (bind-mounted, for example `./.local-data`, which is ignored by git); no emulators.
- Conformance suites run against both adapters in CI, with no containers.

**Out**
- `adapters/gcs`, `adapters/firestore`, emulators, Firestore indexes: [M1-03b](03b-cloud-storage-adapters.md), in the cloud phase.
- Later-milestone collections (`threads`, `suggestions`, `sessions`, and so on).
- Production use. These adapters are for local runs and tests; a self-hosted production backend is a separate decision ([architecture § Build targets](../../../architecture/README.md#build-targets)).

## Deliverables

- `internal/adapters/fsobject`, `internal/adapters/fsdocs`, `internal/adapters/localauth`, each with tests calling the conformance suites
- `local` wiring files
- `compose.yaml` with the shared data volume; the data directory added to `.gitignore`
- A short note in each adapter's package documentation on what it does not model (Firestore query and index limits, transaction size limits, IAM)

## Requirements

- **Same contract as the cloud adapters.** Every conformance suite that `memory` passes, these pass. Behavior a suite cannot check is listed in the package documentation for [M1-03b](03b-cloud-storage-adapters.md) to verify.
- **Write-once enforced by the adapter**, not by callers ([01 § Rules](../../01-bill-ingestion-and-display.md#rules-and-invariants)).
- **Original XML byte for byte** with source URL, retrieval time, and SHA-256.
- **Safe across processes.** `read-api` and the jobs run as separate processes on the same data directory; readers never see partial writes, and concurrent writers to one path cannot both succeed.
- **No vendor types or errors cross a port**; filesystem errors map to the typed errors.
- **Never in a `gcp` build.** `fsobject`, `fsdocs`, `localauth`, and `localkeys` are excluded from `gcp` wiring; a CI dependency check fails if a `gcp` binary links any of them. `localauth` in particular must never reach a deployed `admin-api`.
- Documents never store an account id.

## Suggested breakdown

1. Path helpers shared with the bucket layout; data root config validated at startup without touching other paths.
2. `fsobject` write-once and replaceable writes, metadata sidecar, reads with `ETag`.
3. `fsdocs` document mapping, atomic writes, lock file.
4. Transactional "change plus log entry" with failure injection tests.
5. Catalog index file, filters, `search_terms`, cursors.
6. `localauth`.
7. `local` wiring, `compose.yaml` volume, CI conformance runs, and the `gcp` exclusion check.

## Acceptance criteria

- Both adapters pass every conformance suite for their ports in CI.
- A second write to an immutable path fails with `ErrConflict`; two concurrent processes creating one path: exactly one succeeds.
- A log-plus-change operation with an injected log failure leaves no change.
- A catalog query for a filter combination and a keyword returns stable cursors across pages.
- `docker compose up` runs `read-api` against the shared data directory, and a job run in another container is visible to it.
- `go list -deps -tags gcp` for every service contains none of `fsobject`, `fsdocs`, `localauth`, `localkeys`.

## Open questions

- Does the catalog index file stay fast enough for a full Congress (about 15,000 entries), or does `fsdocs` need an in-memory index built at startup?
- Should the local data directory be resettable from a fixture snapshot (for example `make local-reset`), so tests and demos start from a known state?
