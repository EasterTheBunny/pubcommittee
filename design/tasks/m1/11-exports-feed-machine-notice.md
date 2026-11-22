# M1-11 — Exports, Change Feed, and Machine-Access Notice

## Goal

Publish the M1 bulk exports and change feed so no one needs to scrape, and publish the machine-access notice and crawler directives that tell automated readers what is available and how to treat it.

## Read first

- [10 § Bulk exports](../../10-machine-access.md#bulk-exports), [§ Change feed](../../10-machine-access.md#change-feed), [§ Published policy](../../10-machine-access.md#published-policy), [§ Instruction injection](../../10-machine-access.md#instruction-injection)
- [architecture § Scheduled jobs](../../../architecture/README.md#scheduled-jobs) (export and feed builder)
- [11](../../11-ai-security.md) export rules

## Scope

**In**
- `cmd/job-exports`: per official version of tracked bills, write-once exports of bill text (original document plus parsed sections and lines), cited provisions with release dates, and change explanations with provenance; a catalog export refreshed each run; an export index with as-of times and licence.
- Change feed of M1 events: new catalog bills, newly tracked bills (with reasons), new versions, new or corrected explanations, policy versions, administrator log entries. Clients can request everything since a point in time.
- Feed and export routes served by `read-api` (M1-10) from the object store.
- Machine-access notice at a well-known location ([10 § Published policy](../../10-machine-access.md#published-policy)); which domains serve it is an open question: pointers to the spec, structured forms, exports, feed, rate limits, and the policy that machine reading is welcome and automated submission is prohibited; statement that `user` content is untrusted data (none exists in M1, but the notice is written for later).
- Crawler directives: allow all public content; disallow future sign-in and write routes.

**Out**
- Review item, aggregate, brief, and moderation exports: later milestones. Removal events beyond administrator untrack corrections: later.

## Deliverables

- `cmd/job-exports` and image
- Feed format and export formats documented in the M1-02 spec
- Machine-access notice and crawler directive files

## Requirements

- Each export states its as-of time and licence.
- Exports rebuild from source, never edit in place; old dumps are rebuilt, not retained ([10 § Bulk exports](../../10-machine-access.md#bulk-exports)).
- Content classes carried in every export record.
- Job idempotent per window; overlap-safe.
- Feed entries carry a timestamp and canonical address; cursors are opaque.

## Suggested breakdown

1. Export formats and paths; export index.
2. Per-version exports for tracked bills; catalog export.
3. Feed entry types for M1 events and the "since" query.
4. `job-exports` wiring, flags, metrics.
5. Machine-access notice text and well-known file; crawler directives.
6. Tests: rebuild determinism, as-of and licence present, feed paging.

## Acceptance criteria

- After a local pull and summarize run, exports exist in the local store for every tracked version, `read-api` serves them, and the feed lists the new versions.
- **Cloud phase:** the same after a run in `dev`.
- A client can read the feed since a given time and reach each item by canonical address.
- Notice and crawler directives are reachable without sign-in.

## Open questions

- Licence: **Decided:** official bill text and US Code provisions are US government works in the public domain, and say so; platform-generated content (explanations, summaries, comparisons) is published under CC0 1.0. Each export states both. User content licence is decided before M2 adds review items.
- Feed format: **Decided:** JSON with the API's collection convention (`{ "items": [...], "next": "<cursor>" }`), `GET /v1/feed?since=<RFC 3339>` and opaque cursors; each entry has a type, a day-granular time, and a canonical address.
- When does `job-exports` run in M1? **Decided for the local phase:** each step runs as its own command over stored data; nothing chains steps automatically ([README § Example data and local triggers](../README.md#example-data-and-local-triggers)). Automatic chaining is a cloud-phase question. Remaining for the cloud phase: [architecture § Scheduled jobs](../../../architecture/README.md#scheduled-jobs) runs it after each count batch, and M1 has no batcher.
- Notice and crawler directives: **Decided:** both the UI domain and the API domain; `robots.txt` applies per host. The notice lives on the UI domain and `GET /v1` links to it.
