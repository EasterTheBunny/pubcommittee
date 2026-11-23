# M1-16 acceptance evidence: local gate

Run `r20261122-1412`, 2026-11-23 (UTC clock of the machine), branch `task/M1-16`. Local gate only: the hosted gate (staging) is cloud phase and is not attempted here. No model call was made; the generated outputs come from the live run recorded in `test/accuracy/report.md`.

Reproduce: copy the data root of the live run to `.local-data/`, write `.task/compose.limits.yaml` (limits and labels for `read-api`, `app`, `playwright`), then `RUN_ID=<id> sh test/acceptance/M1-16/stack.sh`. It starts one Compose project (`pc-m1-16`: `read-api` on the `local` target over `.local-data`, the SvelteKit app on Node 24, Playwright on the app's network, plus a dev container), prepares the data (`prepare.sh`), runs the HTTP checks and the admin checks (`run.sh`, `check.mjs`, `admin.sh`), then the browser checks (`e2e/`), and tears everything down.

Result of the last run: 71 PASS lines from `check.mjs` and `admin.sh`, 12 of 12 Playwright tests, exit 0.

## Exit criteria of MILESTONES § M1

| # | Criterion | Evidence | Result |
| --- | --- | --- | --- |
| E1 | Three real bills, one with multiple versions, display correctly with explanations | `check.mjs`: all 3 exit bills have 4 official versions (12 version pages), every version page equals its `read-api` resource in anchors, order and text (363 lines), official content class and public-domain licence on each; 12 of 12 summaries served; 75 served explanations all labelled `auto`, content class `platform`, and the page shows the "Auto-generated" label. Browser: `e2e/real-bills.spec.ts` reads each bill from the catalog without scripts. Review of text against the official source: `test/acceptance/M1-05/run.sh` (live pull byte-identical to `fixtures/`) and `test/acceptance/M1-04/run.sh` (DTD validation, 0 parser warnings). Explanation spot-check: `test/accuracy/report.md` (graded statements, planted errors) and `test/accuracy/operator-sample.md` (operator graded 2026-11-22: 30/30 agree with the grader) | pass |
| E2 | Measured citation resolution rate and explanation accuracy on a sample, with a target set for M3 | Citation resolution: `test/acceptance/M1-07/report.md`. Accuracy: `test/accuracy/report.md`. Measured values and proposed targets: see "Measured targets for M3" below. The targets are proposals for the operator | pass (measured); targets await the operator |
| E3 | Answers recorded for the M1 open questions in 01 (line unit, non-prose content) | `design/01-bill-ingestion-and-display.md` § Open questions: "Decided (M1-04)" for the line unit, non-prose content and continuation lines; `test/acceptance/M1-04/run.sh` passes (parser corpus incl. tables) | pass. Formulas: no real example exists in the 119-1 House archive; deferred by the operator on 2026-11-22 |
| E4 | An automated client can fetch any line range by canonical address, without running page scripts, and the structured form matches the page | `check.mjs`: for every version of the 3 exit bills, every line, every block range and every section range (485 ranges): `read-api` resource vs page HTML (fetched without scripts) compare anchors, text and canonical address; page carries `rel="alternate"` to the resource; older-version ranges carry `changed_since` (359 ranges); `Accept: application/json` on a page answers 303 to the resource; no cookies. Also `app/tests/*.spec.ts` (CI app job, 42 tests) | pass |
| E5 | Structure of machine responses documented and versioned | `test/acceptance/M1-02/run.sh` passes (Redocly lint, generated code current, contract harness, `api/MACHINE-API.md` versioning and deprecation policy); `check.mjs` fetches `/v1`, `/v1/openapi.yaml` and the notice's links | pass. The v1 overlap period (12 months) is proposed and awaits the operator |

## In-scope list of MILESTONES § M1

| Item | Evidence | Result |
| --- | --- | --- |
| Manual pull from the official source, text stored as the architecture specifies | `M1-05/run.sh` (fixtures; `LIVE=1` pull is stage L2 and was done in M1-05); `prepare.sh` pulls catalog and corpus bills with `job-ingest` | pass |
| Catalog for the current Congress, tracked set by the published policy (pilot) | `check.mjs`: 205 catalog entries (180 eligible, 8 in review at the first run, 15 not a candidate), all with a markup status and an official link, tracked entries list reasons and policy version, excluded kinds carry a reason; `M1-06/run.sh` | pass. Backfill is the 200-entry sample plus the exit bills; a full-Congress pull is operator-run |
| Privileged administration, every action in the public log | `admin.sh`: `pubctl` against the local `admin-api` in one container selects a catalog bill, re-runs ingestion, reverses the selection; each needs a reason (a call without one is refused); `read-api` in another container serves the entries and the web app's `/log` page shows them; no principal or token in the log; `read-api` has no admin route. `M1-12/run.sh` (authorisation, refusals, atomic log, roster) | pass. Operators are the fixed local operator and the roster placeholders in `M1-12/run.sh` |
| Installable app, offline shell, previously opened text readable offline | `e2e/real-bills.spec.ts`: no installability errors from the browser's own check on real data; an opened real version (hr-4313 v2, 22 lines) reads offline with its text and the offline label; the catalog shows the out-of-date label offline. `M1-14/run.sh` (16 tests), CI `pwa` job commands run locally | pass for desktop on localhost; mobile and staging installs are cloud phase |
| Public catalog: browse and search, official links, markup status | `check.mjs` (search by title words, markup statuses on the page, untracked bill shows no text); `e2e` (without scripts) | pass. 29 of 205 entries have no bill-page link, see Findings |
| Stable anchors, line-level deep links | E4 above; `e2e` line-address click without scripts goes to the canonical range page | pass |
| Citation resolution against the US Code, unresolved listed | `check.mjs`: hr-4313 10 resolved, 0 unresolved; hr-3062 52 resolved, 4 unresolved (listed on the page); hr-3427 none (freestanding text); `M1-07/run.sh` | pass |
| Change explanations for a target set of bills, quality measured | E1, E2 | pass |
| Version history and stale classification between real versions | `check.mjs`: 5 stored comparisons per exit bill (consecutive pairs and each older version against the latest), the first-to-latest comparison of hr-4313 and hr-3427 classifies changed lines, version switcher links all 4 versions, older-version ranges say whether words changed since; `M1-08/run.sh` | pass |
| Read access for machines: canonical address, rendered pages, structured form with provenance | E4; each range answer carries provenance, as-of and content class (485 checked) | pass |
| Bulk export of text, cited provisions, change explanations per version | `check.mjs`: 50 exports in the index (bill text, existing law, change explanations), each served, each states licence, as-of and content class; an existing-law export holds cited provisions of a real citation set (M1-11 follow-up recheck); `M1-11/run.sh` also with `NOTICE_URL` | pass |
| Change feed | `check.mjs`: feed pages through all entries (version, explanation, bill-tracked, policy, log-entry kinds present), every sampled entry reaches its item by canonical address | pass |
| Long-lived caching of immutable versions; published rate limits | `check.mjs`: latest pinned version is `immutable` with an ETag, `If-None-Match` answers 304; limits are published in `api/MACHINE-API.md` (ranges). The stack raises the limits for test runs; the limiter itself is covered by `M1-10/run.sh` | pass; older pinned versions are cached short by design (M1-08) |
| Machine-access notice and crawler directives | `check.mjs`: `/.well-known/machine-access.json` on the app (automated submission prohibited, untrusted-data statement) and `robots.txt` without sign-in; `/v1` links the notice | pass |
| Selection policy and tracked-set disclosure | `check.mjs`: `/v1/selection-policy`, the `/policy` page without scripts (rules and version), `tracked-set.json` export with counts by topic and sponsor party | pass. Policy v1 is stored locally; publishing it publicly is an operator step |

## Every task's acceptance script

| Script | Where it ran | Result |
| --- | --- | --- |
| `M1-02`, `M1-04`, `M1-05`, `M1-06`, `M1-07`, `M1-08`, `M1-09`, `M1-11`, `M1-12` | dev container (pc-dev:orch), repository mounted | all exit 0 (logs: 2 s to 30 s each). M1-09 runs on `fakegen` only (no model call). M1-11 also with `NOTICE_URL=http://app:3000`: pass |
| `M1-01`, `M1-03a`, `M1-10` runtime parts | compose acceptance loop (`read-api` on `local`, `seed`, `otel-collector`, `acceptance`), fresh data root | pass: 33 ok lines, no FAIL; the Go and Node scripts skip there by design |
| `M1-13`, `M1-14` | Playwright container on a network with a memory-target `read-api` (the CI app and pwa job commands) | pass. CI `app` job commands: `npm ci`, `npm run check` (0 errors), `npm run lint` (Prettier clean), `npm run build`, `playwright test` with the stub API: 42 passed. CI `pwa` job: M1-14 16 passed. M1-13 including the archived `demo/` check and build: pass |
| `M1-16` | `stack.sh` | pass |
| `go test -tags memory -race ./...`, `test/ci/size.sh`, `deps.sh`, `tags.sh` | dev image | all ok; sizes in the table below |

No Go or app code was changed by this task, so golangci-lint was not rerun.

## Items earlier tasks deferred to M1-16

| Item (source) | Result |
| --- | --- |
| M1-13: long-page check on a large real bill | `us-119-hr-3115` (introduced, 2,546 lines, 7 sections, pulled with `job-ingest --source fixtures` using a minimal BILLSTATUS written by `prepare.sh`): renders all lines, scrolls to the last anchor, a line-address click moves in place in 0.2 s. Timings below. Also `us-119-hr-3944` reported text (328 lines) pulled. Pass |
| CI `app` and `pwa` job commands run locally | pass (see above). They have not run on GitHub |
| M1-11: existing-law exports on real citation sets | pass: the index lists `existing-law.*` for each exit-bill version; one holds `usc/...` provisions |
| M1-03a: cross-container visibility | pass: `admin-api` and `job-ingest` in the dev container write; `read-api` and the app in other containers read it (`admin.sh`, `prepare.sh`) |
| M1-10: live contract test (`CONTRACT_BASE_URL`) | covered by the M1-10 and M1-02 scripts above; `check.mjs` adds the page-versus-resource comparison |

## Measured targets for M3 (from existing reports, no new runs)

| Measure | Measured | Source | Proposed M3 target (operator decides) |
| --- | --- | --- | --- |
| Citation resolution rate | 64.0% (128/200); exit bills alone 93.9% (62/66) | `test/acceptance/M1-07/report.md` | Resolution rate is bounded by sources not yet mapped (26 public-law, 17 chapter, 16 Act-section, 3 title-section citations unresolved by design); raise it with an Act-and-section mapping source |
| Precision of `resolved` | 100% (128/128); labels by the task agent, steward review pending | same | 100% on a steward-labelled sample |
| Detection | 100% (200/200) | same | 100% |
| Statements supported (graded) | bill-explanation 100% (28/28), bill-summary 100% (92/92), change-explanation 88.4% (199/225), version-changes 3/3 | `test/accuracy/report.md` | zero published statements graded contradicted; contradicted at or below 1% per task (the report's proposal) |
| Statements contradicted | change-explanation 4.4% (10/225), all served | same | same |
| Outputs ready / unverified / invalid | 75 / 11 / 0 of 86; 11 withheld (not served) | same | as the report |
| Planted-error catch (deterministic) | changed-number 96.8%, invented-number 100%, evaluative wording 100%, nonexistent anchor 100%, reversed effect 66.7% (2/3) | same | nonexistent-anchor 100% by the deterministic check |
| Grader-to-operator agreement | 30/30 (100%) on 30 grader-supported statements; no contradicted statements in the sample | same | operator sets; next sample should include contradicted and unverified statements |
| Spend of the live run | at most 7.5 + 0.9 USD of the 20 USD approval | same | |

Sizes (`test/ci/size.sh`, bytes against budget): read-api 19,157,152 / 20,000,000; admin-api 17,723,552; job-ingest 17,432,736; job-exports 16,740,512; accuracy-report 13,050,016; pubctl 7,114,912; job-summarize 27,668,640 / 30,000,000 (all within budget, read-api at 96%).

Page timings (headless Chromium, local, no throttling; `.task/m1-16-timings.jsonl` of the last run; navigation timing in ms):

| Page | Lines | Bytes | Response end | Load |
| --- | --- | --- | --- | --- |
| hr-4313 v4 | 26 | 51,805 | 11 | 306 |
| hr-3062 v4 | 45 | 107,868 | 37 | 61 |
| hr-3427 v4 | 23 | 43,599 | 11 | 29 |
| hr-4313 v2, section 3 lines 12-20 | 9 | 24,589 | 6 | 82 |
| catalog (first 50 of 205) | n/a | 150,742 | 79 | 95 |
| hr-3115 v1 (largest real bill here) | 2,546 | 1,037,287 | 43 | 324 |
| hr-3115 v1, section 2 lines 2-51 | 50 | 36,578 | 17 | 265 |

## Findings (1 and 2 fixed on `task/M1-16-findings`; the rest filed to the owner)

1. **Catalog entries without a legislature bill page** (M1-05, M1-13; 01 § Public catalog): 29 of 205 entries carry only text-version links because their BILLSTATUS has no `legislationUrl`. The 01 text says every entry links the bill page. **Fixed:** the link is derived from the bill id when the record has none (`internal/ingest/billpage.go`), labelled "Congress.gov (derived from the bill number)"; `job-ingest --evaluate` fills stored entries. Gate rerun on live data: 0 of 205 entries without a bill-page link.
2. **Public log order within a day** (M1-09, M1-12, M1-03a): generation entries are written with a full timestamp, administrator entries with the day only, and the log sorts by that time then id. On the same day all generation entries sort before administrator entries, so with more than 50 generation entries the first page of `/log` shows no administrator action; the API says "newest first". `admin.sh` follows the older-entries links. **Fixed:** every public log entry now stores the day only (design/10 and design/11 require coarse public timestamps), with a write sequence within the day; both stores order by day, then sequence, newest first, stable across pages. Entries stored before the change keep their earlier order within their day. Gate rerun on live data: admin checks pass.
3. **Page weight** (M1-13): the 2,546-line bill is a 1.0 MB page (about 400 bytes per line), roughly twice the text because the hydration payload repeats the resource. Load is 0.3 s locally; over a real network this is the cost to watch. Candidate for M2.
4. **Line-address click with scripts** (M1-13): on a whole-version page the click moves in place to `#<anchor>`; without scripts it opens the canonical range page. Both are by design; noted for the reader session.
5. **Corpus bills have no BILLSTATUS** (M1-04/M1-05): the large-bill check needed a synthetic one (`prepare.sh`, scratch directory, never in `fixtures/`).
6. **Change summaries**: 2 of 12 versions have a served change summary (hr-3062 and hr-3427, version 2); hr-4313 has none (known from M1-09: the number 1866 came from uncited lines).

## Pending operator steps (not attempted)

- Done 2026-11-22: operator sample graded, 30/30 agreement. Still open: decide on withholding contradicted statements (10 of 225 served) and on further paid retries.
- Repoint Netlify at `demo/` (M1-13).
- Publish selection policy v1 publicly with `pubctl` (M1-06, M1-12); confirm the draft topic terms.
- Confirm the v1 overlap period (12 months) in `api/MACHINE-API.md`; the older-pinned-version caching decision (M1-08).
- Invite readers and record what they could not understand (16-acceptance scope).
- Cloud-phase gates: hosted gate in `staging`, mobile and desktop install there, IAM and roster checks (M1-12), M1-03b, M1-15, M1-17, whether any exit bill is tracked by administrator selection in `dev` with a public log entry.

## M1 open questions still unanswered

Product, policy, legal and spending questions are not answered here; each is listed with who must answer it.

| Question | Source | Who answers |
| --- | --- | --- |
| Subject terms for each focus topic, and who maintains them (v1 terms are drafts) | 01, M1-06, architecture § Open decisions | Operator, before the pilot |
| Request threshold for the steward queue; steward and pilot capacity and how a decline is worded | 01, architecture | Operator (product) |
| When the administrator phase rule ends (first two-party steward pair, or a fixed date); when the pilot topic filter ends | 01, M1-12 | Operator |
| How quickly a new official version must appear after publication; which milestone takes automatic feed ingestion | 01, M1-05 | Operator |
| Whether excluded measure kinds get a bill-level signal or stay out | 01 | Operator (product) |
| Title matching versus waiting for subject indexing for pilot bills | 01 | Operator (product) |
| Accuracy bar for display without steward review, and withholding contradicted statements (10 of 225 served) | 01, M1-09 | Operator, from `test/accuracy/report.md` |
| Effort level and model per generation task; which model grades | M1-09 | Operator (spending) |
| Source that maps Act-and-section references to US Code sections (OLC classification tables, popular-names list) | M1-07 | Operator |
| Freshness flags after the first pull, write-once citation sets block them | M1-07 | Design owner (02) |
| Pinned older-version answers cached short rather than immutable | M1-08 | Operator |
| Whether a pull is an `admin-api` action in the cloud phase, how automatic chaining starts steps, schedules and cadence | M1-05, M1-06, M1-09, M1-11, M1-12, M1-15 | Operator (cloud phase) |
| Second parser version storage path for re-parse | M1-12 | M1-03/M1-04 owners |
| A reset-from-fixture snapshot for `.local-data` (for example `make local-reset`) | M1-03a | Operator |
| Hosting questions: web host for `dev` and `staging`, `<domain>`, shared registry project, budget alerts, secret scanner, CD approvals, release tags, Firestore index order, post-deploy contract check | M1-13, M1-15, M1-17, architecture § Open decisions | Operator (cloud phase) |
| Storage stand-in questions (JSON versus gRPC, real bucket, one or two stand-ins) | M1-00 | Operator and M1-00 (cloud phase) |
| Catalog search: keyword matching enough for M1 | M1-03b, architecture | Closed for the local gate by the search check above; confirm on the full catalog |
| Which open questions must be answered to close M1 and which carry to M2 | 16 | Operator |

Answered during M1 and recorded as decided: line unit, non-prose content, continuation lines, anchor scheme, parser package, DTD method (M1-04); range syntax, spec linter, spec publication (M1-02); `ListChanged`, size budgets (M1-01); catalog scale (M1-03a); changed versus removed rule and non-consecutive comparisons (M1-08); existing-law ingest as a `job-ingest` mode (M1-07); licences, feed format, notice placement (M1-11); rate-limit mechanism and published ranges (M1-10); offline bound (M1-14); backfill depth, current Congress only (operator, 2026-11-22).
