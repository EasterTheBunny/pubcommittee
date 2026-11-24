# M1-00 — Cloud Storage Emulator Exploration

## Goal

**Phase:** cloud. Local runs use the filesystem adapters from [M1-03a](03a-local-storage-adapters.md) and need no stand-in. This task is needed only before `adapters/gcs` is built in M1-03b.

Pick the Cloud Storage stand-in for the `emulator` build target and CI by testing what the M1 storage adapter will need. Run the anticipated tests against Google's `storage-testbench` first. If any required test cannot run or fails there, run the same tests against `fake-gcs-server`. Record which stand-in covers what, and which behaviors only a real bucket can verify. Then update the architecture and every task that depends on the answer.

Google publishes no supported Cloud Storage emulator. Its own Go client tests use `storage-testbench` through `STORAGE_EMULATOR_HOST`. The testbench README says it is not an officially supported product, does fewer error checks than the service, and does no ACL or IAM checks.

## Read first

- [architecture § Local run](../../../architecture/README.md#local-run), [§ Conformance tests](../../../architecture/README.md#conformance-tests), [§ Object store](../../../architecture/README.md#object-store-immutable-or-regenerated-files)
- [architecture § Bill source § Data recorded per ingest](../../../architecture/README.md#data-recorded-per-ingest)
- [M1-03b](03b-cloud-storage-adapters.md) (what `adapters/gcs` must do), [M1-05](05-manual-bill-pull-and-storage.md), [M1-11](11-exports-feed-machine-notice.md)
- [environments](environments.md) (stage E1)
- `googleapis/storage-testbench` README; `fsouza/fake-gcs-server` README; the `cloud.google.com/go/storage` package docs on `STORAGE_EMULATOR_HOST`
- The `google-cloud-go` `storage/emulator_test.sh` script, for how Google starts the testbench

## Scope

**In**
- A throwaway Go test module, kept outside the backend module, using `cloud.google.com/go/storage` with its default JSON API client, configured only through `STORAGE_EMULATOR_HOST`.
- Each stand-in started from a public container image pinned by digest, the way CI would start it.
- The anticipated test list below, run against the testbench, then against `fake-gcs-server` for anything the testbench did not cover.
- A coverage matrix and a recommendation.
- Doc and task updates with the result.

**Out**
- Writing `adapters/gcs` itself ([M1-03b](03b-cloud-storage-adapters.md)). The spike's tests are handed to M1-03b as a starting point for its conformance runs.
- Tests against a real bucket. This needs a cloud project (stage C1). The spike lists which behaviors need it.
- The gRPC client: `adapters/gcs` uses the JSON API client (see open questions).

## Anticipated tests

Each test states the behavior expected from the real service, per Google's Cloud Storage documentation. A stand-in **covers** a test when it passes with that behavior. Matching the real service matters more than passing: a stand-in that accepts what the service would refuse does not cover the test.

| # | Test | Expected behavior | Needed by |
| --- | --- | --- | --- |
| 1 | Create bucket in test setup; write and read an object | Bytes read back are identical to bytes written | All |
| 2 | Byte-for-byte originals | A bill XML with a byte-order mark, CRLF line endings, and non-ASCII text round-trips with the same SHA-256 | M1-05 |
| 3 | Write-once create | A write with the "does not exist" precondition (`DoesNotExist`, `ifGenerationMatch=0`) succeeds on a new path | M1-03b, M1-05 |
| 4 | Write-once refusal | The same write to an existing path fails with HTTP 412, so the adapter can return `ErrConflict` | M1-03b |
| 5 | Concurrent create | Two concurrent "does not exist" writes to one path: exactly one succeeds | Overlap-safe jobs |
| 6 | Generation-match update | A write conditioned on the current generation succeeds; one conditioned on a stale generation fails with 412 | Regenerated files, M1-11 |
| 7 | Metadata | `Content-Type`, `Cache-Control`, and custom metadata (source URL, retrieved-at, SHA-256) are stored and returned | M1-05, M1-10 |
| 8 | Checksums and identity | Object attributes return generation, MD5, CRC32C, and ETag; they change when content changes | Strong `ETag` in M1-10 |
| 9 | Not found | Reading a missing object returns `storage.ErrObjectNotExist` | `ErrNotFound` mapping |
| 10 | Range read | `NewRangeReader` returns the requested byte range | Line-range reads if M1-03b slices files |
| 11 | Large object | An object above the client's chunk size (resumable upload) round-trips intact | Large bills, exports |
| 12 | Listing | Prefix and delimiter listing returns the right objects and prefixes, with page tokens across several pages | Exports index, cursors |
| 13 | Delete | Deleting an object removes it; deleting a missing object returns not found | Rebuilt exports |
| 14 | Transient errors | Injected 503 and connection reset on idempotent operations are retried by the client or returned for `ErrUnavailable` mapping (testbench: `x-goog-emulator-instructions`) | Typed errors in M1-03b |
| 15 | Race detector | The suite passes under `go test -race` with `STORAGE_EMULATOR_HOST` set (a data race was reported in `google-cloud-go` issue #2476) | CI |
| 16 | Startup | Container start to first successful request, measured; health check usable by `compose.yaml` | CI time, E1 |
| 17 | Permissions | Writes with no or insufficient permission are refused | Least privilege |

Test 17 is expected to fail on both stand-ins, since neither checks IAM. Record it as real-bucket-only rather than as a gap to close.

## Decision rule

1. Run tests 1–16 against `storage-testbench`.
2. If all are covered, recommend the testbench. Stop the comparison.
3. Otherwise, run the tests the testbench did not cover against `fake-gcs-server`. Rerun the rest only if a single stand-in is still possible.
4. Recommend one of:
   - **testbench only**;
   - **`fake-gcs-server` only**, if it covers everything the testbench covers and more;
   - **both**, with which tests run on which, if neither covers all and running two images in CI is acceptable;
   - **neither for some tests**: those tests run only against a real bucket in `dev`.
5. In every case, list the tests left for a real bucket (at least test 17).

## Deliverables

- Coverage matrix: test × stand-in, with covered, not covered, or wrong behavior, and evidence (test output, versions, image digests)
- Recommendation following the decision rule
- The spike's test code, handed to the M1-03b owner
- Doc and task updates below

## Updates on completion

- [architecture § Local run](../../../architecture/README.md#local-run): replace "Cloud Storage emulator" with the chosen stand-in(s), image and pin, and how it is started.
- [architecture § Conformance tests](../../../architecture/README.md#conformance-tests): which `adapters/gcs` tests run on the stand-in, and which need a real bucket.
- [architecture § Adapter packages](../../../architecture/README.md#adapter-packages) and [§ Build targets](../../../architecture/README.md#build-targets): what `local` points `adapters/gcs` at.
- [M1-01](01-backend-foundation.md): the `emulator` profile in `compose.yaml`.
- [M1-03b](03b-cloud-storage-adapters.md): emulator in scope and acceptance; resolve its stand-in open question; add real-bucket-only tests to acceptance at C1.
- [M1-15](15-infrastructure-and-deploy.md): CI services for the stand-in; a `dev` test bucket if real-bucket tests are needed.
- [environments](environments.md): stage E1 row, tasks-by-stage table, and the stand-in open question.
- Any other task whose tests depend on stored objects ([M1-05](05-manual-bill-pull-and-storage.md), [M1-10](10-read-api.md), [M1-11](11-exports-feed-machine-notice.md)), if the result changes how they test.
- [Task index](../README.md): mark M1-00 done.

## Requirements

- Tests use only the public client API that `adapters/gcs` will use; no stand-in-specific endpoints except the testbench's fault-injection header in test 14.
- Images pinned by digest; versions recorded.
- No cloud account or credentials needed. The spike runs at stage E1 on a local machine.
- Licences of the chosen stand-in are recorded (they are test tools, not shipped in any image).

## Suggested breakdown

1. Read the testbench and `fake-gcs-server` docs; confirm images, ports, and how each takes `STORAGE_EMULATOR_HOST`.
2. Write tests 1–16 as a Go test module parameterised by endpoint.
3. Start the testbench pinned by digest; run the tests; record the results.
4. Apply the decision rule; if needed, start `fake-gcs-server` and run the remaining tests.
5. Write the coverage matrix and recommendation.
6. Make the doc and task updates; hand the test code to M1-03b.

## Acceptance criteria

- Every test 1–16 has a recorded result on the testbench, and on `fake-gcs-server` where the decision rule required it.
- The recommendation follows the decision rule, and lists the tests that need a real bucket.
- Every document in "Updates on completion" is updated, and no task still lists the stand-in as an open question.

## Open questions

- Does `adapters/gcs` use the JSON API client (the default) or the gRPC client? **Answered (operator, 2026-11-23): the JSON API client.** Objects are small (at most about 2 MB per bill), traffic is low and mostly cached, services scale to zero, and the JSON API has the widest stand-in support; gRPC is not needed for M1. Switching later is a constructor change inside `adapters/gcs`, rechecked with the conformance suite against a stand-in that supports gRPC. Recorded in [architecture § Adapter packages](../../../architecture/README.md#adapter-packages).
- **Result (2026-11-23, `task/M1-00`):** neither stand-in covers every test. `fake-gcs-server` 1.56.1 covers 1-13, 15, 16 and lacks fault injection (14); `storage-testbench` 0.45.0 covers 14 but gets 7 (no `Cache-Control` on downloads), 8 (ETag follows metageneration, not content) and 12 (no list paging) wrong; 17 (IAM) fails on both, as expected. Matrix and evidence: the task report; tests and `run.sh` in `test/spike/storage-emulator/`.
- Should the expected behaviors be confirmed against a real bucket in `dev` once it exists (stage C1), and who provides that bucket? **Answered (operator, 2026-11-23):** yes; a dedicated test bucket in `dev` (M1-15), tests run by the operator at C1 (M1-03b).
- If the result is "both", is running two stand-in images in CI acceptable? **Answered (operator, 2026-11-23):** no; `fake-gcs-server` only, with transient errors tested through an injected `http.RoundTripper`.
- Where does the spike's test code live once handed over: inside `adapters/gcs` tests, or discarded after M1-03b writes its own? **Answered (operator, 2026-11-23):** merged as-is at `test/spike/storage-emulator/` (its own module, not under the strict lint config); M1-03b moves the tests into `adapters/gcs` under strict lint and deletes `test/spike/`.
