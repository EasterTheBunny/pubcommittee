# admin-api: service notes

`admin-api` is the privileged administration service. Operators reach it through `pubctl` ([cmd/pubctl/README.md](../pubctl/README.md)). It holds the platform ports, not the product's read path. It sends no CORS headers, is not in the public OpenAPI contract, and is never called from the web tier. Architecture: [§ Administration](../../architecture/README.md#administration).

## Status by target

| Build tag | Storage | Auth | Status |
| --- | --- | --- | --- |
| `local` | `DATA_DIR` (filesystem) | `localauth` | Runs. For local work. |
| `memory` | process memory | `localauth` | Runs. Nothing persists. |
| `gcp` | `cloudports` (GCS, Firestore) | Google ID token | Wired. Generator is nil (see below). |
| `emulator` | none | none | Returns an error; use `gcp`. |

Build and run locally (the `local` tag):

```sh
DATA_DIR=.local-data PORT=8081 go run -tags local ./cmd/admin-api
```

## Environment

| Variable | Default | Meaning |
| --- | --- | --- |
| `DATA_DIR` | none (required for `local`) | Local data directory shared with `read-api` and `job-ingest`. |
| `PORT` | `8080` | HTTP port. |
| `METRICS_PORT` | `9464` | Metrics port. |
| `INGEST_SOURCE` | `fixtures` | Bill source of pulls: `fixtures` or `govinfo`. |
| `FIXTURES_DIR` | `fixtures` | Example data directory when `INGEST_SOURCE=fixtures`. |
| `OPERATORS_FILE` | none | JSON operator list (below). Without it: one operator `op-local` ("Local Operator", principal `local-operator`) and deploy principal `local-deploy`. |

On `gcp` the variables below are required; the operator list is never read from a file:

| Variable (`gcp`) | Meaning |
| --- | --- |
| `ADMIN_API_AUDIENCE` | Required. Accepted ID token audience for service account callers (such as the deploy account): the service URL, `https://<host>` with no path. `pubctl` sends a service account token for `scheme://host` of its `--url`, so the two match when `--url` is the service URL. |
| `ADMIN_API_CLIENT_AUDIENCES` | Optional, comma-separated OAuth client ids, also accepted as audience. A user's `gcloud auth print-identity-token` carries gcloud's OAuth client id as `aud`, never the service URL. Empty or unset: only `ADMIN_API_AUDIENCE` is accepted. Blank entries are ignored. |
| `OPERATORS_JSON` | Operator list in the format below. Unset or invalid: refuses to start. |
| `DEPLOY_PRINCIPAL` | Deploy account in IAM member form. Unset: refuses to start. If `OPERATORS_JSON` also holds `deploy_principal`, the two must agree. |
| cloud port variables | As `cloudports.Open` (project, bucket, database). |

Principals are environment values and are never logged.

Run `admin-api -healthcheck` to call its own `/healthz` (used by the image health check).

## Operator list

`OPERATORS_FILE` holds fake principals only in local work. Shape, from `internal/adminapi/operators.go`:

```json
{"operators":[{"id":"op-1","name":"Operator One","principal":"user:<operator-principal>"}],"deploy_principal":"serviceAccount:<deploy-account>"}
```

Rules checked at load: every operator has an `id`, `name` and `principal`; ids and principals are unique; the deploy principal is not also an operator. Unknown fields are an error. In a deployed environment the file is generated from the private roster; this repository does not hold it.

## Local target and refusals

`localauth` authenticates every request as one fixed operator (`local-operator`). To try the refusal rules, a local request can name another fake principal with the header `X-Local-Principal`. The header works only in `local` and `memory` builds; any value not on the operator list gets 403:

```sh
# acts as the fixed operator (op-local)
curl -sS -X POST http://127.0.0.1:8081/admin/v1/bills/us-119-hr-3062/track \
  -H 'Content-Type: application/json' -d '{"reason":"pilot seed: example"}'

# acts as a stranger: refused with 403
curl -sS -X POST http://127.0.0.1:8081/admin/v1/bills/us-119-hr-3062/track \
  -H 'X-Local-Principal: user:not-an-operator' -H 'Content-Type: application/json' \
  -d '{"reason":"try a refusal"}'

# acts as the deploy account: the roster route only; any other route is 403
curl -sS http://127.0.0.1:8081/admin/v1/jobs -H 'X-Local-Principal: local-deploy'
```

