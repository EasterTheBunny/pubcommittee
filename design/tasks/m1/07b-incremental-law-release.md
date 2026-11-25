# M1-07b — Incremental US Code Release Ingest

After M1 exit (operator, 2026-11-25). [M1-09b](09b-law-aware-replanning.md) uses its changed-section list.

## Goal

Store a new US Code release point by fetching only the titles that release changed, and copying every unchanged title's sections from the stored previous release inside the object store. A new release then costs a few title downloads instead of the whole Code, and it finishes well inside one job execution's timeout.

## Read first

- [M1-07](07-us-code-and-citations.md) (release points, `LawSource`, write-once sections)
- [01 § Existing law freshness](../../01-bill-ingestion-and-display.md#existing-law-freshness)
- [architecture § Bill source](../../../architecture/README.md#bill-source) and [§ Scheduled jobs](../../../architecture/README.md#scheduled-jobs) (weekly existing-law check)
- [infra/README.md § Cloud runbook](../../../infra/README.md#cloud-runbook) (the current full-download procedure)
- The Law Revision Counsel's release point pages, which list the titles each release point updated

## Scope

**In**
- `adapters/uscode`: read which titles a release point updated, from the release point page or its download list.
- `LawSource` and `LawWriter` port additions as needed: the changed titles of a release point; a copy of a stored title's sections from one release to another inside the store. Use a server-side copy where the stored section holds no release fields; otherwise read, set the release, and write.
- `job-ingest --law` with no `--titles` on a new release: fetch the changed titles, copy the rest from the current stored release, then write the release record and move `current.json`, in that order.
- A fallback to the full download when the previous release is not stored, or the changed-title list cannot be read.
- Copied sections carry the new release's id and date, so every provision shown still names its release point.
- A changed-section list stored with the release: the sections of the new release whose text differs from the previous release, plus the added and removed ones. M1-09b reads it to skip bills that cite no changed section.

**Out**
- Schedules (M1-15).
- Changes to citation resolution (M1-07).

## Deliverables

- Changed-title reading in `internal/adapters/uscode`, with a recorded page fixture
- Incremental step in `internal/lawingest`, with conformance tests on the memory, fsobject and gcs adapters
- Changed-section list beside `release.json`
- Updated `infra/README.md` § Cloud runbook and `cmd/job-ingest/README.md`

## Requirements

- The release record and `current.json` are written only after every title of the release is stored, as today.
- A stored section is never overwritten. A copied section that exists already is confirmed byte for byte.
- Unknown is better than wrong: if the changed-title list is missing or ambiguous, fetch every title.
- Same politeness rules as M1-05.
- Progress lines keep the fields M1-07 logs (`law ingest started`, `law title stored`). An incremental run logs `mode: incremental`; a copied title logs `method: copy` and `from` (the release it was copied from); a fetched title keeps `method: fetch`. A fallback to the full download logs `mode: full`.

## Suggested breakdown

1. Confirm where the Law Revision Counsel lists a release point's updated titles; record a fixture.
2. Adapter method and tests for the changed-title list.
3. Copy port and adapter implementations, with a conformance test.
4. Incremental step and its fallback in `lawingest`; job report fields (`fetched`, `copied`).
5. Docs and runbook.

## Acceptance criteria

- With release A stored, ingest of release B fetches only B's changed titles and copies the rest. The stored B equals a full download of B, section for section.
- A stopped incremental run leaves A current, and a rerun completes B.
- Without a stored previous release, the step falls back to the full download.
- A dev run of a real new release finishes within the job's default timeout.

## Open questions

- Does a copied section need any change beyond its release id and date, for example a source hash of the title file it came from? Answer here.
- Is the changed-title list complete for every release point, including appendix titles (`5a`, `11a`, ...)? Answer here from evidence.
