# M1-05b — Version Ordering, Code Addresses, and Referral Collapse

M1 follow-up (operator, 2026-11-25), from a check of how versions are ordered for the exit bills. Related: [M1-05](05-manual-bill-pull-and-storage.md), [M1-08](08-version-history-and-stale.md), [M1-10](10-read-api.md), [MILESTONES § M4 known issues](../../MILESTONES.md#known-issues).

## Goal

A bill's versions are ordered by their version codes alone, in legislative stage order, and addressed by code (`/v/eh`), not by position (`/v/3`). The latest version is the most advanced text, each comparison runs against the version before it, and an address never changes when a version is added. A referral version whose text is the same as the engrossed version is not listed as a version of its own. The engrossed version carries a note that the bill was referred, with the date.

## Seen

- `versions` in `internal/billstatus/billstatus.go` sorts by the BILLSTATUS `<date>` and breaks ties by version code in alphabetical order. Dates are day-only. H.R. 3427 has `rh` and `eh` both dated 2025-09-15, so the list is `ih, eh, rh, rfs`: reported after engrossed. If the bill had stopped at `eh`, `rh` would be served as the latest, and comparisons and `changed_since` would run against the wrong version.
- BILLSTATUS does not list text versions in a dependable order, and one stage can have more than one version (for example, reports from more than one committee).
- `read-api` addresses versions by 1-based position (`VersionPath` in `api/openapi.yaml`). A version inserted into the middle of the list renumbers every version after it.
- Stored versions are already keyed by code (`fixtures/bills/<bill>/v/<code>/source.xml`); `architecture/README.md` still writes the path as `bills/{bill}/v/{n}`.
- H.R. 4313, H.R. 3062, and H.R. 3427 each have an `rfs` version whose `<legis-body>` text is the same as `eh`. Only the markup differs, so the files are not byte-identical. Each `rfs` is served as its own version (for H.R. 4313, `/v/4`), planned for generation on its own, and compared with `eh` with no change.

## Decided

- **Order by code only** (operator, 2026-11-25). Post dates are not used for ordering. They are still stored and served as each version's date.
- **Addresses by code** (operator, 2026-11-25). Position addresses are removed.
- **`/v1` changed in place** (operator, 2026-11-25). A one-time exception to the breaking-change rule in `api/MACHINE-API.md` (a breaking change ships as `/v2`), taken before M1 exit while readers are invited only. Position addresses already served in `dev` and `staging` stop working, with no redirect. The exception is recorded in `api/MACHINE-API.md`.

## Read first

- [M1-05](05-manual-bill-pull-and-storage.md) (status and version pull, write-once originals)
- [M1-08](08-version-history-and-stale.md) (comparisons, `versiondiff.Pairs`)
- [M1-09](09-ai-generation.md) (`PlanAll` walks the version list)
- [M1-10](10-read-api.md), [M1-11](11-exports-feed-machine-notice.md), [M1-13](13-web-ui-real-data.md), and [M1-14](14-installable-app.md) (version addresses, exports and feed, version switcher, service worker cache rules)
- [02](../../02-versions-and-review-lifecycle.md) (versions and the review cycle) and [10](../../10-machine-access.md) (canonical addresses)
- `api/MACHINE-API.md` (contract versioning) and [architecture § Ingest](../../../architecture/README.md) (write-once version paths)

## Scope

**In**
- **Stage rank table.** A total order over every GPO bill version code, checked against govinfo's list of bill version codes, so no two codes share a rank. A draft for House bills: `ih` < `rih`/`rch` < `rh` < `rfh`/`rds` < `eh` < `rfs` < `eas` < `enr`, with the Senate codes (`is`, `rs`, `es`, `eah`, ...) placed the same way, and the codes drawn level here given distinct ranks. The table lives in one place and is used by ingest and by every reader of the version list.
- **Order.** `ListVersions` returns versions by stage rank only. A code that is not in the table is stored, left out of the version list, and reported as a note on every pull until the table covers it: unknown is better than wrong.
- **Code addresses.** In `api/openapi.yaml`, the version path parameter becomes the lower-case version code or `latest`, on every route that takes one, `/compare/{m}` included. `latest` still answers with `address.resolved_from_latest` true and a `Content-Location` of the pinned code address. Version objects and summaries carry the code; the position field, if any, is removed. Regenerated Go and TypeScript types.
- **Every user of addresses.** `read-api` routes, rate-limit route patterns, `admin-api`, exports, the change feed, canonical addresses and links in `app/` (`app/src/lib/addresses.ts`, `app/src/lib/server/api.ts`), service worker cache rules (`app/src/service-worker/policy.ts`), the contract tests and spec walk, and the grader's links (`internal/generate/grade.go`).
- **Same text.** A referral version (`rfs`, `rfh`) whose parsed sections and lines are equal to the version before it (the engrossed `eh` or `es`) is the same version. Equal means: the stored comparison ([M1-08](08-version-history-and-stale.md)) classifies every line as unchanged, with nothing moved, added, or removed. Equal bytes are not required.
- **Collapse.** A collapsed referral is not in the version list that `read-api`, exports, the change feed, comparisons, and generation use. Its original XML is still stored byte for byte, with its source URL, retrieval time, and SHA-256 (write-once, [M1-05](05-manual-bill-pull-and-storage.md)). A request for its code (`/v/rfs`) redirects to the version it collapsed into. A referral whose text differs stays a version of its own.
- **Referral note.** The version a referral collapses into carries a note: the referral's type (for example "Referred in Senate"), its date, and its version code. A field in `api/openapi.yaml` (for example `referrals` on `Version` and on the version summaries), served by `read-api` and shown by the UI next to the version in the switcher and on the bill page, without scripts.
- **Generation.** `PlanAll` plans no tasks for a collapsed referral. Outputs already stored for one (H.R. 4313 `rfs` in `dev` and `staging`) stay stored and are not served.
- **Docs.** `api/MACHINE-API.md` (code addresses, the recorded exception), `architecture/README.md` (storage path by code, version order), [02](../../02-versions-and-review-lifecycle.md) and [10](../../10-machine-access.md) where they describe version order or canonical addresses.

**Out**
- Carrying explanations to a changed version, or to a version with unchanged text that is not a referral ([MILESTONES § M4 known issues](../../MILESTONES.md#known-issues)).
- Skipping inline explanations on versions that are not the latest (a separate cost-saving follow-up).
- Collapsing any version other than a referral.
- Rewriting finished task files that mention position addresses; they describe what was built at the time.

## Deliverables

- Stage rank table and sort, with table tests that include the H.R. 3427 same-day pair, a list out of BILLSTATUS order, and an unknown code
- Collapse rule built on the stored comparison, with tests over the exit-bill fixtures and one synthetic `rfs` with changed text
- Version list used by `read-api`, exports, feed, comparisons, and generation leaves out collapsed referrals and unknown codes
- Contract change to code addresses and the referral field, regenerated types, `read-api` mapping, contract tests
- UI on code addresses, with the referral note in the version switcher and bill page, and a Playwright check that runs without scripts on H.R. 4313
- Docs listed under Scope

## Requirements

- Stored originals are never changed or deleted.
- No model call and no regeneration. Generation keys already name the version by code (`GenKey.Version`); they do not change.
- Pages stay complete without scripts.
- Unknown is better than wrong: if the comparison for a referral is missing, the referral stays a version of its own.

## Suggested breakdown

1. Stage rank table from govinfo's version code list; sort and tests.
2. Collapse rule and the filtered version list; tests over fixtures.
3. Generation and comparison use the filtered list; `job-summarize --dry-run` check.
4. Contract change, regenerated types, `read-api`, `admin-api`, exports, feed, contract tests.
5. UI addresses, referral note, service worker rules, Playwright check.
6. Docs.

## Acceptance criteria

- H.R. 3427 lists `ih, rh, eh`, in that order; `latest` resolves to `eh`.
- H.R. 4313, H.R. 3062, and H.R. 3427 each list three versions. Each `eh` carries a referral note with type "Referred in Senate", the `rfs` date, and code `rfs`.
- `/v1/bills/us-119-hr-4313/v/eh` answers the engrossed version; `/v/rfs` redirects to it; `/v/3` is not a valid address.
- A synthetic `rfs` with one changed line stays a version of its own, after `eh`.
- A version with a code missing from the table is stored, not listed, and named in the pull's notes.
- The `rfs` originals are still stored byte for byte, with unchanged SHA-256.
- `job-summarize --submit --dry-run` plans no task for a collapsed referral.
- The bill page of H.R. 4313 links versions by code and shows the referral note without scripts.
- Cloud phase: `read-api` in `dev` serves three versions for H.R. 4313 at code addresses, with the referral note on `eh`.

## Open questions

- **More than one version of one stage (task-orchestrator, from evidence).** Check the 119th Congress House archive for repeated stages (for example two committee reports) and record how their codes appear. If two versions of one bill can share a code, a code address is not unique, and that is reported to the operator.