## Routes

Every route authenticates first. A principal of the wrong role gets 403. Operators are principals in the operator list.

| Method and path | Who may call | Body or query | Effect |
| --- | --- | --- | --- |
| `POST /admin/v1/bills/{bill}/track` | operator | `reason` | Track a bill and log it. No pull; reingest pulls. |
| `POST /admin/v1/bills/{bill}/untrack` | operator | `reason` | Untrack a bill; stored text is kept. |
| `PATCH /admin/v1/bills/{bill}/catalog` | operator | `reason`, and any of `title`, `status`, `chamber`, `introduced`, `policy_area`, `subjects` | Correct ingestion errors. |
| `POST /admin/v1/bills/{bill}/reingest` | operator | `reason` | Pull again, verify by SHA-256. |
| `POST /admin/v1/bills/{bill}/v/{n}/reparse` | operator | `reason` | Reparse stored version `n`; reports only. |
| `POST /admin/v1/generations` | operator | `reason`, `task`, `bill`, `version` | Regenerate; 501 until M1-09. |
| `POST /admin/v1/selection-policy` | operator | `reason`, `policy` | Publish the next policy version. |
| `GET /admin/v1/ingest/failures` | operator | optional `source=<source>/<session>` | Failed items per source. |
| `GET /admin/v1/jobs` | operator | optional `source=<source>/<session>` | Last state per source. |
| `POST /admin/v1/operators/roster` | deploy account only | `reason`, `operators`, `approver` | Roster log (below). |

Errors are RFC 9457 problems: 400 bad JSON or unknown field, 401 authentication failed, 403 wrong role, 404 not found, 409 conflict, 422 missing reason or invalid value, 503 unavailable, 501 generation not wired.

## Roster route

Body:

```json
{"reason":"<public reason>","operators":[{"id":"op-1","name":"Operator One"}],"approver":"op-1"}
```

- `operators` is the full new roster: public id and name only. Principals never arrive.
- `approver` is the id of the approving operator. Empty only for the bootstrap.
- Bootstrap: when the public log holds no roster entries yet, the first call needs no approver. Its entries are marked `bootstrap`.
- The approver must be on the roster before this change. Otherwise 403.
- One log entry per change: `operator_added`, `operator_renamed`, `operator_removed`. Entries are written in id order, additions first.
- Idempotent: an unchanged roster writes nothing and answers `written: []`.
- Roster entries are written before the deployment applies IAM. A failed write is never logged after the fact.

Example, bootstrap against a local `admin-api` (the deploy account is `local-deploy` with no `OPERATORS_FILE`):

```sh
curl -sS -X POST http://127.0.0.1:8081/admin/v1/operators/roster \
  -H 'X-Local-Principal: local-deploy' -H 'Content-Type: application/json' \
  -d '{"reason":"bootstrap roster (fake ids)","operators":[{"id":"op-1","name":"Operator One"}]}'
```

## Logs and failures

Each mutating action writes one public log entry with kind, subject, reason, operator id and public name, and day. Read them at `GET /v1/moderation-log` on `read-api`. The principal is never logged, echoed, or put in an error.

## Gotchas

- A `track` with no stored selection policy answers 409; publish a policy first (`POST /admin/v1/selection-policy`).
- `track` does not pull. Run `reingest` (or `job-ingest --bill`) to pull text, citations and comparisons; a failed pull leaves the bill tracked and the response carries `pull_error`.
- `reparse` answers 503 for a new parser version; it stores nothing.

## Authentication on gcp

Every request must carry `Authorization: Bearer <Google ID token>`. admin-api verifies signature and expiry with Google's keys, then requires the audience to be `ADMIN_API_AUDIENCE` or one of `ADMIN_API_CLIENT_AUDIENCES`, and requires a present, verified `email` claim. The principal is `serviceAccount:<email>` for emails ending in `.gserviceaccount.com`, otherwise `user:<email>`. Anything else is 401. Cloud Run IAM checks the token in front; this check is defense in depth and gives the identity.

Generation: the gcp `Generator` is nil in M1 (the admin-api service account holds no model role), so `POST /admin/v1/generations` answers as unavailable.
