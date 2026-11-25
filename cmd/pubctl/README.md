# pubctl: operator guide

`pubctl` is the command-line tool operators use to run the privileged actions of M1: select bills for review, correct ingestion errors, re-run fetch and parse, publish selection policy versions, and inspect failures. It calls `admin-api` ([cmd/admin-api/README.md](../admin-api/README.md)). It is not a product screen and has no UI.

## Install and run locally

Start a local `admin-api` first (see its README), then:

```sh
go run ./cmd/pubctl --url http://127.0.0.1:8081 bills track us-119-hr-3062 --reason "pilot seed: example"
```

- `--url` (or env `PUBCTL_URL`) is the base URL of `admin-api`, an `http` or `https` URL. Required. It comes before the command.
- Bill ids are `<jurisdiction>-<session>-<type>-<number>`, for example `us-119-hr-3062`.

## Authentication

| Where `--url` points | Token |
| --- | --- |
| Loopback (`localhost`, `127.0.0.1`, `::1`) | None. `localauth` on the local target authenticates every request as the fixed local operator. |
| Anywhere else | `PUBCTL_TOKEN` if set; otherwise the gcloud ID token described below. |

Without `PUBCTL_TOKEN`, pubctl reads the active login with `gcloud config get-value account`:

- A **service account** (email ends in `.gserviceaccount.com`): `gcloud auth print-identity-token --audiences=<scheme>://<host>`. The token's audience is the service URL.
- A **user login** (anything else): `gcloud auth print-identity-token` with no `--audiences`, because gcloud refuses `--audiences` for a user account ("Invalid account type for `--audiences`. Requires valid service account."). The token's audience is gcloud's own OAuth client id; the operator of `admin-api` lists it in `ADMIN_API_CLIENT_AUDIENCES`. Cloud Run IAM accepts it.

The decision is automatic; there is no flag. To force a token, set `PUBCTL_TOKEN`.

The token is sent as `Authorization: Bearer`. The caller must hold the invoker role on `admin-api` and be on the operator list. Any other caller is refused (see Refusals).

## Commands

Every mutating command needs `--reason`.

| Command | Route | What it does |
| --- | --- | --- |
| `bills track <bill> --reason` | `POST /admin/v1/bills/{bill}/track` | Marks the bill tracked with the reason `administrator` and logs it. Does not pull text; the response names the pull. Needs a stored selection policy. |
| `bills untrack <bill> --reason` | `POST /admin/v1/bills/{bill}/untrack` | Reverses a selection made in error. Stored text is kept. |
| `bills catalog patch <bill> --reason [--title --status --chamber --introduced YYYY-MM-DD --policy-area --subjects a,b]` | `PATCH /admin/v1/bills/{bill}/catalog` | Corrects ingestion errors. Body fields: `title`, `status`, `chamber`, `introduced`, `policy_area`, `subjects`. Send at least one. |
| `bills reingest <bill> --reason` | `POST /admin/v1/bills/{bill}/reingest` | Fetches the bill again with the full pull pipeline (text, citations, comparisons, policy step). Each stored original is compared by SHA-256; a mismatch is reported and nothing is overwritten. Missing versions are stored. |
| `bills reparse <bill> --version <n> --reason` | `POST /admin/v1/bills/{bill}/v/{n}/reparse` | Parses stored version `n` again with the current parser. Reports only (see below). |
| `generations create --task T --bill <bill> --version N --reason` | `POST /admin/v1/generations` | Requests a regeneration. Body fields: `task`, `bill`, `version`. Returns 501 until M1-09. |
| `selection-policy publish (--builtin v1 \| --file policy.json) --reason` | `POST /admin/v1/selection-policy` | Stores the next immutable policy version. Body: `policy` (the policy document, as in `internal/selection/policies/v1.json`) and `reason`. The version must be the latest plus one; the latest again is a no-op. |
| `ingest failures [--source <source>/<session>]` | `GET /admin/v1/ingest/failures` | Lists failed items per source. Optional query `source=<source>/<session>` (repeatable). |
| `jobs` | `GET /admin/v1/jobs` | Lists the last state of each source, without failed items. |

Reason rules: 1 to 500 characters after trimming. The reason is published in the public log. Never put personal data or a principal in it.

## The reason and the public log

Every mutating action writes one public log entry in the same step as the change. The entry holds:

- the log kind (`bill-tracked`, `bill-untracked`, `catalog-patch`, `bill-reingest`, the kind written by policy publication, or a roster kind);
- the operator id and public name, for example `op-1` and `Operator One`;
- the reason you gave;
- the day (not the exact time).

The log never holds a Google principal. Read it with `GET /v1/moderation-log` on `read-api` (newest first, with `cursor` and `limit`, maximum 200):

```sh
curl -sS 'http://127.0.0.1:8080/v1/moderation-log?limit=20'
```

If the log write fails, the change is not applied.

## Refusals

| Situation | Answer |
| --- | --- |
| Caller not on the operator list, even with project IAM access | 403 |
| Unverified caller | 401 |
| Bill of an excluded kind (war powers resolution, commemorative) | 422 with the policy reason, for example `war powers resolution: a privileged yes-or-no question rather than drafting`. Nothing changes. |
| `bills track` in the `steward_signoff` phase | 409, "needs sign-off". Nothing changes. Selection is a two-steward decision (M3). |
| `bills track` with no stored policy | 409 |
| `catalog patch` with an empty title, or with no field | 422 |
| `catalog patch` that changes nothing | 200, `changed: false`, nothing logged |
| Missing or empty reason | 422 |

What `catalog patch` can never change: tracked state, reasons, sponsor party, bill text. Official originals are never changed by any command.

## Track, pull and reingest

`bills track` sets the bill tracked and writes its log entry. It does not pull text: locally, nothing chains steps automatically. The response has a note naming the pull. Run the pull as its own command:

- `pubctl bills reingest <bill> --reason R`: the explicit, logged pull, through admin-api. It runs the same pipeline as `job-ingest` (original text, parsed versions, citations, comparisons, policy step).
- `job-ingest --bill <bill>`: the same pull, run directly on the data directory.

If the pull fails, the bill stays tracked; run `bills reingest` again.

## Reparse

`bills reparse` reports only. A second parser version cannot be stored beside the first yet, so a different parser version answers 503 and nothing is written or logged. If the stored text differs under the same parser version, it answers 409.

## Generations

`generations create` answers 501 until M1-09 provides a generator. Nothing is submitted or logged.

## The roster route

`POST /admin/v1/operators/roster` is called only by the environment's deploy service account, during a deploy. `pubctl` never calls it. Operators get 403 on that route. See [admin-api README](../admin-api/README.md#roster).
