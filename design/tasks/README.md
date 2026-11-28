# Tasks

High-level work packages for delivering milestones. Each task is sized for one top-level agent (or person) to own: it reads the task, breaks it into smaller sub-tasks for sub-agents, and is done when the acceptance criteria pass.

Tasks implement the feature docs in [`design/`](../README.md) using the stack in [`architecture/`](../../architecture/README.md). Tasks name a language, service, or vendor only where the architecture already chose it. Where the architecture is silent, or conflicts with the milestone, the task lists an open question and does not assume an answer. Answers go into `architecture/` (how it is built) or the matching feature doc (how it behaves), not only into the task file.

## M1 — Real bill text, read-only

Scope and exit criteria: [MILESTONES § M1](../MILESTONES.md#m1--real-bill-text-read-only). Architecture fit: [architecture § Milestone fit](../../architecture/README.md#milestone-fit). Environment stages, from local to cloud, and the operator and agent split: [m1/environments.md](m1/environments.md).

| # | Task | Depends on | Delivers |
| --- | --- | --- | --- |
| [M1-00](m1/00-storage-emulator-exploration.md) | Cloud Storage emulator exploration (cloud phase) | — | Tested choice of Cloud Storage stand-in for `adapters/gcs` (`storage-testbench`, then `fake-gcs-server` if needed); architecture and task updates |
| [M1-01](m1/01-backend-foundation.md) | Backend foundation | — | Go module, domain types and ports, memory adapter, conformance suites, `httpx`, `telemetry`, Dockerfile, `compose.yaml`, build-tag wiring, backend CI |
| [M1-02](m1/02-api-contract.md) | API contract | 01 | `api/openapi.yaml` for M1 read routes, generated Go and TypeScript types, contract tests, versioned response structure |
| [M1-03a](m1/03a-local-storage-adapters.md) | Local storage adapters | 01 | `adapters/fsobject`, `adapters/fsdocs`, `adapters/localauth`; `local` target on a shared data directory; no emulators |
| [M1-03b](m1/03b-cloud-storage-adapters.md) | Cloud storage adapters (cloud phase) | 00, 03a | `adapters/gcs` and `adapters/firestore`, passing the same conformance suites against emulators, then a real bucket |
| [M1-04](m1/04-bill-xml-parser.md) | Bill XML parser and anchors | 01 | Parser from official bill XML to sections, lines, depth, blocks, and stable anchors; answers to the line-unit and non-prose questions |
| [M1-05](m1/05-manual-bill-pull-and-storage.md) | Manual bill pull and storage | 01, 03a, 04 | `adapters/govinfo`, `adapters/fixturesrc`, operator-started pull of a bill's status and every text version, stored as the architecture specifies: write-once originals, parsed text, catalog entry |
| [M1-05b](m1/05b-version-ordering-and-referral.md) | Version ordering, code addresses, and referral collapse (M1 follow-up) | 05, 08, 09, 10, 11, 13, 14 | Versions ordered by stage rank of the version code only; addressed by code (`/v/eh`), positions removed, `/v1` changed in place; a referral (`rfs`/`rfh`) with the same text as the engrossed version is collapsed into it, with a referral note |
| [M1-06](m1/06-selection-policy.md) | Selection policy and tracked set | 05 | Versioned policy document, evaluation in `job-ingest`, markup status, tracked-set disclosure counts |
| [M1-07](m1/07-us-code-and-citations.md) | US Code and citation resolution | 03a, 04 | `adapters/uscode`, provision store, citation extraction and resolution, unresolved list, freshness flags, measured resolution rate |
| [M1-07b](m1/07b-incremental-law-release.md) | Incremental US Code release ingest (after M1 exit) | 07 | Fetch only the titles a new release point changed; copy the rest from the stored previous release; changed-section list |
| [M1-08](m1/08-version-history-and-stale.md) | Version history and stale classification | 04 | Version-to-version line mapping and item classification (unchanged, moved, changed, removed), "changed since" data for old links |
| [M1-09](m1/09-ai-generation.md) | AI generation | 03a, 04, 07, 08 | `Generator` port, `adapters/claude`, `adapters/fakegen`, `job-summarize`; accuracy checks against cited lines (deterministic validators, grader, planted errors, operator sample, CRS coverage) and `test/accuracy/report.md`; no steward review until stewards exist; no model calls in CI |
| [M1-09b](m1/09b-law-aware-replanning.md) | Law-aware re-planning of explanations (after M1 exit) | 07, 09; 07b optional | Generation keys carry a law-inputs hash: a changed cited provision re-plans its explanations only; old explanations stay readable from old links, marked superseded |
| [M1-09c](m1/09c-latest-version-explanations.md) | Explanations on the latest version only (M1 follow-up) | 05b, 09 | Inline explanations (`change-explanation`, `bill-explanation`) planned only for the latest version; superseded versions keep bill and version change summaries; stored outputs stay; `job-summarize --explain-version` explains one older version on request |
| [M1-10](m1/10-read-api.md) | `read-api` | 02, 03a | Public read service: bills, catalog, versions, lines by anchor, explanations, policy, log; provenance, caching, rate limits |
| [M1-10b](m1/10b-explanation-line-pinning.md) | Explanation line pinning (M1 follow-up) | 09, 10, 13 | Each explanation pinned to the lines it cites, not the lowest-to-highest range; explanations of one section overlap only on shared lines; no regeneration |
| [M1-11](m1/11-exports-feed-machine-notice.md) | Exports, change feed, machine-access notice | 03a, 05, 10 | `job-exports`, change feed, well-known access notice, crawler directives |
| [M1-12](m1/12-administration.md) | Administration | 03a, 05, 06 | `admin-api`, `pubctl`, public log entries for every admin action, roster log route for operator changes |
| [M1-13](m1/13-web-ui-real-data.md) | Web UI on real data | 02, 10 | M0 demo archived to `demo/`, still on Netlify; SvelteKit app server-rendered from `read-api`: catalog, bill pages, explanations, versions, canonical addresses |
| [M1-14](m1/14-installable-app.md) | Installable app on real data | 13 | Service worker caching for real resources, installability and offline checks in CI |
| [M1-15](m1/15-infrastructure-and-deploy.md) | Infrastructure (cloud phase) | 01, 03b | Public modules in `infra/`; private deployment repository roots for `dev`, `staging`, `prod`; shared registry; API edge; scheduled jobs; secret and pattern checks |
| [M1-16](m1/16-acceptance.md) | M1 acceptance | all | Local gate: every exit criterion proved on a local machine. Hosted gate: the same in `staging` for invited readers. Measured targets for M3, recorded answers to M1 open questions |
| [M1-17](m1/17-continuous-delivery.md) | Continuous delivery (cloud phase) | 01, 15 | GitHub App trigger from this repository; private deploy workflows: build, push, deploy `dev` by digest, promote to `staging` and `prod` behind approvals, rollback, deploy records |
| [M1-17b](m1/17b-release-approval-gate.md) | Release approval gate (required for the M3 pilot release) | 17 | A release tag never reaches `staging` or `prod` without the operator's approval for that environment |

### Phases

M1 is built in two phases ([m1/environments.md](m1/environments.md)):
- **Local phase.** Every task except 00, 03b, 15, and 17 is built and accepted on a local machine with the `memory` and `local` build targets: filesystem stores, live public sources, and an Anthropic API key for live generation. No cloud account, emulator, or deployed environment is needed. The phase closes with the M1-16 local gate.
- **Cloud phase.** 00 and 03b add the Google Cloud storage adapters, 15 the infrastructure, and 17 delivery. Each task's criteria marked *cloud phase* are then checked in `dev` or `staging`, and the phase closes with the M1-16 hosted gate.

### Order

```
Local phase
01 ──┬─► 02 ─────────────┬─► 10 ─┬─► 11
     │                   │       └─► 13 ─► 14
     ├─► 03a ─┬─► 05 ─► 06 ─► 12
     │        └─► 07 ─┐
     ├─► 04 ─┬────────┼─► 09
     │       └─► 08 ──┘
                              all local ─► 16 local gate

Cloud phase
start: next operator sample (M1-09) ─► 00 ─► 03b ─► 15 ─► 17 ─► cloud-phase criteria of 05, 06, 09, 10, 11, 12, 13, 14 ─► 16 hosted gate
```

Parallel tracks once 01 lands: contract and read path (02, 10, 13, 14); ingest (03a, 04, 05, 06); law and generation (07, 08, 09); operations (12). Cloud-phase work may start early, as long as no local-phase task waits on it.

## M1 boundaries that every task respects

- **Local first.** Every task's local acceptance closes without a cloud service. Cloud connections belong to the cloud phase; a task that needs one names it as a cloud-phase criterion ([m1/environments.md](m1/environments.md)).
- **Bill text is pulled manually.** An operator starts each pull; M1 has no scheduled or feed-driven ingestion ([MILESTONES § M1](../MILESTONES.md#m1--real-bill-text-read-only)). Text is stored as [architecture § Data recorded per ingest](../../architecture/README.md#data-recorded-per-ingest) specifies.
- **No accounts, signals, or briefs.** No SQL, no `write-api`, no sign-in. Anything that needs them stays a placeholder that names its milestone ([`app/src/lib/placeholders.ts`](../../app/src/lib/placeholders.ts)).
- **No steward program yet.** Outputs that will later need steward review are labelled *auto-generated*; nothing is labelled *steward-checked*. No steward review or evaluation set exists until stewards do ([M1-09](m1/09-ai-generation.md)).
- **Global principles hold** ([design/README § Global principles](../README.md#global-principles)), especially: official versions immutable, every privileged action public and attributed, machines read and people write.
- **Nothing private in the repository.** Use placeholders (`<domain>`, `<project>`, `<region>`) and fake ids ([architecture § What stays private](../../architecture/README.md#what-stays-private)).
- **Jurisdiction-general.** Bill ids carry jurisdiction and session (`us-119-hr-4127`); no code assumes one Congress.

## Example data and local triggers

- **Example data.** Real bills from govinfo, committed under `fixtures/` and read by the `fixturesrc` adapter, so every task builds and tests offline. Content does not matter for development; coverage does. [M1-04](m1/04-bill-xml-parser.md) chooses the bills and names three as the M1 exit bills; M1-05 adds a catalog sample; M1-07 adds the cited US Code provisions. Each bill's `status.xml` keeps its CRS summaries, which M1-09 uses as references for bill-summary coverage.
  - Layout: `fixtures/bills/<bill-id>/status.xml`, `fixtures/bills/<bill-id>/v/<version-code>/source.xml`, `fixtures/catalog/`, `fixtures/uscode/`, and `fixtures/README.md` with each file's source URL, retrieval date, SHA-256, and what it covers.
  - Originals byte for byte. At most 2 MB per bill and 20 MB in all. Exit bills are short (about 30 KB or less per version), because live generation costs scale with bill length; M1-04 lists pre-selected exit bills. US government works, so committing them is fine; nothing else goes in `fixtures/`.
  - Larger local pulls (full catalog, whole US Code release points) go to `.local-data/`, never into git.
- **Local triggers.** In the local phase each pipeline step is its own command over stored data: `job-ingest --bill <id>` (pull, parse, policy evaluation, citations, version comparison), `job-ingest --publish-policy v1|<file> --reason <text>` and `job-ingest --evaluate` (re-evaluate the stored catalog), `scripts/law-ingest` (US Code release point and citation sets, until wired into a job), `job-summarize`, `job-exports`. Nothing chains steps automatically; automatic chaining, scheduling, and whether a pull is an `admin-api` action are cloud-phase questions.

## M2 — Interactive prototype

Scope and exit criteria: [MILESTONES § M2](../MILESTONES.md#m2--interactive-prototype). Architecture fit: [architecture § Milestone fit](../../architecture/README.md#milestone-fit). Status: [STATUS.md § M2](STATUS.md#m2).

| # | Task | Depends on | Delivers |
| --- | --- | --- | --- |
| [M2-00](m2/00-decisions-and-prerequisites.md) | Decisions and prerequisites | — | Operator answers to the decisions M2 tasks cannot assume; domain, OAuth clients, VAPID keys |
| [M2-01](m2/01-write-ports-and-domain.md) | Write ports and domain types | M1 | Account, session, review item, signal, moderation, role, notification, snapshot types; ports; `Keys` port; memory adapter; conformance suites |
| [M2-02](m2/02-postgres-adapter.md) | Postgres adapter | 01 | `adapters/postgres`, migrations, Postgres in `compose.yaml`, conformance in CI |
| [M2-03](m2/03-cloud-keys.md) | Cloud key adapter (cloud phase) | 01 | `adapters/kmsgcp`, key rotation procedure |
| [M2-04](m2/04-api-contract-writes.md) | API contract for review and writes | 01 | `read-api` review routes and `write-api` routes in `api/openapi.yaml`; `content_class` `user`; generated types; contract tests |
| [M2-05](m2/05-write-api-sign-in-sessions.md) | `write-api`: sign-in, sessions, write barriers | 02, 04 | `cmd/write-api`, Google and Facebook sign-in (tier 1), local stand-in provider, sessions, Origin and anti-forgery checks, per-account rate limits, test-user gate |
| [M2-06](m2/06-review-items.md) | Line comments and suggested edits | 04, 05 | Persisted, anchored review items; stale state on new versions; `read-api` review routes |
| [M2-07](m2/07-signals-and-markup-requests.md) | Support, bill-level signals, markup requests | 05, 06 | One private, withdrawable signal per person per item; markup requests with private reasons; own-signal list |
| [M2-08](m2/08-batcher-and-published-counts.md) | `job-batcher` and published counts | 07 | Batched counts, "fewer than N", snapshots, brief eligibility, `Snapshots` read port |
| [M2-09](m2/09-notifications.md) | Inbox and device notifications | 07, 08 | Inbox route, `job-notify`, `adapters/webpush`, content-free pushes, encrypted subscriptions |
| [M2-10](m2/10-moderation-and-steward-actions.md) | Basic moderation and steward actions | 06 | Flag, remove with reason, public moderation log; mark well formed, wording fix, re-anchor; test role grants |
| [M2-11](m2/11-brief-page.md) | Brief page from live data | 08 | Brief data route and a page that renders without scripts; no delivery |
| [M2-12](m2/12-share-links-and-cards.md) | Share links and preview cards | 06, 08 | Links for every shareable target; cards with no sharer identity, position, or user text in preview fields |
| [M2-13](m2/13-injection-and-inference-defenses.md) | Instruction-injection and inference defenses | 06, 08, 12, 14 | Seeded injection corpus and cross-service checks; generation never reads user content; batch-window privacy test |
| [M2-14](m2/14-exports-and-feed.md) | Review items in exports and the feed | 06, 08, 10 | Review items in bulk exports with per-bill pseudonyms; review and removal events in the feed |
| [M2-15](m2/15-web-ui-interactive.md) | Interactive web UI | 05–12 | Sign-in, side-by-side view, comment and suggestion forms, signals, markup requests, inbox, steward and moderator controls, share buttons; writes work without scripts |
| [M2-16](m2/16-infrastructure.md) | Infrastructure for writes (cloud phase) | 00, 02, 03, 05 | Domain, `write-api`, serverless Postgres, KMS, secrets, schedules for `job-batcher` and `job-notify` |
| [M2-17](m2/17-testers-and-charter.md) | Tester sessions and draft charter | 15 | Session plan and rubric, tester findings, draft public charter, decided batch interval and N |
| [M2-18](m2/18-acceptance.md) | M2 acceptance | all | Local gate and hosted gate in `staging`; evidence per exit criterion |

### Phases

M2 is built in two phases, as M1 was ([m2/environments.md](m2/environments.md)):

- **Local phase.** Every task except 03 and 16 is built and accepted on a local machine with the `memory` and `local` build targets: M1's filesystem stores, Postgres in a container, `localkeys`, a stand-in identity provider with seeded test accounts, and a locally generated development VAPID key. No cloud account, domain, or provider client is needed. Each task adds what it builds to `scripts/local.sh`, so the whole prototype runs with `scripts/local.sh up`. The phase closes with the M2-18 local gate and the operator's walkthrough of every feature on the local stack.
- **Cloud phase.** 03 adds Cloud KMS, 16 the infrastructure for writes, and M2-00's cloud decisions and operator steps the domain and real sign-in. Each task's criteria marked *cloud phase* are then checked in `dev` or `staging`; the tester sessions (17) run in `staging`, and the phase closes with the M2-18 hosted gate.

### Order

```
Local phase
00 local decisions ─────────────────────────────────────── (before the tasks they block)
01 ──┬─► 02 ─┬─► 05 ─► 06 ─┬─► 07 ─► 08 ─┬─► 09
     │       │             ├─► 10 ───────┼─► 14 ─┐
     └─► 04 ─┘             │             ├─► 11  ├─► 13
                           └─────────────┴─► 12 ─┘
                                   05–12 ─► 15 ─► 17 (materials, charter, walkthrough)
                                    all local ─► 18 local gate ─► operator walkthrough on scripts/local.sh

Cloud phase
00 cloud decisions and operator steps ─► 03 ─► 16 ─► cloud-phase criteria of 02, 05–15 ─► 17 tester sessions in staging ─► 18 hosted gate
```

Parallel tracks once 01 lands: storage (02) and contract (04); then the write path (05, 06, 07, 08) with moderation (10) beside it. The UI (15) can start on the contract's generated types with a memory-target stack. Cloud-phase work may start early, as long as no local-phase task waits on it.

### M2 boundaries that every task respects

- **Local first.** Every task's local acceptance closes without a cloud service, a domain, or a real provider account; cloud connections are cloud-phase criteria ([m2/environments.md](m2/environments.md)).
- **Viewable locally.** What a task builds runs in `scripts/local.sh up`, so every finished feature can be used on the local stack.
- **Tier 1 only, test users only.** Google and Facebook sign-in for invited testers; no phone, address, or ID verification (M5) and no email.
- **No delivery to offices.** The brief is a page; `job-briefs`, delivery, responses, and the responsiveness record are M3.
- **No two-party sign-off, no steward program.** Steward and moderator roles are test grants for single-person actions ([M2-10](m2/10-moderation-and-steward-actions.md)).
- **Counts only in batches.** No public count changes between batches or appears in a write's response.
- **Machines read and people write.** No programmatic write path; generation never takes user content as input.
- **Nothing private in the repository**, as in M1: placeholders for `<domain>`, project ids, and client ids; secrets only in Secret Manager.

## Status and running tasks with agents

[STATUS.md](STATUS.md) records each task's state and branch.

Tasks can be run in parallel by agents, three levels deep, with the project skills in `.claude/skills/`:

| Level | Who | Skill | Does |
| --- | --- | --- | --- |
| 1 | Orchestrator: the top-level session, started with `/run-milestone m1` | `run-milestone` | Sets per-task CPU and memory budgets from host usage, picks ready tasks, launches one task-orchestrator per task in its own git worktree, splits tasks or scales up models when churn crosses a threshold, merges finished branches into `m1-integration`, runs the whole application as a Compose project for acceptance, applies doc changes, updates `STATUS.md` |
| 2 | `task-orchestrator` agent (`.claude/agents/`), Sonnet by default | `run-task` | Owns one task file: starts its resource-limited containers, splits the task into small sub-tasks for Haiku, runs them in parallel waves, enforces strict lint, runs only its own `test/acceptance/<id>/` checks, watches churn, commits on `task/<id>`, reports |
| 3 | `task-worker` agent, Haiku | `do-subtask` | Does one sub-task inside the files it owns, running commands only with `docker exec` in its task's container; cannot start agents or containers |

Only the orchestrator edits shared docs (`architecture/README.md`, `design/*.md`, this file, `STATUS.md`); the other levels propose changes in their reports. No level runs operator-only steps ([m1/environments.md § Operator and agent](m1/environments.md#operator-and-agent)), merges into `main`, or pushes. Agent commits are unsigned (`git -c commit.gpgsign=false`) and stay on `task/*` and `m1-integration`; the user lands work on `main` with their own signed squash commit.

## Task file format

Each task file has:
- **Goal**: one paragraph on the outcome.
- **Read first**: the docs and sections the owner must read.
- **Scope**: in and out.
- **Deliverables**: files, services, and records produced.
- **Requirements**: rules the work must meet, with their source.
- **Suggested breakdown**: sub-tasks sized for sub-agents, in a workable order. The owner may change it.
- **Acceptance criteria**: checks that close the task.
- **Open questions**: gaps or conflicts in the docs that block or shape the task, and where each answer goes. A task never assumes an answer to one.
