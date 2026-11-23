# Architecture

Implementation architecture for PubCommittee from M1 onward. The feature docs in [`design/`](../design/README.md) stay stack-agnostic; this is where stack, hosting, and storage choices are recorded. M0 remains a static site ([MILESTONES](../design/MILESTONES.md#m0--proof-of-concept)); from M1 it is archived at `demo/` and still served from its existing Netlify site, while `app/` becomes the M1 app.

**Status:** draft. **Decided:**
- Web UI: the SvelteKit app in [`app/`](../app/) on **any Node-capable web host**, talking to the backend **only through its APIs**.
- Backend: **Google Cloud**, scale-to-zero.
- Backend language: **Go**, one small static binary per service or job, in a minimal image ([Language and images](#language-and-images)).
- All data access through swappable adapters behind interfaces, chosen at build time ([Data access layer](#data-access-layer)).
- API contract: **REST** over JSON, specified in OpenAPI ([API contract](#api-contract)).
- Bill source: Go ingest pulling official XML directly from GPO's govinfo bulk data, behind a swappable source port ([Bill source](#bill-source)).
- AI summarization: batch jobs calling Claude through Claude on Google Cloud, official text in and labelled platform content out, steward-reviewed ([AI-generated content](#ai-generated-content)).

Provider prices and limits named here must be re-checked before commitment.

## Constraints

| Constraint | Source | Consequence |
| --- | --- | --- |
| Scale to zero | Cost: long idle periods between legislative events, spikes around markups and shared links | Serverless compute, per-request databases, no always-on servers, no fixed-cost networking |
| Document-heavy, SQL-light | Bills, versions, comments, suggestions, briefs are nested, versioned, read-mostly documents | Document store and object storage hold almost everything; a small SQL database holds what needs uniqueness and exact counting |
| Minimal personal data | [08](../design/08-onboarding-and-verification.md), [09](../design/09-integrity-moderation-and-privacy.md), [11](../design/11-ai-security.md) | Social sign-in only, no passwords, no stored email or name by default, pseudonymous keys |
| Signals private, totals public | Principle 5 | Private data lives in a separate store, unreachable from the public read path |
| Machines read, people write | Principle 8, [10](../design/10-machine-access.md) | Public reads are cacheable GETs; writes go only through an authenticated API |
| Batched counts | [11](../design/11-ai-security.md) | Counts are computed on a schedule and published as files, never read live |
| Fast cold start | Scale to zero means many requests meet a cold instance | Go static binaries in near-empty images; small services with few dependencies |
| Encapsulated components | Separate hosting for UI and backend | The web tier holds no backend credentials and never sees sessions; it is just another API client |

## Overview

```
  people, crawlers, agents
        │                                   │
        ▼                                   ▼
┌───────────────────────────┐   ┌───────────────────────────────────────────┐
│ Web host  <domain>        │   │ Firebase Hosting  api.<domain>            │
│ SvelteKit UI              │   │ CDN for GET responses (cache by headers)  │
│ server rendering + CDN    │   └──────┬──────────────────────────┬─────────┘
│ no credentials, no cookies│          │ GET /v1/... (public)      │ /v1/... writes, /auth
└────────────┬──────────────┘          ▼                           ▼
             │ server-side GET   ┌──────────────┐          ┌──────────────┐
             └──────────────────►│ read-api     │          │ write-api    │
   browser: GET + credentialed   │ Go, Cloud Run│          │ Go, Cloud Run│
   writes to api.<domain> ──────►│ read ports   │          │ write ports  │
                                 └──┬────────┬──┘          └─┬────┬────┬──┘
                                    ▼        ▼               ▼    ▼    ▼
                          ┌──────────────┐ ┌─────────────┐ ┌─────┐ ┌──────────┐ ┌─────┐
                          │ Cloud Storage│ │ Firestore   │ │Fire-│ │ Postgres │ │ KMS │
                          │ bill text,   │ │ public docs │ │store│ │ PRIVATE  │ │     │
                          │ snapshots,   │ │             │ │     │ │          │ │     │
                          │ exports, feed│ │             │ │     │ │          │ │     │
                          └──────▲───────┘ └──────▲──────┘ └─────┘ └────▲─────┘ └─────┘
                                 │                │                    │
                          ┌──────┴────────────────┴────────────────────┴──────┐
                          │ Go jobs on Cloud Run jobs: ingest · batcher ·      │
                          │ briefs · exports · detector      + worker service  │
                          └────────────────────────────────────────────────────┘
```

Four rules hold the shape together:
1. **The public read path never touches SQL.** `read-api` is built without any SQL or key adapter. A bug or injection in the read path cannot reach signals or identities.
2. **Counts are files.** The batcher reads SQL on a schedule, applies suppression, and writes snapshot JSON. Nothing serves a live count.
3. **Only `write-api`, `worker`, and jobs hold SQL and key credentials.** Each holds only what it needs.
4. **The web tier is an ordinary client.** The web host renders pages from `read-api` responses, the same public JSON any machine can fetch ([10](../design/10-machine-access.md)). It never receives a session cookie or backend credential.

## Components

| Component | Role | Scales to zero | Notes |
| --- | --- | --- | --- |
| Web UI | Renders every page; serves static assets | Host-dependent | SvelteKit on any Node host, using that host's adapter. Pages render fully without scripts. Immutable version pages cache long at the host's CDN |
| API edge (Firebase Hosting) | Custom domain, TLS, and CDN in front of the Cloud Run APIs | Yes | No fixed-cost load balancer. Caches public GETs by `Cache-Control`; never caches credentialed responses |
| `read-api` | Public, read-only JSON: bills, versions, lines by anchor, explanations, threads, suggestions, issues, snapshots, exports, feed | Yes | Read ports only. Every response carries version, as-of time, provenance, and content class ([10](../design/10-machine-access.md)) |
| `write-api` | Sign-in, sessions, comments, suggestions, signals, flags, office responses | Yes | Only service the browser sends credentials to |
| `worker` | Handles queued tasks (re-anchoring, promotion of accepted bills, re-render hints) | Yes | HTTP service Cloud Tasks pushes to; not publicly routed |
| `admin-api` | Privileged operations interface for administrators ([Administration](#administration)) | Yes | Not part of the product; no UI; callable only with Google Cloud IAM credentials |
| Jobs | Ingest, batcher, briefs, exports, detector | Yes | Run to completion; triggered by Cloud Scheduler |
| Object store | Immutable bill text (source XML + parsed JSON), existing-law excerpts, rendered briefs, exports, count snapshots, feeds | Yes (storage only) | Versioned, write-once paths: `bills/{bill}/v/{n}/text.json` |
| Document store | Public, mutable documents | Yes (per operation) | See [Data placement](#data-placement) |
| SQL database | Private, relational data | Yes, with resume delay | Serverless Postgres. Never reachable from `read-api` |
| Key management | HMAC and encryption keys | Near zero | Keys never leave the service; code calls sign and encrypt operations |
| Queue | Re-anchoring after a new version, promotion of accepted bills | Yes | Small volume |

Every backend unit is a standalone container image ([Service packaging](#service-packaging)).

## Web tier

- **Hosting:** any web host that runs Node and SvelteKit, deployed from `app/`. The host is not part of the architecture; swapping it means changing the SvelteKit adapter and the API base URLs.
- **The contract is the API.** The UI depends on nothing in the backend except the `read-api` and `write-api` HTTP interfaces ([API contract](#api-contract)). The backend does not know or care which host serves the UI.
- **Rendering:** SvelteKit server rendering on the host, plus prerendering for pages that never change (official version text by canonical address). Pages are complete HTML without scripts. In M1 nothing is prerendered: `adapter-node` renders every page on request from `read-api`, so the build needs no API.
- **Configuration (M1-13):** `READ_API_URL` (server-side calls) and `PUBLIC_API_URL` (links and `303` redirects for `Accept: application/json`), read from `process.env`. The server keeps a short in-memory cache of `GET` responses, because every reader's requests reach `read-api` from the web host's one address and share its rate-limit allowance; each environment sets `RATE_LIMITS` for that address.
- **Data:** server-side `GET` calls to `read-api`. No direct access to any store.
- **Caching:** immutable version pages cache long at the host's CDN; pages showing review items use a short TTL with stale-while-revalidate, so comments appear within minutes without waking the backend on every view.
- **Signed-in features:** run in the browser. Server-rendered pages are always the signed-out view; the browser then calls `write-api` with credentials for "my signals", composing, and supporting.
- **Domains:** the UI at `<domain>` and the APIs at `api.<domain>`, under one registrable domain. That keeps the session cookie same-site, which browsers that block third-party cookies require.
- **Cross-origin rules:** `write-api` allows credentialed CORS only from `https://<domain>` and checks `Origin` on every write. `read-api` allows any origin without credentials.
- **What the web host never sees:** session cookies (they are host-only on `api.<domain>`), sign-in redirects, signals, or any backend credential.

### Progressive web app
The UI is a **progressive web app** and must be installable ([08](../design/08-onboarding-and-verification.md#installable-app)).
- **Manifest** served by the web host: name, icons, theme, start URL, `display: standalone`.
- **Service worker** served from the UI's origin, registered on first visit. Responsibilities:
  - **App shell** cached for offline start.
  - **Official text** (`read-api` immutable version resources and the matching pages) cached cache-first: they never change, so a cached copy is always correct. Bounded to the 50 most recently opened version pages; least recently opened evicted first.
  - **Review items, counts, and the catalog** network-first with a short fallback copy, labelled as possibly out of date when offline.
  - **Never caches credentialed responses** (`/v1/me*`, `/auth/*`, any write) and never queues writes for later; a write made offline fails visibly.
  - **Receives device notifications** ([Notifications](#notifications)) and opens the inbox on tap.
- **Build:** the SvelteKit service-worker support in `app/`, versioned with each deploy so a new release replaces old caches.
- **Requirement checks in CI:** installability (manifest and service worker present and valid) and an offline load of a cached version page.

## Data placement

### Object store (immutable or regenerated files)
- Official versions: original XML and parsed sections/lines with anchors. Written once at ingestion; never changed ([01](../design/01-bill-ingestion-and-display.md)).
- Version comparisons: `bills/{bill}/compare/{from}..{to}/{method}.json`, write-once, where method is `<parser version>+versiondiff/1` with `/` written as `_` ([02](../design/02-versions-and-review-lifecycle.md)).
- Existing-law provisions cited, with source effective date: `law/usc/<release>/t<title>/<section>.json` and `law/usc/<release>/release.json` (write-once), `law/usc/current.json` (replaceable pointer). Citation sets: `bills/<bill>/v/<ver>/citations/<method>.json` (write-once), method `billxml/1+citations/1+usc@<release>`.
- Count snapshots per bill and item, per batch.
- Bulk exports and the change feed ([10](../design/10-machine-access.md)).
- Rendered brief documents for delivery and export ([07](../design/07-legislator-brief-and-responses.md)).

Bill text lives here rather than in the document store because large bills exceed per-document size limits, and because immutable files cache perfectly. `read-api` serves these files with long-lived cache headers, so the API edge answers most requests without waking it.

### Document store (public, mutable)

| Collection | Key | Contents |
| --- | --- | --- |
| `bills` | bill id | Catalog entry for every bill (status, titles, sponsors, subject indexing, committee activity, related bills, `official_links`, `markup_status`, exclusion reason, `search_terms`); for tracked bills also version list, lineage links, issue id, and `tracked` (since, policy version, reasons) |
| `selection_policy` | version | The published bill selection policy ([Bill selection](#bill-selection)) |
| `markup_decisions` | decision id | Public accept or decline of a requested or nominated bill: bill, outcome, reason, policy version, published request total, surge disclosure, signing stewards ([Markup requests](#markup-requests)) |
| `markup_queue` | bill id | Bills awaiting a steward decision and why (threshold reached, nomination). Read only by `write-api` steward routes; not published until decided |
| `explanations` | bill/version/section | Explanation, provenance flag, steward edit history |
| `threads` | thread id | Anchor; `comments` sub-collection with author pseudonym, body, timestamp (day), moderation state |
| `suggestions` | suggestion id | Anchor, title, replacement, reason, author pseudonym, steward status, stale state; `revisions` sub-collection |
| `issues` | issue id | Competing bills, neutral comparison, sign-offs |
| `responses` | suggestion id | Office response, status, reason |
| `moderation_log` | action id | Public, sanitized entries ([09](../design/09-integrity-moderation-and-privacy.md)) |
| `briefs` | bill/week | Brief contents as delivered |
| `ingest_state` | source/Congress | Ingest watermark and failed items to retry ([Bill source](#bill-source)) |
| `generations` | task/bill/version/target/prompt/model | Status, pointer to the stored output, review state, steward edits ([AI-generated content](#ai-generated-content)) |
| `sessions` | random id | Session record with expiry (time-to-live deletion) |
| `rate_limits` | hashed key + window | Short-lived counters (time-to-live deletion) |

Documents never store an account id. Authorship is stored two ways:
- `author_pseudonym`: the public display string, per bill ([11](../design/11-ai-security.md)).
- `author_key`: `HMAC(k_author, account_id)`, used only by `write-api` to check edit rights. Stripped from every public response and export.

### SQL (private, small)

| Table | Columns (sketch) | Why SQL |
| --- | --- | --- |
| `accounts` | `id`, `created_on` (day), `tier`, `display_name`, `us_at_signup` (bool), `status` | Referential integrity for everything below |
| `identities` | `provider`, `subject_hmac` (unique), `account_id` | One account per sign-in identity; linking several providers |
| `signals` | `signal_key` (PK), `item_id`, `item_kind`, `value`, `tier_at_signal`, `created_day`, `withdrawn` | Exactly one signal per person per item, enforced by the primary key; exact aggregation for the batcher and auditor. Markup requests are stored here with `item_kind = markup_request` |
| `my_signals` | `account_id`, `items_encrypted` | Lets a person list and withdraw their own signals; source of their notifications |
| `roles` | `account_id`, `role`, `scope`, `granted_by`, `start`, `end` | Steward, moderator, office grants ([05](../design/05-roles-and-stewardship.md)) |
| `snapshots` | `item_id`, `batch`, `published_count` | Record of what was published, for audit and consistent suppression |
| `push_subscriptions` | `id`, `account_id`, `endpoint_encrypted`, `keys_encrypted`, `created_day`, `last_ok_day` | One row per opted-in device. No item ids. Deleted on revoke, sign-out, or when the push service reports the subscription gone |
| `notify_state` | `account_id`, `last_event_cursor`, `last_read_cursor` | Where each person's inbox and device notifications are up to |
| `request_reasons` | `signal_key` (PK), `bill_id`, `reason`, `created_day` | Optional reason given with a markup request. No account id; shown to stewards without names; deleted with the request |

`signal_key = HMAC(k_signal, account_id ‖ item_id)`. The table holds no account id, so a copy of the database alone shows totals but not who signalled. Checking or withdrawing a signal recomputes the key through the key service. `my_signals` is encrypted with a separate key so a person can still see their own list.

Why signals are not in the document store: one-per-person uniqueness, frequent increments on popular items (document stores throttle sustained writes to a single document), and exact, reproducible totals for the audit all favor a relational table.

## Data access layer

No service talks to a database, bucket, queue, or key service directly. Every service depends on **interfaces (ports)** stated in domain terms, and each storage technology is an **adapter package** that implements some of them. Which adapters a service contains is chosen **at build time** with Go build tags, so a binary links only the adapters, and SDKs, it was built with.

### Goals
- Swap storage technology per port without touching service code: Firestore today, another document store later, in-memory for tests.
- Enforce privacy boundaries at build time: `read-api` is built without any adapter that can reach signals, identities, or keys.
- Run every service locally and in tests with no cloud account.
- Keep vendor SDKs, types, and credentials out of services and binaries that do not need them.

### Ports
Ports are grouped by the boundary they sit behind. A service receives only the group it needs.

| Group | Port | Responsibility | Default adapter |
| --- | --- | --- | --- |
| Public read | `BillText` | Get immutable version text, sections, lines by anchor | `adapters/gcs` |
| | `PublicDocs` | Read bills, explanations, threads, suggestions, issues, responses, moderation log, briefs | `adapters/firestore` |
| | `Snapshots` | Read published count snapshots | `adapters/gcs` |
| Public write | `ReviewItems` | Create and revise threads, comments, suggestions; flags; office responses | `adapters/firestore` |
| | `Moderation` | Record actions and appeals in the public log | `adapters/firestore` |
| Platform write | `PlatformDocs` | Write the public documents the platform itself changes: catalog entries and `tracked` (`bills`), `selection_policy` versions, `explanations`, `generations`, `ingest_state`; and `ApplyLogged(ctx, change, entry)`, which writes a change and its public log entry atomically or neither | `adapters/firestore` |
| Private | `SignalLedger` | Set, withdraw, and check one signal per key; aggregate by item and day | `adapters/postgres` |
| | `Identities` | Look up or create an account by identity hash; link providers | `adapters/postgres` |
| | `Accounts` | Tier, display name, roles, notification preference and device subscriptions | `adapters/postgres` |
| | `Keys` | HMAC and encrypt/decrypt by named key; keys never leave the adapter | `adapters/kmsgcp` |
| Session and limits | `Sessions` | Create, read, expire sessions | `adapters/firestore` |
| | `RateLimits` | Increment and check hashed counters with expiry | `adapters/firestore` |
| Publishing | `Publisher` | Write immutable text, snapshots, exports, feeds | `adapters/gcs` |
| | `Queue` | Enqueue tasks for the worker | `adapters/cloudtasks` |
| Ingestion | `BillSource` | List changed bills since a watermark; fetch bill status and each text version's original XML | `adapters/govinfo` |
| | `LawSource` | List and fetch existing-law release points (US Code) | `adapters/uscode` |
| Notification | `Notifier` | Send an encrypted Web Push message to a subscription; report subscriptions that are gone | `adapters/webpush` |
| Generation | `Generator` | Submit summarization tasks as a batch, collect results; one synchronous call for steward-requested regeneration | `adapters/claude` |

M1 notes (from M1-01):
- The public read group holds only `BillText` and `PublicDocs` in M1; `Snapshots` arrives in M2.
- `PublicDocs` also has `GetSelectionPolicyVersion` (M1-10).
- Ingest (M1-05): `BillFetcher` (`FetchRecord`, `FetchOriginal`) sits beside `BillSource`; `ListChanged` and watermark ingestion stay out of M1. `OfficialVersion` gains `ParserVersion` and an unserialised `Source`; `IngestState` gains `Failed`. `CatalogEntry` gains `search_terms`, `sponsor_id`, `sponsor`, `introduced`, `latest_action`, `policy_area`, `committees`, and `related`; `Bill` gains `sponsor_id` (bioguide). The pipeline is `internal/ingest.DefaultSteps()`, an ordered list of named steps (`fetch-record`, `catalog`, `policy`, `versions`, `after-store`, `write-bill`, `write-catalog`, `ingest-state`) that later tasks extend with `After` and `Replace`.
- Existing law and citations (M1-07): `LawReader`/`LawWriter` and `CitationReader`/`CitationWriter` (`internal/data/citations.go`), in `PublicReadPorts.Law`/`Citations` and `PlatformPorts.LawStore`/`CitationStore`; `fsobject` and `memory` implement all four; `gcp`/`emulator` leave them nil (routes answer 404). A version with no stored citation set answers 404.
- Comparisons (M1-08): `ComparisonReader` in the public read group (`PublicReadPorts.Comparisons`), `ComparisonWriter` in the platform group (`PlatformPorts.ComparisonStore`); `fsobject` and `memory` implement both, and `gcp`/`emulator` wiring leaves the reader nil (route answers 404) until M1-03b.
- `PlatformDocs` also has `PutBill`, `GetIngestState`, and `PutIngestState`. `ApplyLogged` takes a `Change`, which carries exactly one of a bill, catalog entry, policy, or explanation, plus a `LogEntry`.
- `BillSource` declares `ListChanged(ctx, since, page)` in M1. Manual pulls ([M1-05](../design/tasks/m1/05-manual-bill-pull-and-storage.md)) pass an empty watermark.

### Interface rules
1. **Domain types only.** Ports take and return types from `internal/data` (`Anchor`, `Thread`, `SignalKey`, `Snapshot`). No vendor types (document references, timestamps, rows, SDK errors) cross a port.
2. **Domain operations, not generic queries.** `SignalLedger.Set(ctx, key, item, value)` is an idempotent upsert; there is no `Query(sql)` or `Collection(name)`. Each adapter decides how to make the operation atomic.
3. **Typed errors.** Adapters wrap failures in `data.ErrNotFound`, `ErrConflict`, `ErrUnavailable`, `ErrRateLimited`, `ErrInvalid`; services check them with `errors.Is` and never inspect vendor error codes.
4. **`context.Context` first.** Every operation takes a context for deadlines and cancellation.
5. **Opaque cursors.** Pagination uses an opaque string the adapter owns.
6. **Adapters own their schema.** Collections, indexes, tables, and migrations live in the adapter package, not the service.
7. **Config validated at startup.** Each adapter constructor takes a typed config struct and fails fast on missing values, without network calls. Credentials come from workload identity, never from config files.
8. **No runtime plugin loading.** The adapter set is fixed when the binary is built. An environment variable cannot switch a production service to a different backend.

Sketch:

```go
// internal/data/signals.go
type SignalLedger interface {
	Set(ctx context.Context, key SignalKey, item ItemRef, v SignalValue, tier Tier) error
	Withdraw(ctx context.Context, key SignalKey) error
	Get(ctx context.Context, key SignalKey) (SignalValue, error) // ErrNotFound if none
	// Aggregate returns exact totals for the batcher and auditor; never exposed to the read path.
	Aggregate(ctx context.Context, item ItemRef, until Day) (Totals, error)
}

// internal/data/groups.go
type PublicReadPorts struct {
	BillText   BillText
	PublicDocs PublicDocs
	Snapshots  Snapshots
}

// PlatformPorts are held by ingest, generation, exports, and admin-api; never by read-api or write-api.
type PlatformPorts struct {
	PublicReadPorts
	PlatformDocs PlatformDocs
	Moderation   Moderation
	Publisher    Publisher
}

type WritePorts struct {
	PublicReadPorts
	ReviewItems ReviewItems
	Signals     SignalLedger
	Identities  Identities
	Accounts    Accounts
	Keys        Keys
	Sessions    Sessions
	RateLimits  RateLimits
	Queue       Queue
}
```

### Build-time wiring
Each service has one **wiring file per build target**, selected by a build tag. Only the selected file is compiled, and Go links only packages that compiled code imports, so unselected adapters and their SDKs never enter the binary:

```
cmd/write-api/
  main.go
  wiring_gcp.go      //go:build gcp     → adapters/firestore, gcs, postgres, kmsgcp, cloudtasks
  wiring_local.go    //go:build local   → adapters/fsobject, fsdocs, localkeys, localauth (filesystem; no cloud service)
  wiring_emulator.go //go:build emulator → adapters/firestore, gcs against emulators, adapters/localkeys
  wiring_memory.go   //go:build memory  → adapters/memory, adapters/localkeys
  wiring_none.go     //go:build !gcp && !local && !emulator && !memory
```

`wiring_none.go` references an undefined identifier, so a build with no target tag fails to compile instead of producing a binary with no storage.

Each wiring file defines the same function, `wire(...)`, which `main.go` calls to get the service's port group.

Build: `CGO_ENABLED=0 go build -tags gcp -trimpath -ldflags="-s -w" ./cmd/write-api`.

`cmd/read-api` wiring files import read-port adapters only. CI runs `go list -deps -tags gcp ./cmd/read-api` and fails if `adapters/postgres`, `adapters/kmsgcp`, or their SDKs appear; a `depguard` rule in the linter enforces the same in review. This makes "the public read path never touches SQL" a property of the build, not a convention.

### Adapter packages

| Package | Implements | Backed by |
| --- | --- | --- |
| `internal/data` | Port interfaces, domain types, errors, conformance tests | — |
| `internal/adapters/memory` | Every port | Process memory; seeded from the M0 mock data |
| `internal/adapters/firestore` | `PublicDocs`, `ReviewItems`, `Moderation`, `PlatformDocs`, `Sessions`, `RateLimits` | Firestore |
| `internal/adapters/gcs` | `BillText`, `Snapshots`, `Publisher` | Cloud Storage |
| `internal/adapters/postgres` | `SignalLedger`, `Identities`, `Accounts` | Any Postgres (Neon in production) |
| `internal/adapters/kmsgcp` | `Keys` | Cloud KMS |
| `internal/adapters/localkeys` | `Keys` | Keys from a local file; development only, excluded from `gcp` builds |
| `internal/adapters/fsobject` | `BillText`, `Snapshots`, `Publisher` | Local filesystem, same paths as the bucket; write-once by hard-linking a synced temporary file to the final path (exclusive and atomic), atomic rename for replaceable files, content hash as `ETag`. Local only, excluded from `gcp` builds |
| `internal/adapters/fsdocs` | `PublicDocs`, `ReviewItems`, `Moderation`, `PlatformDocs` | One JSON file per document on the local filesystem; atomic rename, one lock file (`<root>/.lock`) serializing writes and change-plus-log operations. The catalog has no index file: file names encode the bill key, and a page lists the directory and reads only its entries (about 10 ms for the first page of 15,000). Local only, excluded from `gcp` builds |
| `internal/adapters/localauth` | Operator authentication for `admin-api` | A fixed fake operator. Local only, excluded from `gcp` builds |
| `internal/adapters/cloudtasks` | `Queue` | Cloud Tasks |
| `internal/adapters/claude` | `Generator` | Claude through the Anthropic Go SDK: the Google Cloud client (`googlecloud`) in `gcp` builds, the first-party client elsewhere |
| `internal/adapters/fakegen` | `Generator` | Recorded fixtures; deterministic, no network, for tests and local runs |
| `internal/adapters/govinfo` | `BillSource` | GPO govinfo bulk data over HTTPS |
| `internal/adapters/usccollector` | `BillSource` | Output of the `unitedstates/congress` collector, read from a bucket or directory |
| `internal/adapters/uscode` | `LawSource` | US Code release points from the Office of the Law Revision Counsel |
| `internal/adapters/webpush` | `Notifier` | Web Push protocol (RFC 8030) with message encryption (RFC 8291) and VAPID (RFC 8292) |
| `internal/adapters/fakenotify` | `Notifier` | Records messages in memory for tests and local runs |
| `internal/adapters/fixturesrc` | `BillSource`, `LawSource` | Example data committed under `fixtures/`: real bills, a catalog sample, and cited US Code provisions ([tasks § Example data](../design/tasks/README.md#example-data-and-local-triggers)); no network |

Adding a backend means adding a package that implements one or more ports and passes their conformance tests. Candidates later: DynamoDB, S3-compatible storage, AWS KMS, Postgres-backed documents, SQLite.

### Conformance tests
`internal/data/conformance` holds a test suite per port as exported functions (`conformance.SignalLedger(t, factory)`): uniqueness of signals, idempotent upserts, expiry of sessions, revision history, typed errors, cursor stability. Every adapter's tests call the suites for the ports it implements: in process for `memory`, on a temporary directory for `fsobject` and `fsdocs`, and against the service's emulator or a disposable container where one exists. Because the filesystem and cloud adapters pass the same suites, local runs are a faithful stand-in for the contract; what a suite cannot check (query and index limits, transaction size limits, IAM) is listed by the filesystem adapters and tested on the cloud adapters. An adapter that fails a suite cannot be wired into a service.

### Repository layout

```
api/
  openapi.yaml         the REST contract (source of truth)
go.mod                 one Go module for the backend
cmd/
  read-api/            one directory per binary; main.go + wiring files
  write-api/
  worker/
  admin-api/
  pubctl/             operator command-line tool for admin-api (a binary, not a container)
  job-ingest/
  job-batcher/
  job-briefs/
  job-exports/
  job-detector/
  job-summarize/
  job-notify/
  accuracy-report/     development tool for the accuracy checks (not a deployed image)
internal/
  data/                ports, domain types, errors, conformance tests
  adapters/            memory, fsobject, fsdocs, localauth, firestore, gcs, postgres, kmsgcp, localkeys, cloudtasks, claude, fakegen, webpush, fakenotify, govinfo, usccollector, uscode, fixturesrc
  telemetry/           OpenTelemetry setup, standard instruments, attribute allowlist
  httpx/               server bootstrap: health, readiness, metrics port, shutdown
  ...                  domain packages (auth, review, signals, ingest, briefs), each used by few binaries
app/                   SvelteKit UI, deployed to any Node web host
demo/                  archived M0 demo (static, mock data), served from its Netlify site
docker/
  Dockerfile           one multi-stage file; SERVICE and TARGET build arguments
compose.yaml           every backend service on a shared local data volume; emulators behind a profile
infra/
  modules/gcp/         public, reusable infrastructure modules; every name, project, region, and domain is an input
  examples/            placeholder root that CI validates against fake values
deploy/                ignored here: local checkout of the private deployment repository
```

## Service packaging

Every backend unit is a **standalone container image** that runs on any container runtime: Cloud Run, a laptop with Docker, a Kubernetes cluster, or a single VM. Cloud Run is where they run in production, not a requirement for running them.

### Units

| Image | Kind | Runs as on Google Cloud | Runs as elsewhere |
| --- | --- | --- | --- |
| `read-api` | HTTP | Cloud Run service behind Firebase Hosting | Any container with a published port |
| `write-api` | HTTP | Cloud Run service behind Firebase Hosting | Same |
| `worker` | HTTP | Cloud Run service, pushed to by Cloud Tasks, not publicly routed | Same; the local `Queue` adapter delivers tasks by HTTP |
| `admin-api` | HTTP | Cloud Run service requiring IAM authentication; not behind the API edge | Same, behind whatever authentication the host provides |
| `job-ingest` | Job | Cloud Run job, triggered by Cloud Scheduler | `docker run`, cron, systemd timer, Kubernetes CronJob |
| `job-batcher` | Job | Same | Same |
| `job-briefs` | Job | Same | Same |
| `job-exports` | Job | Same | Same |
| `job-detector` | Job | Same | Same |
| `job-summarize` | Job | Same | Same |
| `job-notify` | Job | Same | Same |

One image per unit, so each deploys, scales, and holds credentials independently.

### Language and images

**Go** for every backend service and job. A Go service is one static binary with no runtime to install. It starts in tens of milliseconds, so scale-from-zero latency is dominated by the platform pulling and starting the image, which a small image keeps short.

| Choice | Decision | Why |
| --- | --- | --- |
| HTTP | Standard library `net/http` with method and path patterns | No framework; route patterns double as the `http.route` metric attribute |
| Logging | Standard library `log/slog`, JSON handler | Structured logs with no dependency |
| Postgres | `pgx` | Pure Go, no cgo |
| Sign-in | `golang.org/x/oauth2`, `go-oidc` | Standard flows, PKCE, nonce, ID-token validation; Facebook via its OAuth endpoint and a minimal Graph call |
| Google Cloud | Official `cloud.google.com/go` clients (Firestore, Storage, KMS, Cloud Tasks) | Mature, maintained by Google; imported only inside adapters |
| Telemetry | OpenTelemetry Go SDK with the Prometheus exporter | Official and maintained |
| Build | `CGO_ENABLED=0`, `-trimpath`, `-ldflags="-s -w"`, `GOARCH=amd64` and `arm64` | Fully static, stripped, reproducible binaries |
| Runtime image | `gcr.io/distroless/static-debian12:nonroot`, pinned by digest | About 2 MB: CA certificates, time zone data, a non-root user, no shell or package manager |

Build: multi-stage Dockerfile. A Go builder stage with module and build caches compiles one binary with the selected target tag and copies it alone into the distroless stage.

### Tooling

| Need | Decision | Why |
| --- | --- | --- |
| Go version | Go 1.27, in `go.mod` (`go` and `toolchain` lines) and the builder image, pinned by digest | One version everywhere; bumped deliberately |
| Go tools | `golangci-lint` and `oapi-codegen` pinned with the `tool` directive in `go.mod`, run with `go tool` | Versions live in the module; nothing to install by hand |
| Lint | `golangci-lint` v2, strict: all linters enabled, each disabled linter justified in `.golangci.yml`, no issue limits, `//nolint` only with a named linter and reason | Findings fail CI |
| Development container | `docker/dev.Dockerfile`; builds, tests, and lint run inside it, with explicit CPU and memory limits, for agents and CI alike | Same toolchain everywhere; parallel work cannot starve the host |
| Node | Node 24 (`node:24-bookworm-slim`, pinned by digest in `docker/dev.Dockerfile`) for `app/` | One runtime for UI builds, type generation, and Playwright |
| TypeScript types | `openapi-typescript` as a dev dependency in `app/`, with the npm override `overrides.openapi-typescript.typescript=$typescript` because its peer range stops at TypeScript 5 | Pinned by the app's lockfile |
| OpenAPI linter | Redocly CLI (`@redocly/cli`, dev dependency in `app/`), config in `api/redocly.yaml` | Validates OpenAPI 3.1 with no extra runtime |
| API code generation | `oapi-codegen` (tool directive) generates `api/v1` (package `apiv1`); `sh api/generate.sh` regenerates Go and TypeScript, `sh api/check.sh` lints the spec and fails if generated code is stale | One command each to regenerate and to check |
| Multi-arch images | Cross-compile in the builder stage (`--platform=$BUILDPLATFORM`, `GOOS`/`GOARCH` from `TARGETOS`/`TARGETARCH`) | Static Go binaries need no emulation; the final stage only copies the binary, so no QEMU or binfmt |
| SBOM | BuildKit SBOM attestation (`--sbom=true`) | Generated at build with no separate tool |
| CI | GitHub Actions, in this repository and the deployment repository | The repository is on GitHub; secret scanning with push protection and OIDC federation to Google Cloud are built in |
| Infrastructure as code | OpenTofu | Open-source licence; the HCL also works with Terraform |
| UI, no-script, and installability tests | Playwright (image `mcr.microsoft.com/playwright:v1.65.1-noble`, which bundles Node 26, so tests drive an app running on Node 24 in another container; 2 workers within 1.5 GiB), with manifest and service-worker checks and an offline reload | One browser tool for all UI checks; Lighthouse dropped its PWA audits in version 12 |
| Firestore emulator | Container image under the `emulator` profile in `compose.yaml` (cloud phase) | CI and every machine run the same emulator |
| Firebase CLI | Cloud phase only, in the deployment repository's CI | Local runs never use Firebase Hosting |
| Postgres | Not in `compose.yaml` until M2 | No SQL in M1 |
| Bill DTD validation | `xmllint --dtdvalid` (`libxml2-utils`) in a CI container, against the official `bill.dtd` and `res.dtd` committed under `internal/billxml/testdata/dtd/` | Uses the official DTDs unchanged; Go has no DTD validator |
| Secret scanning | GitHub secret scanning with push protection, plus the format-based pattern check in CI | See [What stays private](#what-stays-private) |

Open: whether a local pre-commit secret scanner is wanted beside CI scanning.

### Keeping each service small
- **One responsibility per binary.** A service that grows a second job is split.
- **Few dependencies.** Standard library first. A new third-party module needs a reason in review. Vendor SDKs are imported only from adapter packages.
- **Size budget per binary, checked in CI.** Google Cloud clients built on gRPC dominate binary size; services that do not need an adapter must not link it. The budget is set from the first M1 builds and fails the build when exceeded. First measurement (M1-01): `read-api` on the `memory` target, `linux/amd64`, `-trimpath -ldflags="-s -w"`, is 16.8 MB, most of it the OTLP exporter's gRPC and protobuf. The budget is 20 MB, in `test/ci/size-budget.txt`, checked by `test/ci/size.sh`.
- **No work at startup** beyond reading config and constructing clients. Clients connect on first use.

### Contract for every image
- **Configuration from environment variables only.** Each service validates its variables at startup and exits non-zero with a clear message if one is missing. No config files baked into the image.
- **Credentials from the environment.** On Google Cloud, the service account attached to the Cloud Run service or job. Elsewhere, standard Google application credentials or the local adapters. Never inside the image.
- **Stateless.** No data kept on local disk beyond `/tmp` for the life of one request or run. The root filesystem can be mounted read-only.
- **Logs to stdout as JSON lines.** No client IPs, cookies, or request bodies ([Platform logs](#platform-logs)).
- **Non-root user, minimal base image, pinned by digest.** Built for `linux/amd64` and `linux/arm64`.
- **Labelled** with source repository, commit, and `AGPL-3.0-or-later`, with a software bill of materials generated at build.

### HTTP services (`read-api`, `write-api`, `worker`)
- Listen on `0.0.0.0:$PORT`, defaulting to `8080`.
- `GET /healthz` reports the process is alive. `GET /readyz` reports its adapters are configured and reachable.
- `GET /metrics` on a separate port (`$METRICS_PORT`, default `9464`) serves metrics for scraping ([Observability](#observability)). The metrics port is never routed publicly.
- On `SIGTERM`, stop accepting requests, finish in-flight ones, and exit within 10 seconds.
- `read-api` reads `RATE_LIMITS` (form `text=600,live=300,bulk=120,discovery=600`, per minute per route class), `CLIENT_IP_HEADER`, and, on the `memory` target only, `SEED_FIXTURES=1` to load invented test fixtures.
- Shared bootstrap lives in `internal/httpx`, so every service gets the same health, readiness, metrics, and shutdown behavior.

### Jobs
- **Run to completion and exit.** Exit `0` on success, non-zero on failure, so any scheduler can retry.
- **Idempotent.** Re-running a job for the same window produces the same result. The batcher keys its output by batch window; ingest skips versions already stored; exports rebuild from source.
- **Safe to overlap.** Two runs at once must not corrupt state: writes are keyed by window or version, and immutable paths are written once.
- **Window from flags or environment,** defaulting to "now". `--dry-run` reports what would change without writing.
- **No schedule inside the image.** The schedule belongs to the platform (Cloud Scheduler, cron, CronJob), so the same image runs on demand or on any cadence.

### Build targets
The `TARGET` build argument selects the build tag, and so the [wiring](#build-time-wiring) compiled into the image:

| Target | Adapters | Use |
| --- | --- | --- |
| `gcp` | `firestore`, `gcs`, `postgres`, `kmsgcp`, `cloudtasks`, `claude`, `webpush` | Production and staging. Also runs off Google Cloud when given credentials to the same project |
| `local` | `fsobject`, `fsdocs`, `localkeys`, `localauth`; sources `fixturesrc` or `govinfo`/`uscode`; `fakegen` or `claude` with an API key; a local Postgres container from M2 | Durable local development and integration tests with `compose.yaml`. Needs no cloud account or emulator |
| `emulator` | `firestore` and `gcs` against the Firestore emulator and a Cloud Storage stand-in; `localkeys` | Conformance and integration tests of the cloud adapters, locally and in CI |
| `memory` | `memory`, `localkeys` | Unit and conformance tests; nothing persists |

A target only includes the adapters a given service's wiring imports, so a `read-api` image of any target contains no SQL or key adapter. CI also fails if a `gcp` binary links any local-only adapter (`fsobject`, `fsdocs`, `localkeys`, `localauth`, and their helper `fsutil`), so a local stand-in can never reach a deployed service.

**Local first.** Every service and job builds and runs on `local` before any cloud adapter exists; cloud connections are added afterwards behind the same ports ([M1 phases](../design/tasks/README.md#phases)).

A fully self-hosted durable deployment with no Google services needs production-grade adapters for the document and object stores outside Google Cloud; the filesystem adapters are for local use only. These fit the adapter model and are listed as candidates.

### Local run
`compose.yaml` starts every backend HTTP service and an OpenTelemetry Collector on the `local` target, with one shared data volume (for example `./.local-data`, ignored by git) that every service and job mounts. From M2 it also starts Postgres. The Firestore emulator and Cloud Storage stand-in run only under the `emulator` profile (`docker compose --profile emulator`). The UI runs with `npm run dev` in `app/`, pointed at the local `read-api` and `write-api`. Jobs run on demand:

```sh
docker compose up read-api write-api worker
docker compose run --rm job-batcher --window 2026-11-21T14:00Z --dry-run
```

## Observability

All backend services use **OpenTelemetry** for metrics through the shared `internal/telemetry` package. HTTP services expose metrics for **scraping** in Prometheus format; jobs, which exit too soon to be scraped, **push** over OTLP before they exit.

### Model
- **SDK:** OpenTelemetry Go SDK, initialised by `internal/telemetry` before anything else in each service.
- **Exporter (HTTP services):** OpenTelemetry Prometheus exporter serving `GET /metrics` on `$METRICS_PORT`.
- **Exporter (jobs):** OTLP push to `$OTEL_EXPORTER_OTLP_ENDPOINT`, flushed before exit.
- **Resource attributes** on every metric: `service.name` (`read-api`, `write-api`, `worker`, `job-batcher`, …), `service.version` (commit), `deployment.environment.name` (`dev`, `staging`, `prod`), plus detected cloud attributes.
- **Naming:** OpenTelemetry semantic conventions where one exists; project metrics under the `pubcommittee.` prefix.

### Standard instruments

Every HTTP service gets these from `internal/telemetry` and `internal/httpx` without writing code:

| Metric | Type | Attributes |
| --- | --- | --- |
| `http.server.request.duration` | Histogram (s) | `http.request.method`, `http.route` (the matched pattern, e.g. `GET /v1/bills/{bill}/v/{version}`), `http.response.status_code` |
| `http.server.active_requests` | Up-down counter | `http.request.method` |
| `pubcommittee.data.operation.duration` | Histogram (s) | `pubcommittee.port`, `pubcommittee.operation`, `pubcommittee.adapter`, `pubcommittee.outcome` (`ok` or typed error name) |
| `pubcommittee.startup.duration` | Histogram (s) | — (cold-start cost) |
| Go runtime and process metrics | Various | Semantic conventions |

The data-operation histogram is recorded by a wrapper around every port, so adapters do not instrument themselves and every adapter reports the same way.

Service-specific counters:

| Metric | Service | Attributes |
| --- | --- | --- |
| `pubcommittee.writes` | `write-api` | `pubcommittee.write.kind` (`comment`, `suggestion`, `signal`, `markup_request`, `flag`, `response`, `markup_decision`), `pubcommittee.outcome` |
| `pubcommittee.rate_limited` | `write-api` | `pubcommittee.write.kind` |
| `pubcommittee.signin` | `write-api` | `pubcommittee.provider` (`google`, `facebook`), `pubcommittee.outcome` |
| `pubcommittee.tasks` | `worker` | `pubcommittee.task.kind`, `pubcommittee.outcome` |
| `pubcommittee.notifications` | `job-notify` | `pubcommittee.outcome` (`sent`, `gone`, `error`, `nothing_new`). No account or item attributes |
| `pubcommittee.job.runs`, `pubcommittee.job.duration`, `pubcommittee.job.items` | jobs | `pubcommittee.job.name`, `pubcommittee.outcome` |
| `gen_ai.client.token.usage` | `job-summarize`, `worker` | `gen_ai.operation.name`, `gen_ai.request.model`, `gen_ai.token.type` (`input`, `output`, `cache_read`, `cache_creation`). OpenTelemetry GenAI semantic conventions; drives cost tracking |
| `pubcommittee.generation.results` | `job-summarize`, `worker` | `pubcommittee.generation.task`, `pubcommittee.outcome` (`ok`, `refusal`, `invalid`, `truncated`, `error`) |
| `pubcommittee.job.last_run_timestamp` | jobs | `pubcommittee.job.name`, `pubcommittee.outcome`. Gauge in Unix seconds, so a single push per run is enough to detect a failed or missing run |

These metrics are shaped for alerting without further changes: error ratios come from outcome and status-code attributes, latency from histograms, and job health from the timestamp gauge. Alert rules themselves are out of scope for now.

The UI is outside this model; its request metrics come from its host.

### Privacy rules for telemetry
Metrics are a public-output-adjacent data set: anyone with dashboard access could otherwise watch live counts and defeat the batching in [11](../design/11-ai-security.md).
- **Attribute allowlist.** `internal/telemetry` registers metric views that keep only the attribute keys listed above and drop all others. A new attribute needs a change to the allowlist, reviewed like a schema change.
- **Never as attributes:** account ids, pseudonyms, session ids, IPs, user agents, bill, item, or thread ids, raw URL paths, query strings.
- **No signal values.** `pubcommittee.writes` counts signals without support or oppose, and without the item. Live per-item counts exist only inside the private store and in published snapshots.
- **Routes as patterns.** `http.route` uses the matched route pattern, never the concrete path, which keeps cardinality bounded and keeps bill and line addresses out of metrics.
- **Exemplars off.** Exemplars would attach request-level trace ids to metric points.

### Collection

| Where | HTTP services | Jobs |
| --- | --- | --- |
| Google Cloud | Managed Prometheus sidecar container in each Cloud Run service scrapes `localhost:$METRICS_PORT` and writes to Cloud Monitoring | OTLP push to the Cloud Monitoring OTLP endpoint, or to a collector |
| Anywhere else | Prometheus or an OpenTelemetry Collector (Prometheus receiver) scrapes `/metrics` | OTLP push to an OpenTelemetry Collector |
| Local | `compose.yaml` runs an OpenTelemetry Collector that scrapes every service and prints or forwards metrics | Same collector over OTLP |

Scraping inside the instance is what makes this work with scale to zero: an external scraper cannot reach an instance that does not exist, and cannot choose which of several instances it hits. The sidecar runs and stops with each instance. Metric volume, and so cost, drops to zero when the service does.

## Identity and personal data

### Sign-in
- **Providers:** Google and Facebook at launch ([08](../design/08-onboarding-and-verification.md)). No local passwords.
- **Flow:** OAuth authorization code with PKCE, `state`, and `nonce`, handled by `write-api` on `api.<domain>`. The browser is redirected there and back; the web host is not involved. No third-party identity service holds user profiles.
- **Scopes:** Google `openid` only, so no email or profile is returned. Facebook default `public_profile`, reading only `id`; the returned name is discarded.
- **Stored:** `HMAC(k_identity, provider ‖ subject)` mapped to an internal account id. The raw provider subject is not stored.
- **US check:** country from the edge's location header or an in-memory lookup at sign-in. Only `us_at_signup = true` and the day are kept; the IP is never stored.
- **Display name:** chosen by the person, pseudonymous by default.

### Sessions
- Opaque random id in an `HttpOnly`, `Secure`, `SameSite=Lax` cookie, host-only on `api.<domain>`; server-side record with expiry. `Lax` is needed for the provider redirect to land signed in.
- Every write checks the `Origin` header against `https://<domain>` and a per-session anti-forgery token. No bearer tokens, no API keys, no delegated write access ([10](../design/10-machine-access.md)).
- Presence checks for signals (later milestone) use device-bound passkeys tied to the session account.

### Personal data inventory

| Data | Stored | Where | Retention |
| --- | --- | --- | --- |
| Name, email from provider | No | — | — |
| Provider subject | HMAC only | SQL `identities` | Account lifetime |
| Display name | Yes | SQL, and per-bill pseudonym in public docs | Account lifetime |
| IP address | No (in memory at sign-in only) | — | Request lifetime |
| Signals and markup requests | Keyed by HMAC, no account id | SQL `signals` | Per [09](../design/09-integrity-moderation-and-privacy.md) retention decision |
| Markup request reasons | Text only, keyed by HMAC, never published | SQL `request_reasons` | Until the request is withdrawn or decided, then per retention decision |
| Email address | Not collected (email notifications deferred) | — | — |
| Device notification subscriptions | Encrypted endpoint and keys; no item ids | SQL `push_subscriptions` | Until revoked, signed out, or reported gone |
| Phone (tier 1 uniqueness, later) | HMAC only | SQL | Account lifetime |
| Address (tier 2, later) | District and state only, plus verification token | SQL | Until residency changes |
| ID documents (tier 3, later) | Never; one-way token from the verifier | SQL | Per verifier contract |

### Platform logs
Managed platforms log request IPs and headers by default, which would undo the inventory above.
- Strip or truncate client IPs in backend request logs; never log cookies or request bodies.
- Keep request logs for days, not months.
- Rate limiting uses `HMAC(k_rate, ip ‖ day)` with time-to-live deletion, never raw IPs.
- The web host keeps its own request logs for the UI. They carry no identity (no session cookie reaches it), but whichever host is used has its retention and analytics settings reviewed against this inventory.

## Bill source

### How ourdistrict did it
The earlier prototype ([prior work](../design/MILESTONES.md#prior-work-ourdistrict)) did not fetch from Congress itself. It relied on a separately run, forked copy of the [`unitedstates/congress`](https://github.com/unitedstates/congress) collector writing to a local disk, and read that disk:

| Step | ourdistrict | Problem for PubCommittee |
| --- | --- | --- |
| Collection | A Python collector, run by hand or cron outside the app, wrote `data/{congress}/bills/{type}/{number}/data.json` and `text-versions/{code}/data.json` | A second toolchain and a shared filesystem; neither exists in a scale-to-zero container |
| Metadata | A background actor re-read every `data.json` every 7 days and merged it into SQL (job disabled in `Boot.scala`; update path had a lastmod bug) | Weekly is too slow around markups; full re-reads do not scale |
| Bill text | On the first page view with `?initialize`, fetched the XML from the GPO URL in the newest text version's `data.json`, parsed it, and stored it | Parsing on a reader's request; only the newest version kept, so no version history or stale detection ([02](../design/02-versions-and-review-lifecycle.md)) |
| Scope | One Congress, set in config (`settings.congress=115`) | Hard-coded jurisdiction and session |

What carries over: the collector's directory layout and JSON schema are a well-known public format, and its text-version metadata points at the same official GPO files.

### Sources

| Source | Provides | Access | Use |
| --- | --- | --- | --- |
| govinfo bulk data, `BILLS` collection | Official XML of every bill text version | HTTPS, no key; `/json` or `/xml` listing per Congress, session, and bill type (requires an `Accept` header) | **Primary** for text |
| govinfo bulk data, `BILLSTATUS` collection | Official bill status XML: titles, sponsors, actions, committees, related bills, list of text versions | HTTPS, no key; current Congress updated about every 4 hours; sitemap index with last-modified dates; RSS feed listing files changed in each batch | **Primary** for metadata and change detection |
| `unitedstates/congress` collector (public domain, maintained) | The same official data, normalised to JSON | Run the Python tool; output to files | **Optional** backfill or fallback, read by its own adapter |
| Office of the Law Revision Counsel | US Code as XML release points | HTTPS download from `https://uscode.house.gov/download.html`; files at `download/releasepoints/us/pl/<congress>/<law>/xml_usc<TT>@<release>.zip`, where release `119-142` means current through Public Law 119-142. `priorreleasepoints.htm` answered 404 on 2026-11-22, so the adapter lists only the current release point | **Primary** for existing law ([01](../design/01-bill-ingestion-and-display.md)) |
| `usgpo/bill-dtd` | DTDs for bill XML | Git repository | Parser tests and validation |

Exact paths, listing formats, and update cadences are confirmed against GPO's published guides at M1 start. Confirmed by M1-05:
- Status: `https://www.govinfo.gov/bulkdata/BILLSTATUS/{congress}/{type}/BILLSTATUS-{congress}{type}{number}.xml`.
- Text: the XML URL each status lists, of the form `https://www.govinfo.gov/content/pkg/BILLS-{congress}{type}{number}{code}/xml/BILLS-{congress}{type}{number}{code}.xml`. The version code is the part of the file name after the number, in lower case. A status lists XML only for versions already published; items without an XML URL are skipped.
- Listings: `https://www.govinfo.gov/bulkdata/json/BILLSTATUS/{congress}/{type}` with `Accept: application/json`, about 4 MB for `hr`, including file sizes.
- Servers send `Last-Modified` and no `ETag`, so conditional requests use `If-Modified-Since`. The `govinfo` adapter fetches only `www.govinfo.gov` URLs, one request at a time.

### Recommendation
**Pull directly from govinfo in Go, behind a `BillSource` port.** No Python collector, no shared disk, no API key.

- **Change detection.** `job-ingest` reads the `BILLSTATUS` RSS feed or sitemap last-modified dates since its stored watermark, so each run touches only bills that changed. A full listing walk is the fallback when the watermark is missing or a feed gap is detected.
- **Versions.** Each bill status lists its text versions. Every version not yet stored is fetched from `BILLS`, kept as the **original XML, byte for byte**, with its source URL, retrieval time, and SHA-256, at a write-once path (`bills/{bill}/v/{n}/source.xml`). Nothing is ever overwritten ([01](../design/01-bill-ingestion-and-display.md)).
- **Parsing at ingest, never on a request.** The parser turns the original XML into sections, lines, and anchors (`text.json`), recording each line's depth from the XML's nesting (subsection, paragraph, subparagraph, clause, list item) so blocks and edit ranges can be computed ([01](../design/01-bill-ingestion-and-display.md)), using the element rules ourdistrict worked out (`legis-body`, `resolution-body`, `preamble`, `quoted-block`; tables, formulas, and graphics as single units). Parser output carries a parser version so re-parsing a version is a deliberate, versioned act.
- **Identity.** Bill ids include jurisdiction and session (`us-119-hr-4127`), so nothing assumes one Congress ([principle 7](../design/README.md#global-principles)).
- **Hand-offs.** A newly stored version triggers stale classification and re-anchoring ([02](../design/02-versions-and-review-lifecycle.md)), lineage candidates from related bills ([04](../design/04-bill-lineage-and-competing-bills.md)), and `job-summarize`.
- **Existing law.** A separate run checks for a new US Code release point, stores it with its release date, and flags provisions that public laws enacted after that release may have changed, for steward review.
- **Politeness.** One request at a time per host with backoff, a descriptive user agent naming the project (no personal contact details), and conditional requests where the source supports them.

The `usccollector` adapter keeps the collector as an option, for example to backfill older Congresses quickly from its output, without making it a dependency. The `fixturesrc` adapter lets every environment run ingestion with no network.

### Parser
`internal/billxml` parses official bill XML into sections and lines. It is a pure function (`Parse(src, logger)`), called by ingest ([M1-05](../design/tasks/m1/05-manual-bill-pull-and-storage.md)), never on a request. Its version, `ParserVersion` (now `billxml/1`), is stored with the parsed text; a change to parsing rules needs a new version and regenerated golden files. Line unit, non-prose units, continuation lines, and anchors are as [01 § Open questions](../design/01-bill-ingestion-and-display.md#open-questions) records them (M1-04 answers). Parsed `text.json` lines carry `n`, `anchor`, `depth`, `text`, `source_id`, `kind` (`text`, `group`, `table`, `toc`, `formula`, `graphic`), `quoted`, `continuation`, and `raw` for non-prose markup.

### Bill selection

Implements [01 Bill selection](../design/01-bill-ingestion-and-display.md#bill-selection): a cheap **catalog** of every bill, and an expensive pipeline only for the **tracked set**.

- **Catalog for everything.** Every `BILLSTATUS` change updates the bill's catalog entry in `bills`. No text is fetched.
- **Policy evaluated on every status change.** `job-ingest` loads the current `selection_policy` and evaluates it against the catalog entry. A newly tracked bill gets `tracked: {since, policy, reasons}` and enters the full pipeline: text versions fetched and parsed, `job-summarize` triggered, review opened.
- **Promotion needs no backfill.** All versions of a newly tracked bill are listed in its status, so they are fetched at promotion time.
- **Never untracked by rule.** Policy changes apply going forward only.
- **Cost follows the tracked set.** Text storage, parsing, and AI summarization scale with tracked bills, not with all of Congress.

Policy shape (stored as one versioned document, published through `read-api`):

```yaml
version: 3
mode: pilot                     # pilot: committee action AND topic | general: committee action only
phase: early_pilot              # early_pilot: administrators may select bills alone | steward_signoff: two-party sign-off required
measure_kinds: [bill, joint_resolution]          # simple/concurrent and privileged resolutions excluded
exclude_kinds: [war_powers_resolution, commemorative]
committee_activity: [hearing, markup, reported, discharged, floor, calendar]   # referral alone does not count
follow_companions: true
topics:
  healthcare:        { policy_areas: [...], subjects: [...], title_terms: [...] }
  deficit_reduction: { policy_areas: [...], subjects: [...], title_terms: [...] }
  affordability:     { policy_areas: [...], subjects: [...], title_terms: [...] }
# Requested and nominated bills are not listed here: an accepted decision in markup_decisions
# tracks the bill directly, with the decision id among its reasons.
```

Signals used, all from `BILLSTATUS`: bill type, committee activity names, actions, policy area, legislative subjects, titles, and related bills. Subject indexing can arrive weeks after introduction, so title terms let a bill match early and subjects confirm later. The subject terms for each topic are taken from the legislature's published subject vocabulary and confirmed before the pilot.

The same evaluation sets every catalog entry's `markup_status`: `in_review` (tracked), `eligible` (allowed measure kind, not yet selected), or `not_candidate` (excluded kind, with the reason shown).

As built (M1-06, `internal/selection`):
- The stored policy document carries the structured fields above plus `rules`, a plain-language statement generated from them. Version 1 is embedded at `internal/selection/policies/v1.json`; its topic terms are drafts until the operator confirms them ([policies/README.md](../internal/selection/policies/README.md)). `selection.Publish` stores a version and its public log entry atomically, requires a reason, refuses a version below the latest, and treats an identical republish as a no-op.
- Reason vocabulary: `committee-action:<hearing|markup|reported|discharged|floor|calendar>`, `topic:<name>:<subject|policy-area|title-only>`, `companion:<bill id>`, `administrator`, and `manual-pull` (local `--track`). A companion is a related bill marked identical or companion that is already tracked; it needs no committee action or topic but must be an allowed kind. A `title-only` reason is upgraded when indexing later confirms the topic.
- Catalog entries gain `policy_version`, `tracked_since`, and `sponsor_party`. `sponsor_party` is stored only for the disclosure counts and is not served by `read-api`; the evaluator's input type has no sponsor fields, and a test fails if one is added.
- The tracked-set breakdown is written to `exports/tracked-set-breakdown.json` by every pull and every `job-ingest --evaluate` run. No `read-api` route serves it yet.
- At scale (cloud phase) the policy step needs a `PublicDocs.GetCatalogEntry` read; locally it pages through `ListCatalog` once per run, which does not scale to a whole Congress.

### Public catalog
- **Every catalog entry is public** through `read-api`, including bills that are not tracked.
- **Official links.** The `govinfo` adapter records, from the bill status, the legislature's page for the bill and the official URL of each text version, stored as `official_links`. The UI links untracked bills there; their text is never fetched, stored, or re-rendered ([01](../design/01-bill-ingestion-and-display.md#public-catalog)).
- **Search.** The document store has no full-text search, so each entry carries `search_terms` (normalised bill number, title words, subject terms) for keyword matching, alongside filters on status, chamber, committee activity, and subject. The catalog is also part of the bulk exports. A dedicated search index is an open decision.

### Markup requests
Implements [01 Markup requests](../design/01-bill-ingestion-and-display.md#markup-requests). A request is private in the same way a signal is, so it reuses the signal machinery.

- **Storage.** `write-api` stores a request as a signal with `item_kind = markup_request` and `signal_key = HMAC(k_signal, account_id ‖ bill_id)`: one per person per bill by primary key, withdrawable, no account id stored. The optional reason goes to `request_reasons` under the same key.
- **Totals.** `job-batcher` aggregates requests with every other signal and publishes them in the bill's snapshot under the same small-count and batching rules. When a bill's total first reaches the request threshold, it adds the bill to `markup_queue`.
- **Nominations.** Pilot offices and stewards add a bill to `markup_queue` directly through `write-api`.
- **Steward queue.** Steward routes on `write-api` list queued bills with their published total and the reasons, without names or keys.
- **Decisions.** A steward records accept or decline with a reason. Accept stays `pending` until a steward from another party signs; then `write-api` writes the public `markup_decisions` entry, sets `tracked` on the bill with `requested:{decision id}` as a reason, and enqueues a `worker` task that fetches and parses all its text versions and triggers `job-summarize`. Decline is published immediately with the rule applied.
- **Surges.** `job-detector` checks request totals like support totals; a detected surge is attached to the decision as a disclosure.
- **Notifications.** Decisions reach requesters through the general [notification](#notifications) path: the decision is a public event, and each requester's own `my_signals` list includes the request, so no list of requesters exists.

`job-ingest` also publishes the tracked set's counts by topic and by sponsor party with each run, for the neutrality disclosure in [09](../design/09-integrity-moderation-and-privacy.md#coverage-neutrality). Policy changes are written to the public moderation log.

### Data recorded per ingest

| Record | Where | Contents |
| --- | --- | --- |
| Original document | Object store, write-once | Source bytes; URL, retrieved-at, and SHA-256 in a sidecar (`source.xml.meta.json`), read through `SourceInfoReader` |
| Parsed version | Object store, write-once | Sections, lines, anchors, parser version |
| Bill metadata | `bills` | Status, titles, sponsors, actions, version list with source references |
| Watermark | `ingest_state` | Last processed position per source and Congress; failures to retry. In M1 the key is `<source>/<session>` (for example `govinfo/119`) and it holds failed items only, with no watermark; its read-modify-write is not atomic, so overlapping pulls can drop a failure that the next pull records again |

## AI-generated content

Some work is handed to AI models: mainly summarizing bills and explaining what they change in existing law. These are **single model calls run in batches by a job**, not agents: each task has fixed inputs and a fixed output shape, and code controls the flow.

### Tasks

| Task | Input | Output | Shown as | Feature doc |
| --- | --- | --- | --- | --- |
| Bill summary | Official text of one version | Short plain-language summary of the whole bill | Auto-generated until a steward checks it | [01](../design/01-bill-ingestion-and-display.md) |
| Change explanation | One bill section plus the existing-law provisions it cites | What the provision says now, what the bill changes, why it matters | Same, with the existing provenance flags | [01](../design/01-bill-ingestion-and-display.md) |
| Version change summary | Two official versions of the same bill | What changed between them, by section | Same | [02](../design/02-versions-and-review-lifecycle.md) |
| Competing-bill comparison | Official text of the bills on one issue | A neutral side-by-side draft | Never shown until two stewards from different parties sign off | [04](../design/04-bill-lineage-and-competing-bills.md) |

Brief contents are not generated: the brief quotes authors' own words ([07](../design/07-legislator-brief-and-responses.md)).

### Rules
1. **Official and platform content in, never user content.** Prompts are built only from official bill text, existing-law provisions, and platform metadata. Comments, suggestions, reasons, and display names never reach a model, so text planted by a poster cannot steer a summary ([10](../design/10-machine-access.md#instruction-injection)). Bill text itself is passed as clearly delimited source material, not as instructions.
2. **No personal data leaves the system.** Every input is public legislative text.
3. **Output is platform content.** Stored with `content_class: platform`, provenance `auto`, the model id, and the prompt version. The UI and `read-api` label it "auto-generated" until a steward checks it ([05](../design/05-roles-and-stewardship.md)).
4. **Anchored claims.** Each statement in an output lists the line anchors (`s4-l12`) and provision citations it rests on. The job rejects outputs that cite anchors that do not exist in the input, and stewards see the cited lines beside each statement.
5. **Neutral language.** Prompts require neutral, descriptive wording; the evaluation set scores it. Generation never ranks, recommends, or takes a position ([principles 2 and 3](../design/README.md#global-principles)).
6. **Steward review before trust.** Every output enters the steward review queue. Whether an unreviewed output may be shown at all, and under which accuracy bar, is decided per task from evaluation results (the open question in [01](../design/01-bill-ingestion-and-display.md)). **Until stewards exist** there is no review queue, no steward review, and no evaluation set: outputs are recorded with `review_state: unreviewed` and shown labelled *auto-generated*, and nothing is labelled *steward-checked*.
7. **Corrections are attributed.** A steward's edit replaces the displayed text, is logged and attributed, and is kept beside the original output.

### Flow

```
job-ingest ──► new version stored ──► job-summarize (submit)
                                           │ builds tasks from official text only
                                           ▼
                                   batch submitted to Claude ──► generations: submitted
                                           │
                     job-summarize (collect, next scheduled run)
                                           │ validate schema, anchors, stop reason
                                           ▼
                         stored output + generations: ready ──► steward review queue
```

- **Two-phase job, no waiting.** One run submits a batch and exits; a later scheduled run collects results. Nothing stays running while the batch processes, which keeps the job scale-to-zero.
- **Idempotent by key.** Each task is keyed by `(task, bill, version, target, prompt_version, model)`. Re-running submits only keys with no stored result; a new prompt version or model creates new keys and leaves old outputs in place for comparison.
- **Outputs are immutable files** in the object store (`generated/{task}/{bill}/v/{n}/{key}.json`); `generations` records status and which output is current.
- **Steward-requested regeneration** goes through the `worker` as one synchronous call with the same rules.

### Model and platform

| Choice | Decision | Why |
| --- | --- | --- |
| Platform | Claude Models on Google Cloud (Anthropic managed), Go SDK `googlecloud` client | Same Claude API and features (Message Batches included); authenticates with the job's Google service account through Application Default Credentials, so there is no API key to store; billed through the Google Cloud project. Google's Vertex AI offering lacks Message Batches |
| Model | `claude-opus-5-5` | Current default model. Whether a cheaper model holds quality on a task is measured with the evaluation set before any change |
| Thinking and effort | Adaptive thinking (always on for this model); `effort` set explicitly per task | The model's default effort is `medium`; set it explicitly and tune it per task against the evaluation set |
| Output shape | Structured outputs (`output_config.format`) with a JSON schema per task | Outputs parse and validate by construction; the schema includes the anchor list per statement |
| Batching | Message Batches API | Roughly half the price, asynchronous, matches the low urgency of ingestion ([01](../design/01-bill-ingestion-and-display.md)) |
| Caching | Prompt caching on the stable prefix: instructions, then the bill's full text | Many section-level tasks share one bill's text, so later tasks read it from cache. Volatile values go after the cached prefix |
| Refusals | Check `stop_reason` before reading output; enable the server-side `fallbacks` beta in its `"default"` form | A refused or truncated (`max_tokens`) task is recorded with its outcome and retried or sent to a steward, never published |

Model ids, prices, and feature availability on the platform are confirmed at M1 start. The `claude` adapter keeps all of this behind the `Generator` port, so services never import the SDK.

### Accuracy checks (before stewards)
Accuracy is measured against each statement's **cited lines**, which needs no reference text. Every statement in an output lists the bill line anchors and provision citations it rests on, so each check compares a statement with exactly those lines.

| Check | What it does | When it runs | Effect |
| --- | --- | --- | --- |
| **Fact fidelity** (deterministic) | Every number, date, dollar amount, percentage, and section or US Code reference in a statement must appear in its cited lines | Every collect phase; in CI on `fakegen` outputs | A failing output is recorded `unverified` and not published |
| **Amendment consistency** (deterministic) | Parses the bill's amendment instructions ("striking X and inserting Y", "is amended by adding", "is repealed") in the cited lines, and checks the explanation names the same target provision, removed text, and inserted text | Same, for amendatory text | Same |
| **Grounding grader** (model) | A grader model, different from the generating model, labels each statement *supported*, *unsupported*, or *contradicted* by its cited lines, and rates neutral wording | Local live runs only (stage L3), with operator approval; never in CI | *Contradicted* outputs are withheld; grades are stored beside the output |
| **Planted errors** | Injects known errors into good outputs (changed numbers, swapped sections, reversed effect, evaluative wording) and measures how many each check catches | Deterministic checks in CI; grader in local live runs | Reports each check's catch rate |
| **Operator sample** | The operator grades about 30 statements against their cited lines once, recorded as an operator check, not a steward review | Once in M1, before the first report | Reports grader agreement with the operator |
| **CRS summary coverage** | For bill summaries, the grader compares our summary with the Congressional Research Service summary of the same version from `BILLSTATUS` (public domain): points covered, points contradicted | Local live runs | Reports coverage and contradictions |

- **CRS summaries are references, never inputs.** They are not passed to the generating model, so the coverage check stays independent.
- **Grades are stored** at `generated/{task}/{bill}/v/{n}/{key}.grade.json` with the grader model and prompt version.
- **Metrics per task:** share of statements *supported*; share *contradicted*, reported separately; fact-fidelity and amendment-consistency failure rates; each check's catch rate on planted errors; grader-to-operator agreement; CRS coverage for bill summaries. The M1 report proposes an M3 target from these.
- **Reports** are written by a local command (`cmd/accuracy-report`, a development tool, not a deployed image) to `test/accuracy/report.md`, with the inputs needed to reproduce them.

### Evaluation (with stewards)
**Not built until stewards exist.** The references are steward-written, so the evaluation set, its grader, and evaluation runs start with the steward program. The accuracy checks above keep running beside it.

- An evaluation set of real bills with steward-written reference summaries and explanations, covering amendatory text, insertions, repeals, and freestanding text ([01](../design/01-bill-ingestion-and-display.md)).
- Graded on factual accuracy against the cited lines, completeness, neutral wording, and anchor correctness.
- Run before changing the model, effort, or prompt version, and its results decide the per-task display policy. Runs spend money, so they are triggered deliberately, not on every commit.
- `fakegen` fixtures cover the job's code paths in CI without calling a model.

### Live model calls
- **Local (stage L3):** an operator-supplied Anthropic API key with the first-party client, on the operator's machine only. The key is never committed and never given to CI.
- **CI:** never calls a model. Every generation path runs on `fakegen`. Live runs stay behind a switch that is off by default (a workflow input or repository variable), so they can be turned on later without changing the workflow's shape.

## Notifications

Implements [08 Notifications](../design/08-onboarding-and-verification.md#notifications): an in-app inbox for everyone, and opt-in browser and installed-app notifications through Web Push. No email.

### Finding whom to notify without a supporter list
Notifying "everyone who supported X" would need a stored list of who supported X, which the design forbids. Instead:
1. **Events are public.** Revisions, stale flags, new versions, markup decisions, office responses, and adoptions are already entries in the public change feed ([10](../design/10-machine-access.md)).
2. **Each person's own interests are private.** `my_signals` holds each person's items, encrypted.
3. **`job-notify` joins the two in memory.** After each count batch, for each account with at least one device subscription, it decrypts that person's `my_signals`, reads feed events since their `last_event_cursor`, and if any match, sends one content-free push to each of their devices. It then advances the cursor.
4. **Nothing joined is stored.** No item-to-person index exists at rest. A copy of the database shows which accounts have devices, not what they took part in.

The inbox (`GET /v1/me/notifications`) does the same join on request for the signed-in person, from their `last_read_cursor`, so it works with no device subscription at all.

### Web Push
- **Subscribe.** The service worker subscribes using the VAPID public key from `GET /v1/push/public-key` and sends the subscription to `write-api`, which stores the endpoint and keys encrypted with a dedicated key.
- **Payload.** Encrypted end to end (RFC 8291) and content-free: `{ "kind": "updates" }`. The service worker shows "You have updates on PubCommittee" and opens the inbox. The push service sees only that a message was sent to a device, and when.
- **Signing.** `job-notify` signs pushes with the VAPID private key from Secret Manager. Only `job-notify` holds it, and only `job-notify` and `write-api` can decrypt `my_signals` and subscriptions.
- **Cleanup.** Subscriptions the push service reports as gone are deleted; signing out or turning notifications off deletes that device's row.
- **Installed apps.** Some mobile browsers deliver Web Push only to installed web apps, one reason the UI must be installable ([Progressive web app](#progressive-web-app)).

### Cost and limits
`job-notify` work grows with the number of accounts that have device subscriptions, which is small at pilot scale. If it grows large, the job can be sharded by account range; it still never stores the join.

## API contract

**Decided: REST** over HTTPS with JSON bodies, specified in **OpenAPI 3.1** at `api/openapi.yaml`. The spec is the contract between the backend and every client: the web UI, agents, and third parties are equal consumers.

### Why REST
- **Caching is the scale-to-zero strategy.** Every resource has a stable URL, so the CDN answers most reads and the backend stays asleep. Immutable bill versions cache indefinitely.
- **Freshness is per resource.** Text, review items, and counts change at very different rates and are separate resources with separate cache lifetimes.
- **Machine access.** API addresses mirror page addresses; agents and crawlers need only `GET` and JSON ([10](../design/10-machine-access.md)).
- **Fixed shapes resist profiling.** There is no endpoint that lists one author's activity across bills, and no query language to build one ([11](../design/11-ai-security.md)).
- **Small services.** Handlers on the standard library router; no query engine, resolvers, or batching loaders.
- **Telemetry fits.** Route patterns are the `http.route` attribute and status codes carry outcomes ([Observability](#observability)).

GraphQL was considered and rejected for these reasons. If a GraphQL view is wanted later, it can be a separate read-only layer over the public REST API and exports.

### Resources

Resources are cut by **freshness and boundary**, not by screen. A page that needs several resources fetches them in parallel during server rendering.

| Resource | Service | Auth | Cache |
| --- | --- | --- | --- |
| `GET /v1/bills`, `GET /v1/bills/{bill}` | `read-api` | None | Short. Lists the tracked set; `?scope=catalog` lists catalog entries |
| `GET /v1/selection-policy`, `GET /v1/selection-policy/{version}` | `read-api` | None | Short; versions immutable |
| `GET /v1/bills?scope=catalog&status=&chamber=&committee_activity=&subject=&q=` | `read-api` | None | Short. Catalog entries with official links and markup status |
| `GET /v1/markup-decisions`, `GET /v1/markup-decisions/{id}` | `read-api` | None | Short |
| `GET /v1/bills/{bill}/v/{n}` | `read-api` | None | Immutable: metadata, sections, lines, anchors |
| `GET /v1/bills/{bill}/v/{n}/s/{section}`, `.../s/{section}/l/{range}` | `read-api` | None | Immutable |
| `GET /v1/bills/{bill}/s/{section}`, `.../s/{section}/l/{range}` | `read-api` | None | Short: latest version, with an `address` block naming the pinned and latest versions |
| `GET /v1/bills/{bill}/v/{n}/summary`, `.../changes` | `read-api` | None | Short: bill summary and change summary against the previous version ([M1-09](../design/tasks/m1/09-ai-generation.md)) |
| `GET /v1/bills/{bill}/v/{n}/citations?state=`, `.../citations/unresolved` | `read-api` | None | Short ([M1-07](../design/tasks/m1/07-us-code-and-citations.md)) |
| `GET /v1/bills/{bill}/v/{n}/compare/{m}` | `read-api` | None | Immutable: base `n` against `m` ([M1-08](../design/tasks/m1/08-version-history-and-stale.md)) |
| `GET /v1/bills/{bill}/v/{n}/explanations` | `read-api` | None | Short (stewards correct explanations) |
| `GET /v1/bills/{bill}/v/{n}/review` | `read-api` | None | Short: threads and suggestions for that version |
| `GET /v1/threads/{id}`, `GET /v1/suggestions/{id}` | `read-api` | None | Short |
| `GET /v1/issues`, `GET /v1/issues/{id}` | `read-api` | None | Short |
| `GET /v1/briefs/{bill}` | `read-api` | None | Until next brief |
| `GET /v1/snapshots/bills/{bill}` | `read-api` | None | Until next batch |
| `GET /v1/moderation-log` | `read-api` | None | Short |
| `GET /v1/feed`, `GET /v1/exports/{export}` | `read-api` | None | Until next batch |
| `GET /v1`, `GET /v1/openapi.yaml` | `read-api` | None | Short |
| `GET /auth/{provider}/start`, `GET /auth/{provider}/callback`, `POST /auth/signout` | `write-api` | Session created here | Never |
| `GET /v1/me`, `GET /v1/me/signals` (includes markup requests) | `write-api` | Session cookie | Never |
| `GET /v1/me/notifications`, `POST /v1/me/notifications/read` | `write-api` | Session cookie | Never |
| `GET /v1/push/public-key` | `write-api` | None | Long (the VAPID public key) |
| `PUT /v1/me/push-subscriptions`, `DELETE /v1/me/push-subscriptions/{id}` | `write-api` | Session, Origin, anti-forgery token | Never |
| `POST /v1/threads`, `POST /v1/threads/{id}/comments` | `write-api` | Session, Origin, anti-forgery token | Never |
| `POST /v1/suggestions`, `POST /v1/suggestions/{id}/revisions` | `write-api` | Same | Never |
| `PUT /v1/signals/{item}`, `DELETE /v1/signals/{item}` | `write-api` | Same | Never |
| `PUT /v1/markup-requests/{bill}` (optional reason), `DELETE /v1/markup-requests/{bill}` | `write-api` | Same; bill must be `eligible` | Never |
| `GET /v1/steward/markup-queue`, `POST /v1/steward/nominations`, `POST /v1/steward/markup-decisions`, `POST /v1/steward/markup-decisions/{id}/signoff` | `write-api` | Same; steward or office role; second sign-off from another party | Never |
| `POST /v1/flags`, `PUT /v1/responses/{suggestion}` | `write-api` | Same; responses need an office role | Never |

`PUT` and `DELETE` on signals make set and withdraw idempotent, matching `SignalLedger`. The paths above are a starting outline; `api/openapi.yaml` is authoritative once written.

Path parameters (M1-02): `{n}` is the 1-based position in the bill's version list, or `latest`; the version code (for example `ih`) is the `code` field. `{range}` is `12` or `12-13`, inclusive, the same form as the UI fragment `#s4-l12-13`. A block has no syntax of its own: each line carries `block_to`, and the block is the range `n-block_to`. `markup-decisions` routes are not yet in the spec.

Cache classes (M1-10): immutable `public, max-age=31536000, immutable`; short `public, max-age=60, stale-while-revalidate=300`; batch `public, max-age=900`. Answers resolved from `latest` use the short class. `GET /v1/bills` answers with `anyOf` bill summary or catalog entry, because a tracked catalog entry satisfies both.

### Conventions
- **Versioning.** Major version in the path (`/v1`). Additive changes do not bump it. A new major version runs beside the old one for a published period ([10](../design/10-machine-access.md)).
- **Representation.** Plain JSON, `snake_case` fields, UTC timestamps in RFC 3339 (public timestamps coarsened to the day where [11](../design/11-ai-security.md) requires). No envelope; collections return `{ "items": [...], "next": "<cursor>" }`.
- **Provenance on every record.** `version`, `as_of`, `provenance`, and `content_class` (`official`, `platform`, `user`) on each record that carries text ([10](../design/10-machine-access.md#instruction-injection)).
- **Pagination.** Opaque cursors from the adapter, passed as `?cursor=`. No offsets.
- **Caching.** Strong `ETag` on every `GET`, with `If-None-Match` returning `304`. `Cache-Control` per resource as in the table; `immutable` for official text. Credentialed responses are `private, no-store`.
- **Errors.** RFC 9457 problem details (`application/problem+json`) with a stable `type` per typed data error, in the path form `/problems/<name>` (`not-found`, `conflict`, `invalid`, `rate-limited`, `unavailable`). No stack traces or vendor errors.
- **Rate limits.** `429` with `Retry-After`; published limits per route class, as ranges. `read-api` limits in memory per instance, keyed on `HMAC(k_rate, ip ‖ day)` that never leaves memory; it holds no storage port for limits.
- **Feed.** `GET /v1/feed?since=<RFC 3339>` in the collection convention; entries carry type, day-granular time, and canonical address.
- **Licences.** Official text and US Code provisions are marked public domain; platform-generated content is CC0 1.0.
- **Writes.** JSON request bodies only; `Content-Type` checked. Every write requires the session cookie, an `Origin` of `https://<domain>`, and the anti-forgery token header.
- **CORS.** `read-api`: any origin, no credentials. `write-api`: only `https://<domain>`, with credentials.
- **Discovery.** `GET /v1` lists top-level resources and links to the spec at `GET /v1/openapi.yaml`; UI pages link to their resource with `rel="alternate"`, and answer `Accept: application/json` with `303 See Other` to it.

### Contract workflow
- `api/openapi.yaml` is written first and reviewed like code.
- Go server types and request validation are generated from it (`oapi-codegen`), so handlers cannot drift from the spec.
- TypeScript types for `app/` are generated from it (`openapi-typescript`).
- CI fails if generated code is out of date, and runs contract tests that call each service and validate responses against the spec.
- The spec is published at a stable URL for agents and third parties.

## Administration

Implements [platform administration](../design/05-roles-and-stewardship.md#platform-administration) as a **privileged architecture feature, not a product feature**. There is no administrator role in the product's role model, no administrator account type, and no administrator screen in the web UI. Administrative access is an infrastructure grant.

### Service and access
- **`admin-api`** is a separate Go service on Cloud Run with its own service account. It is **not** behind the API edge and does not accept citizen sessions.
- **Authentication is Google Cloud IAM.** The service requires an identity token; only principals granted the invoker role in the production project can call it. No load balancer or identity proxy is needed, so it still scales to zero.
- **Operators use `pubctl`,** a Go command-line tool that calls `admin-api` with the operator's Google credentials (`gcloud auth` / Application Default Credentials). No UI is built on `admin-api`: it sends no CORS headers, is not in the public OpenAPI contract, and is never called from the web tier.
- **Who is an administrator is private infrastructure configuration.** The operator roster lives in the private deployment repository at `operators/operators.yaml` ([Public modules, private deployment](#public-modules-private-deployment)): a stable id and public display name for each administrator, and their Google principal per environment.
    ```yaml
    operators:
      - id: op-1
        name: "[Operator name]"
        principals:
          dev: "user:<operator-principal>"
          staging: "user:<operator-principal>"
          prod: "user:<operator-principal>"
    ```
  - Infrastructure code turns the roster into the IAM invoker binding on `admin-api` and the service's operator list. Adding, renaming, or removing an administrator is a reviewed change in the deployment repository, not an action in the product.
  - Principals are never committed to this repository, logged, or published ([What stays private](#what-stays-private)). The roster file itself is not published.
- **Roster changes are public.** Granting or removing administrator access is a privileged action (principle 4), so every roster change is written to the public log before it takes effect ([Roster log](#roster-log)). The list of who can act is therefore reconstructible from the public log, while principals stay private.
- **Attribution.** `admin-api` maps the caller's verified Google identity to an operator id and refuses anyone not on the list. The public log records the operator id and public name, never the principal. Every mutating call requires a reason.
- **Separate from product accounts.** Administrator identities never appear in `accounts` or `roles`. An administrator who also takes part as a reviewer signs in through the product like anyone else, with no added standing.
- **Least privilege.** `admin-api` holds `PlatformPorts`, `Queue`, and `Generator`, but never calls `SignalLedger` aggregate or reads `request_reasons`; a dependency rule keeps those operations out of its wiring.

### Actions

| Action | Route | Effect |
| --- | --- | --- |
| Select a bill | `POST /admin/v1/bills/{bill}/track` | Sets `tracked` with reason `admin:{log id}`, enqueues the `worker` task that fetches and parses all versions and triggers `job-summarize`. Refused for excluded measure kinds. After the early-pilot phase, creates a pending decision needing two-party steward sign-off instead |
| Correct an error | `POST /admin/v1/bills/{bill}/untrack`, `PATCH /admin/v1/bills/{bill}/catalog` | Ingestion-error corrections only; reason required |
| Re-ingest or re-parse | `POST /admin/v1/bills/{bill}/reingest`, `POST /admin/v1/bills/{bill}/v/{n}/reparse` | New fetch is checked against the stored SHA-256; a re-parse writes a new parser version beside the old one |
| Regenerate | `POST /admin/v1/generations` (task, bill, version) | Submits generation tasks under a new key; outputs go to steward review |
| Publish policy | `POST /admin/v1/selection-policy` | Writes the next immutable `selection_policy` version |
| Product roles | `POST /admin/v1/roles`, `DELETE /admin/v1/roles/{id}` | Grants and revokes steward, moderator, and office roles; steward and office grants record their verification. Administrator access is not grantable here |
| Failures | `GET /admin/v1/ingest/failures`, `GET /admin/v1/jobs` | Watermarks, retries, last runs |
| Operator roster | `POST /admin/v1/operators/roster` | Called only by the environment's deploy service account during a deploy, never by `pubctl`. Writes one public log entry per roster change ([Roster log](#roster-log)) |

```sh
pubctl bills track us-119-hr-4127 --reason "pilot seed: healthcare, reported by committee"
pubctl bills reparse us-119-hr-4127 --version 2 --reason "parser fix for tables"
```

### Roster log
The public log records every change to the operator roster, so administrator access is public without a public roster file.

1. A roster change is a pull request in the deployment repository, approved by a current operator. Each roster entry carries the operator's GitHub login (private, like the principals), so CD can map a review to an operator id. The file holds no approver or reason: those come from the pull request.
2. Before applying infrastructure, the deployment repository's CD finds the merged pull request that changed the roster and takes the approver from its reviews: an `APPROVED` review on the pull request's final commit, by a login that maps to an operator on the roster **before** the change (read from the base commit). The reason is the pull request title. A roster change that reached `main` without such a pull request is refused. CD then calls `POST /admin/v1/operators/roster` in each environment with the public part of the new roster (`id`, `name`), the approving operator id, and the reason. Principals and GitHub logins are never sent.
3. `admin-api` rebuilds the current roster from earlier roster entries in `moderation_log`, compares, and writes one entry per change: `operator_added`, `operator_renamed`, or `operator_removed`, each with operator id, public name, approver, reason, and day. No change writes nothing, so the call is idempotent.
4. Only if the log write succeeds does CD apply the IAM binding and `admin-api`'s operator list. If the apply fails, the next run converges; the log is never written after the fact.
5. The approver must be on the roster before the change; GitHub does not let an author approve their own pull request, so one operator cannot add themself alone. The very first roster, when the log has no roster entries yet, is the bootstrap: it needs no approver and its entries say so.

The caller is the environment's deploy service account. It is `admin-api`'s only caller that is not an operator, and is allowed this one route only.

### Logging
- Every mutating action writes a public entry to `moderation_log` (actor: the named administrator, action, target, reason, time) in the same transaction as the change. An action whose log write fails is not applied.
- The `phase` (early pilot or steward sign-off) is a field of the current `selection_policy`, so the switch is itself a logged, versioned change.
- `job-ingest` includes the count of administrator-selected bills in its published tracked-set breakdown ([09](../design/09-integrity-moderation-and-privacy.md#coverage-neutrality)).

## Request paths

| Path | Flow | Touches SQL |
| --- | --- | --- |
| Read a bill page | Web host CDN → (miss) SvelteKit on the web host → API edge → (miss) `read-api` → object store + public docs | No |
| Read counts | API edge → (miss) `read-api` → snapshot JSON | No |
| Machine access, exports, feed | API edge → (miss) `read-api` → object store | No |
| Post a comment or suggestion | Browser → `write-api`: session, Origin, token, rate limit → document store | No |
| Signal | Browser → `write-api`: session, eligibility, rate limit → `signals` upsert by `signal_key` | Yes |
| Turn on device notifications | Browser asks permission → service worker subscribes with the VAPID public key → `write-api` stores the encrypted subscription in `push_subscriptions` | Yes |
| Request markup | Browser → `write-api`: session, bill `eligible`, rate limit → `signals` upsert (`markup_request`), optional `request_reasons` row | Yes |
| Decide a request | Steward → `write-api`: role, conflict rule, second sign-off → `markup_decisions`, `bills.tracked`, `worker` task to fetch text | Yes (role check) |
| Sign in | Browser → `write-api` → provider → callback → `identities` lookup or insert → session cookie → redirect to `<domain>` | Yes |

## Scheduled jobs

| Job | Cadence | Reads | Writes |
| --- | --- | --- | --- |
| Ingest | Every few hours for the current Congress (matching the source's update cycle); daily for prior Congresses | `BillSource` since the stored watermark | Object store originals and parsed text, `bills`, `ingest_state`, re-anchor queue, summarize trigger |
| Ingest existing law | Weekly check for a new release point | `LawSource` | Object store provisions with release-point date; freshness flags |
| Count batcher | Fixed interval (open question in [11](../design/11-ai-security.md)) | SQL `signals` (support, opposition, markup requests) | Snapshot JSON, SQL `snapshots`; applies small-count and complementary suppression; adds bills reaching the request threshold to `markup_queue` |
| Brief builder | Weekly, plus before markup and floor events | Snapshots, public docs | `briefs`, rendered brief, delivery queue |
| Export and feed builder | After each batch | Object store, public docs | Exports, feed; drops removed content |
| Spike and automation detector | After each batch | SQL aggregates by day | Integrity events in public docs |
| Notify | After each count batch | `notify_state`, opted-in accounts' `my_signals` (decrypted in memory), public change feed since each cursor | Content-free Web Push per device with updates; cursors advanced; gone subscriptions deleted |
| Summarize | After ingest, for tracked bills only; then on a short interval until its batches finish | Object store text and cited provisions, `generations` | Batch submissions; stored outputs, `generations`, steward review queue |

## Hosting

| Need | Chosen | Alternative (future adapters) |
| --- | --- | --- |
| Web UI | Any Node web host (SvelteKit, server rendering + CDN) | Not a backend choice; the host is interchangeable |
| API edge | Firebase Hosting rewrites to Cloud Run | CloudFront |
| APIs and worker | Cloud Run services | Lambda running the same images through an HTTP adapter (ECS on Fargate also runs them, but does not scale to zero) |
| Jobs | Cloud Run jobs + Cloud Scheduler | ECS scheduled tasks via EventBridge Scheduler |
| Document store | Firestore (native mode) | DynamoDB on-demand |
| Object store | Cloud Storage | S3 |
| SQL | Serverless Postgres (e.g. Neon) over TLS | Aurora Serverless v2 with auto-pause |
| Keys | Cloud KMS | AWS KMS |
| Queue | Cloud Tasks | SQS |

Firestore's query model fits nested, per-bill documents; Cloud Run runs ordinary containers and scales to zero; Firebase Hosting gives the APIs a CDN and custom domain without a fixed-cost load balancer. Because every backend unit is a container and every store sits behind an adapter, moving providers means swapping adapters and the container runtime, not rewriting services.

### Fixed-cost traps to avoid
- Load balancers and NAT gateways bill hourly even when idle.
- Managed SQL instances that do not pause bill around the clock.
- Minimum instances, provisioned concurrency, and private-network connectors for serverless services.
- One key-management key costs little, but a key per tenant or per bill adds up; derive per-bill values with HMAC instead.

### Cold starts
- Readers hit the web host's CDN and the API edge, so backend cold starts are mostly felt on cache misses and writes.
- Go services start in tens of milliseconds; image pull and container start dominate.
- A paused serverless Postgres takes from under a second to several seconds to resume, depending on vendor. Acceptable for writes and jobs; one more reason the read path never uses SQL.

## Environments and delivery
- Separate Google Cloud projects for `dev`, `staging`, `prod`, and matching web-host environments. No shared credentials.
- Deploys from CI with short-lived federated credentials, never stored keys.

### Public modules, private deployment
Infrastructure code is split between two repositories:

| Repository | Holds | Visibility |
| --- | --- | --- |
| This repository, `infra/` | Reusable modules named by vendor (`infra/modules/gcp/...`), a placeholder example root | Public |
| Private deployment repository | Environment roots for `dev`, `staging`, `prod`; state backend config; per-environment values; the operator roster (`operators/operators.yaml`); deploy workflows and their federated credentials; the real-value leak check | Private |

- **Why public modules.** A new unit and its Cloud Run service or job, schedule, service account, and IAM bindings are reviewed in one change with the code. Anyone can check least-privilege claims (for example, that `read-api`'s service account has no SQL or key access) from the modules themselves.
- **Vendor-specific by name.** Modules are per vendor, while services stay vendor-neutral through adapters. Another vendor means new modules under `infra/modules/<vendor>/`, not changes to services.
- **Module rules.** No defaults that are real values; every name, project, region, and domain is an input. No network specifics or defense tuning (rate-limit values, detection thresholds) hard-coded; those are inputs set privately. No sensitive outputs. CI runs `validate` and a plan of `infra/examples/` against fake values.
- **Pinning.** The deployment repository pins modules by tag (`?ref=vX.Y.Z`). Changing a module is a tagged release here, then a pin bump there. The application commit and image digests running in each environment are recorded by each deploy ([Continuous delivery](#continuous-delivery)).
- **Where images are built and deployed.** This repository's CI builds and tests images but never pushes or deploys, because its logs are public and would show project ids and service URLs. The deployment repository's CI builds, pushes, and deploys ([Continuous delivery](#continuous-delivery)).
- **Local layout.** The deployment repository may be checked out at `deploy/` inside this repository; that path is ignored here ([`.gitignore`](../.gitignore)). Local module development can point a module `source` at `../infra/modules/...` instead of a tag, never committed that way.
- **State** lives in a remote bucket created by the deployment repository's bootstrap root, with restricted access.

### Continuous delivery
Deploys run only in the private deployment repository. This repository's only part is to send a trigger naming the commit.

```
this repository (public)                      deployment repository (private)
────────────────────────                      ───────────────────────────────
PR ─► CI: build, test (no credentials)
merge to main ─► CI green
  └─► dispatch-deploy ── repository_dispatch ─► deploy-dev
      (GitHub App token,     {sha, kind}          verify sha is on public main
       one permission)                            leak check
release tag v* ─► dispatch-deploy ──────────────►  build changed units, push to shared registry
                                                  apply dev by digest, smoke test
                                                  record dev.json on the deployed branch
                                                promote (release or manual)
                                                  staging: same digests, approval
                                                  prod:    same digests, approval
```

- **Trigger.** A GitHub App installed on the deployment repository only, with `Contents: write` (needed for `repository_dispatch`) and nothing else. Its id and key are secrets of a `deploy-trigger` environment in this repository, limited to `main` and release tags. The dispatch job runs on `workflow_run` of a successful `push` CI run on `main`, or on a release tag, and prints only the commit and kind.
- **Untrusted payload.** The deployment repository refuses a commit that is not on this repository's `main`, or a release whose tag does not point at it. The dispatch says *which* commit; it never decides *whether* to deploy.
- **Same bytes everywhere.** Images are built once, pushed to one shared Artifact Registry, and applied by digest. `staging` and `prod` receive exactly the digests `dev` ran; nothing is rebuilt per environment.
- **Approvals.** `dev` deploys on every merge. `staging` and `prod` are GitHub environments with required reviewers.
- **Roster first.** When a deploy changes the operator roster, CD writes the roster log entries through `admin-api` before applying IAM ([Roster log](#roster-log)).
- **Deploy records.** Each deploy writes `<env>.json` (commit, kind, tag, per-unit digests, time, run id) to a `deployed` branch of the deployment repository. That branch says what each environment runs; rollback re-applies an earlier record without rebuilding.
- **Independent units.** Only units whose inputs changed are rebuilt and redeployed.
- **Secrets.** Google Cloud access is federated per environment; no keys are stored. Infrastructure creates Secret Manager secrets without values; values are added outside the IaC tool, so they never reach state or logs.
- Each backend unit deploys separately with its own service account; `read-api` never receives SQL or key permissions.
- The UI deploys from `app/` to its host and needs only the API base URLs.

## What stays private

The architecture is public on purpose: the code is open source, and the protections described here are controls (IAM, keyed hashes, credential separation, a read path with no SQL access), not secrecy. What must never be in the repository, documentation, issues, or logs are the **specifics of a running environment**.

| Never public | Where it lives instead |
| --- | --- |
| Secrets: keys, tokens, passwords, connection strings, HMAC key material, the VAPID private key | Secret Manager and KMS; workload identity instead of stored keys |
| Infrastructure state (`*.tfstate`) | Remote state bucket with restricted access |
| Per-environment values: project ids and numbers, region choices, bucket, database, queue, and key names | Private deployment repository, applied by its CI ([Public modules, private deployment](#public-modules-private-deployment)) |
| Service URLs other than the public domains, especially `admin-api` and `worker` | Environment values; discovered by CI, never written into docs |
| The deploy trigger's GitHub App private key | Secret of this repository's `deploy-trigger` environment, limited to `main` and release tags; it can only send a dispatch |
| Identities of people with privileged access: Google principals of administrators, service-account emails tied to them | Operator roster in the deployment repository ([Administration](#administration)); only ids, public names, and roster changes are published |
| Network specifics: IP allowlists, connector or peering details | Environment values |
| Defense tuning: rate-limit values, automation and spike detection thresholds, detector features | Environment configuration; published only as ranges or after the fact, so detection cannot be tuned against ([10](../design/10-machine-access.md), [11](../design/11-ai-security.md)) |
| Production data and exports beyond the public bulk exports | Production project only |

Rules:
- **Placeholders in docs.** Use `<domain>`, `<project>`, `<region>`, and similar; never paste real values into documentation or examples.
- **Examples use fake values.** Test fixtures and sample configs use obviously fake ids.
- **Checked in CI.** Secret scanning with push protection runs on every push. A format-based pattern check here fails the build on `run.app` URLs, service-account emails, and project-id-shaped values in tracked files; listing the real values here would publish them. The deployment repository holds the real values and scans this repository for them on a schedule and before each deploy.
- **Public CI never touches an environment.** This repository's workflows hold no cloud credentials and print no environment values.
- **Reports go private.** Anything found exposed is reported through [`SECURITY.md`](../SECURITY.md), not in a public issue.

## Milestone fit

| Milestone | Needs |
| --- | --- |
| M1 read-only | Local phase first, then cloud phase ([M1 phases](../design/tasks/README.md#phases)). Go module; `internal/data`, `adapters/memory`, `adapters/fsobject`, `adapters/fsdocs`, `adapters/localauth`, `adapters/gcs`, `adapters/firestore` (read ports), `adapters/claude`, `adapters/fakegen`, `telemetry`, `httpx`; `adapters/govinfo`, `adapters/uscode`, `adapters/fixturesrc`; `admin-api` and `pubctl` (privileged: direct selection, re-ingest, re-parse, regenerate, policy versions); `read-api` (including the public catalog with official links and markup status), `job-ingest` (catalog backfill of the current Congress; tracked set in pilot mode), `job-summarize` (bill summaries and change explanations; no evaluation set or steward review until stewards exist), `job-exports`; API edge; UI reading `read-api`. No SQL, no sign-in |
| M2 interactive | `adapters/postgres`, `adapters/kmsgcp`, write ports; `write-api`, sign-in, SQL (`accounts`, `identities`, `signals`, `my_signals`, `request_reasons`, `push_subscriptions`, `notify_state`), sessions, rate limits, markup requests, `job-batcher`, `job-notify` and `adapters/webpush` (inbox and device notifications), moderation log; custom domain so cookies are same-site |
| M3 pilot | `job-briefs` and delivery queue, `worker`, roles, `job-detector`; `markup_queue`, steward decision routes with two-party sign-off, `markup_decisions`, promotion of accepted bills |
| M4 launch | Audit export, log hardening review, load test of the CDN-first read path |
| M5 verification | Tier records, district fields, complementary suppression across breakdowns |

## Open decisions
- **Domain name** for `<domain>` and `api.<domain>`; required before M2 sign-in.
- **Serverless Postgres vendor**, and whether its resume latency is acceptable for sign-in.
- **Sessions in the document store or SQL:** the document store avoids a SQL resume on every signed-in request.
- **Batch interval**, shared with [11](../design/11-ai-security.md).
- **Review-item freshness on the UI:** CDN TTL for pages showing comments and suggestions.
- **Email notifications:** deferred. Revisit only if device notifications and the inbox leave too many people unreachable; it would need verified, encrypted addresses and a mail provider under a data processing agreement.
- **CI workflows** (GitHub Actions, [Tooling](#tooling)) for the Go module and `app/`, including the dependency checks, binary size budgets, and contract checks.
- **Image registry:** one shared Artifact Registry, pushed only by the deployment repository's CI, promoted by digest ([Continuous delivery](#continuous-delivery)). Which project holds it is open.
- **Self-hosted adapters** for document and object storage, if running entirely off Google Cloud becomes a goal.
- **Metrics backend and cost:** Managed Prometheus bills per sample ingested; set scrape intervals and histogram buckets with that in mind.
- **Traces:** the same OpenTelemetry setup can add tracing later; decide whether traces are wanted, given they carry request detail.
- **Catalog search:** keyword matching on `search_terms` at first; a dedicated search index if volume or quality needs it, chosen to keep scale-to-zero.
- **Markup request threshold** and steward capacity, shared with [01](../design/01-bill-ingestion-and-display.md#markup-requests).
- **Topic subject terms** for healthcare, deficit reduction, and affordability, confirmed against the legislature's subject vocabulary before the pilot.
- **Bill backfill depth:** **Decided (operator, 2026-11-22):** current Congress only in M1. Earlier Congresses for lineage detection ([04](../design/04-bill-lineage-and-competing-bills.md)) are a later decision.
- **AI display policy:** per task, whether unreviewed output is shown, and the accuracy bar: from the accuracy checks in M1, and the evaluation set once stewards exist.
- **Effort level per task**, and whether any task moves to a cheaper model, both decided from evaluation results.
- **Key rotation** for HMAC keys: rotating `k_signal` requires re-keying every signal; plan the procedure before M2.
