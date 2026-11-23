# M1-12 — Administration

## Goal

Give named operators privileged system access to run M1: select catalog bills for review, correct ingestion errors, re-run ingestion, parsing, and generation, publish selection policy versions, and inspect failures. There is no product screen. Every mutating action is public, attributed to the named administrator, and carries a reason.

## Read first

- [05 § Platform administration](../../05-roles-and-stewardship.md#platform-administration)
- [architecture § Administration](../../../architecture/README.md#administration) (service and access, actions, logging)
- [architecture § What stays private](../../../architecture/README.md#what-stays-private)

## Scope

**In**
- `cmd/admin-api`: Go service on Cloud Run, IAM-authenticated, not behind the API edge, no CORS, not in the public contract.
- `cmd/pubctl`: command-line tool using the operator's Google credentials.
- Operator list as configuration: ids, public names, and principals come from the operator roster in the private deployment repository (`operators/operators.yaml`; M1-15 applies it). Nothing about operators is committed to this repository; examples use fake values.
- `POST /admin/v1/operators/roster`: the roster log route ([architecture § Roster log](../../../architecture/README.md#roster-log)). Accepts only the environment's deploy service account; rebuilds the current roster from earlier roster log entries; writes `operator_added`, `operator_renamed`, `operator_removed` entries with id, public name, approving operator, reason, and day; idempotent; never receives principals.
- M1 actions:
  - `bills track` (refused for excluded kinds; in `early_pilot` phase applies directly; in `steward_signoff` phase returns "needs sign-off", since the decision flow is M3);
  - `bills untrack` and `bills catalog patch` for ingestion errors only;
  - `bills reingest` (re-fetch compared by SHA-256) and `bills reparse` (new parser version beside the old);
  - `generations create` (regenerate under a new key);
  - `selection-policy publish` (next immutable version, using M1-06);
  - `ingest failures` and `jobs` inspection.
- Public log entry for every mutating action, written in the same transaction as the change; read through `GET /v1/moderation-log`.
- How a newly selected bill gets its text in M1: an open question shared with the manual pull in [M1-05](05-manual-bill-pull-and-storage.md).

**Out**
- Role grants: not in the M1 administration scope in [MILESTONES § M1](../../MILESTONES.md#m1--real-bill-text-read-only).
- Any administrator UI.

## Deliverables

- `cmd/admin-api`, `cmd/pubctl`, roster log route, roster entry kinds in the public log schema ([M1-02](02-api-contract.md))
- Operator guide (`pubctl` usage, required reason, what is logged)
- Tests for authorisation, refusal rules, and log atomicity

## Requirements

- Only principals on the operator list can call; callers not on the list are refused even with project IAM access. The deploy service account is the one exception, and may call only the roster route.
- A roster change is logged before it takes effect; the approving operator must be on the roster before the change (the bootstrap entry excepted, and labelled).
- The public log records the operator id and public name, never the Google principal.
- Every mutating call requires a non-empty reason.
- An action whose log write fails is not applied.
- Corrections are for ingestion errors only, never a bill's content or politics; official originals are never changed.
- `admin-api` wiring excludes any operation that could read individual signals or request reasons (none exist in M1; keep the rule in its dependency check).
- Tracked-set breakdown counts administrator-selected bills (M1-06).

## Suggested breakdown

1. Operator list format, loader, and identity-to-operator mapping from the verified token.
2. `admin-api` skeleton behind an operator-auth port: `localauth` on the `local` target, Google ID token verification on `gcp`; reason requirement.
3. Track and untrack with phase rule and kind refusal; catalog patch.
4. Reingest and reparse calling M1-05 and M1-04 paths.
5. Regenerate calling M1-09; policy publish calling M1-06.
6. Failure and job inspection.
7. Transactional log writes; public log read path in `read-api`.
8. `pubctl` commands, help text, and operator guide.

## Acceptance criteria

- On the `local` target with `localauth`, `pubctl` against the local `admin-api` tracks a catalog bill with a reason; the bill shows "selected by an administrator" among its reasons, its text is ingested, and the log shows the operator's public name and reason.
- **Cloud phase:** the same in `dev` with Google Cloud IAM, and a non-operator principal is refused.
- An action with an injected log failure leaves no change.
- Posting a roster with one operator added and one removed writes exactly two roster entries; posting it again writes none; an approver not on the prior roster is refused; the deploy principal calling any other route is refused. Checked locally with fake principals, and in the cloud phase in `dev`.
- Tracking a war powers resolution is refused with the policy reason.

## Open questions

- How does `bills track` lead to a pull of the bill's text? Locally, `bills track` sets `tracked` and the pull runs as its own command (`job-ingest --bill <id>`). Cloud phase: [architecture § Actions](../../../architecture/README.md#actions) enqueues a `worker` task, and `worker` arrives in M3. Shared with [M1-05](05-manual-bill-pull-and-storage.md).
- Cloud phase: is the manual pull an `admin-api` action, and so logged publicly like other privileged actions, or a job run outside `admin-api`? Shared with M1-05.
- When does the administrator phase rule end ([01](../../01-bill-ingestion-and-display.md#open-questions))? M1 only needs the `phase` field.
  - M1-12 reads only `phase`; `steward_signoff` answers `409` and changes nothing. The end of the phase is an operator decision.
- Re-parse beside the old version: parsed text is write-once per version and only `billxml/1` exists; a storage path for a second parser version is open (M1-03/M1-04 layout).
