# M1-05 — Manual Bill Pull and Storage

## Goal

Let an operator pull a bill from Congress's official sources on demand: its status and every official text version. Store the result exactly as the architecture specifies. That means the original XML write-once with its provenance, the parsed version, and the bill's catalog entry. M1 has no scheduled or feed-driven ingestion. Every pull is started by a person, and the same code runs with no network in tests through fixtures.

## Read first

- [MILESTONES § M1](../../MILESTONES.md#m1--real-bill-text-read-only) (manual pull; automatic ingestion out of scope)
- [architecture § Bill source](../../../architecture/README.md#bill-source) (sources, recommendation, data recorded per ingest)
- [architecture § Data placement](../../../architecture/README.md#data-placement) (object store paths, `bills` document)
- [architecture § Jobs](../../../architecture/README.md#jobs) (idempotent, overlap-safe, `--dry-run`)
- [01 § Ingestion](../../01-bill-ingestion-and-display.md#ingestion) and [§ Public catalog](../../01-bill-ingestion-and-display.md#public-catalog)
- [MILESTONES § Prior work, M1](../../MILESTONES.md#m1--real-bill-text-read-only-1) (newest-version-only gap, parse-on-request problem)
- GPO govinfo bulk data guides for `BILLS` and `BILLSTATUS`

## Scope

**In**
- Confirm the govinfo paths and formats for fetching one bill's status and each of its text versions; record them in [architecture § Bill source](../../../architecture/README.md#bill-source).
- `adapters/govinfo` implementing the `BillSource` fetch operations: bill status, and each text version's original XML.
- `adapters/fixturesrc` implementing the same from the example data in `fixtures/` (chosen by [M1-04](04-bill-xml-parser.md)), plus the catalog sample in `fixtures/catalog/` (about 200 bill-status records, added by this task).
- A pull of one bill, by id, that:
  - writes or updates its catalog entry in `bills`: identity, titles, chamber, session, sponsor, status, latest action, subject indexing, committee activity, related bills, `official_links` (bill page and each text version), `search_terms`, and the version list with source references;
  - hands the entry to selection policy evaluation ([M1-06](06-selection-policy.md));
  - for a tracked bill, fetches every text version not yet stored; stores the original byte for byte at `bills/{bill}/v/{n}/source.xml` with source URL, retrieval time, and SHA-256; parses it with [M1-04](04-bill-xml-parser.md); stores `bills/{bill}/v/{n}/text.json` with the parser version;
  - on a repeat pull, fetches only versions not yet stored, and compares any re-fetched original by SHA-256;
  - reports what it stored, and supports `--dry-run`.
- Records that downstream steps read after a new version is stored: citations ([M1-07](07-us-code-and-citations.md)), version comparison ([M1-08](08-version-history-and-stale.md)), generation ([M1-09](09-ai-generation.md)), and the feed ([M1-11](11-exports-feed-machine-notice.md)). How those steps start is an open question.
- End-to-end test with `memory` and `fixturesrc`; integration test on the `local` target with the filesystem stores.

**Out**
- Scheduled runs, change detection from the RSS feed or sitemap, watermarks, gap detection, and full listing walks. Automatic feed ingestion is out of M1 scope.
- Selection rules themselves ([M1-06](06-selection-policy.md)). Lineage candidates (M3). The `usccollector` adapter.

## Deliverables

- `internal/adapters/govinfo`, `internal/adapters/fixturesrc`
- The pull entry point (form decided by the open question below) and an operator guide for it
- Stored originals, parsed versions, and catalog entries for the M1 exit bills in the local store (and, in the cloud phase, in `dev`)
- Source notes in [architecture § Bill source](../../../architecture/README.md#bill-source)

## Requirements

- **Storage as the architecture specifies** ([architecture § Data recorded per ingest](../../../architecture/README.md#data-recorded-per-ingest)): original document write-once with source bytes, URL, retrieved-at, and SHA-256; parsed version write-once with sections, lines, anchors, and parser version; bill metadata in `bills`.
- **Write-once.** A stored version is never overwritten. A SHA-256 mismatch on re-fetch is reported as a failure and not applied.
- **Every version kept.** No newest-only shortcut.
- **Parsing in the pull, never on a reader's request.**
- **Untracked bills never have text fetched, stored, or re-rendered** ([01 § Public catalog](../../01-bill-ingestion-and-display.md#public-catalog)).
- **Idempotent and overlap-safe** ([architecture § Jobs](../../../architecture/README.md#jobs)).
- **Politeness:** one request at a time per host, backoff, conditional requests where supported, and a descriptive user agent naming the project with no personal contact details.
- Bill ids include jurisdiction and session (`us-119-hr-4127`). Sponsors are keyed on bioguide id, never name matching.

## Suggested breakdown

1. Source confirmation spike: read GPO guides, fetch samples for the exit bills, record paths and formats.
2. `BILLSTATUS` XML mapping to catalog entries; `official_links`; `search_terms` normalisation.
3. `adapters/govinfo` fetch operations with backoff and conditional requests.
4. `adapters/fixturesrc` and the fixture corpus (shared with M1-04).
5. Version pipeline: fetch, hash, store original, parse, store text.
6. Downstream records for new versions.
7. Pull entry point, dry run, metrics, operator guide.
8. End-to-end and `local`-target tests.

## Acceptance criteria

- With `fixturesrc`, pulling a tracked fixture bill stores its catalog entry, every original, and every parsed version; a second pull changes nothing.
- On the `local` target, the three exit bills are pulled from govinfo into the local store, with every official version stored and parsed.
- **Cloud phase:** the same pull in `dev`.
- Pulling an untracked bill stores no text (checked by a test).
- A forced SHA-256 mismatch is reported and does not overwrite.
- Stored paths and records match [architecture § Data recorded per ingest](../../../architecture/README.md#data-recorded-per-ingest).

## Open questions

- **Decided for the local phase:** a pull is a `job-ingest` run with a bill flag (`job-ingest --bill <id>`), against `fixturesrc` (the example data in `fixtures/`) or `govinfo`. The catalog for development is the sample in `fixtures/catalog/`; a full current-Congress catalog may be pulled locally into `.local-data/` but is never committed.
- **Cloud phase:** is a pull also an `admin-api` action, logged publicly? How does `bills track` in [M1-12](12-administration.md) lead to a pull without the M3 `worker`? How is the full current-Congress catalog kept in a deployed environment?
- Pulling an untracked bill: **Decided:** writes its catalog entry only; text is pulled once the bill is tracked ([01 § Public catalog](../../01-bill-ingestion-and-display.md#public-catalog)).
- How do downstream steps start after a pull? **Decided for the local phase:** each step runs as its own command over stored data; nothing chains steps automatically ([README § Example data and local triggers](../README.md#example-data-and-local-triggers)). Automatic chaining is a cloud-phase question.
- `ingest_state` in M1: **Decided:** yes, for failed pulls to retry and inspect ([M1-12](12-administration.md) `ingest failures`); no watermark.
- Which milestone takes automatic feed ingestion, and does `job-ingest` keep its architecture name for the manual pull in M1?
  - **Answered in part (M1-05):** the manual pull is `job-ingest --bill <id[,id]> [--source fixtures|govinfo] [--track] [--dry-run] [--verify]` (guide: `cmd/job-ingest/README.md`). Until [M1-06](06-selection-policy.md), `--track` tracks with reason `manual-pull`. The milestone for feed ingestion is an operator decision.
- How quickly must a new official version be pulled after publication ([01](../../01-bill-ingestion-and-display.md#open-questions)), when pulls are manual?
- Backfill depth: current Congress only, or earlier Congresses ([architecture § Open decisions](../../../architecture/README.md#open-decisions))?
  - **Decided (operator, 2026-11-22):** current Congress only.
