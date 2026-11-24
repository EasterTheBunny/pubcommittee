# M1-03b — Cloud Storage Adapters

## Goal

**Phase:** cloud. Implement the Google Cloud adapters M1 needs, `adapters/gcs` for immutable files and `adapters/firestore` for public mutable documents, behind the M1-01 ports. They pass the same conformance suites as the filesystem adapters from [M1-03a](03a-local-storage-adapters.md), first against emulators on the `emulator` build target, then against a real bucket and database in `dev`. After this task, every service and job builds for the `gcp` target. No local-phase task waits on it.

## Read first

- [architecture § Data placement](../../../architecture/README.md#data-placement) (object store and document store)
- [architecture § Data access layer](../../../architecture/README.md#data-access-layer) (interface rules, adapters own schema, conformance)
- [architecture § Bill source § Data recorded per ingest](../../../architecture/README.md#data-recorded-per-ingest)
- M1-01 port interfaces
- [M1-03a](03a-local-storage-adapters.md) (the same ports on the filesystem, and what those adapters do not model)
- [M1-00](00-storage-emulator-exploration.md) result and its test code

## Scope

**In**
- `adapters/gcs`: `BillText` (read version text, sections, lines by anchor), `Publisher` (write-once text, provisions, generated outputs, exports, feed). Paths such as `bills/{bill}/v/{n}/source.xml`, `bills/{bill}/v/{n}/text.json`, `generated/{task}/{bill}/v/{n}/{key}.json`.
- `adapters/firestore`: `PublicDocs` reads, `Moderation`, and `PlatformDocs` (with `ApplyLogged` as a Firestore transaction) for the M1 collections `bills` (catalog entries and tracked fields), `selection_policy`, `explanations`, `generations`, `moderation_log`, and `ingest_state` if M1 uses it ([M1-05](05-manual-bill-pull-and-storage.md)).
- Firestore indexes for catalog filters and `search_terms` keyword matching, owned by the adapter.
- `emulator` wiring against the Firestore emulator (in a container) and the Cloud Storage stand-in from M1-00, as an `emulator` profile in `compose.yaml`.
- Conformance runs in CI against the emulators.

**Out**
- `threads`, `suggestions`, `issues`, `responses`, `briefs`, `sessions`, `rate_limits`, `markup_queue`, `markup_decisions`: later milestones.
- SQL and key adapters: M2.

## Deliverables

- `internal/adapters/gcs`, `internal/adapters/firestore` with tests calling the conformance suites
- Index definitions for Firestore, deployable by M1-15
- Wiring files for `emulator` and `gcp` in every service and job

## Requirements

- **Write-once is enforced by the adapter**, not by callers: writing an existing immutable path fails with `ErrConflict` (use object preconditions such as "does not exist"). Official versions never change ([01 § Rules](../../01-bill-ingestion-and-display.md#rules-and-invariants)).
- Original XML is stored byte for byte with source URL, retrieval time, and SHA-256.
- Reads of immutable objects expose enough metadata (generation or hash) for strong `ETag`s in `read-api`.
- An admin log entry and the change it records are written atomically; if the log write fails, the change is not applied ([architecture § Administration § Logging](../../../architecture/README.md#logging)). Provide a port operation that does both in one transaction.
- No vendor types or errors cross a port; credentials from workload identity only; config validated without network calls.
- Documents never store an account id (no accounts exist in M1, but the schema must not introduce one).

## Suggested breakdown

1. GCS object layout doc and path helpers in the adapter.
2. `gcs` `Publisher` with write-once preconditions; `BillText` reads with line-range slicing.
3. Firestore collection schemas and document mappers for M1 collections.
4. Catalog queries: filters, `search_terms` matching, opaque cursors.
5. Transactional "change plus log entry" operation.
6. Emulator setup in `compose.yaml` and CI; conformance runs. Start from the M1-00 spike tests in `test/spike/storage-emulator/storage_test.go`: move them into `adapters/gcs` tests under the strict lint config, replace the testbench fault test (14) with an injected-transport unit test, then delete `test/spike/`. No test may rely on stand-in ETag values.
7. Wiring files for `emulator` and `gcp`.
8. At C1, the real-bucket tests listed by M1-00.

## Acceptance criteria

- Both adapters pass every conformance suite for the ports they implement, against emulators, in CI.
- The behaviors M1-03a lists as not modelled locally (query and index limits, transaction size limits) are tested here.
- **Cloud phase, C1:** real-bucket-only tests from M1-00 pass in `dev`.
- A second write to an immutable path fails with `ErrConflict` in a test.
- Catalog query for a filter combination and a keyword returns stable cursors across pages.
- Log-plus-change atomicity covered by a test that injects a log write failure.
- `read-api` built with `gcp` still links no SQL or key adapter.

## Open questions

- Do the final object paths and document shapes match [architecture § Data placement](../../../architecture/README.md#data-placement)? Record any difference there.
  - **Answered (M1-03b):** object paths match `fsobject`; the Firestore document shape (`body` plus query fields), ids and extra M1 collections are recorded in architecture § Data placement. A mid-page cursor can be invalidated only by a deleted catalog document, and M1 has no delete.
- Is keyword matching on `search_terms` enough for M1 catalog search (open decision "Catalog search")?
  - **Answered (M1-03b):** yes for M1: lowercased words, all must match; no stemming, prefix match or ranking. The query is `firestore.QueryCatalog`, not yet a port operation.
- Which Cloud Storage stand-in runs locally and in CI? Google publishes no Cloud Storage emulator, although [architecture § Local run](../../../architecture/README.md#local-run) assumes one. Does `adapters/gcs` pass conformance against it? Answered by [M1-00](00-storage-emulator-exploration.md), which also lists the tests that need a real bucket. **Answered (M1-00, operator 2026-11-23):** `fake-gcs-server` only ([architecture § Conformance tests](../../../architecture/README.md#conformance-tests)); transient errors through an injected `http.RoundTripper`; real-bucket tests in a `dev` test bucket.
