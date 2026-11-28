# M2-02 — Postgres Adapter

**Phase:** local first (Postgres container), cloud phase for the managed database.

## Goal

`adapters/postgres` implements the private SQL ports from [M2-01](01-write-ports-and-domain.md), with versioned migrations, passing the same conformance suites as the memory adapter against a local Postgres container.

## Read first

- [architecture § SQL (private, small)](../../../architecture/README.md#sql-private-small)
- [architecture § Adapter packages](../../../architecture/README.md#adapter-packages) and [§ Local run](../../../architecture/README.md#local-run) (Postgres joins `compose.yaml` in M2)
- [09 § Privacy](../../09-integrity-moderation-and-privacy.md#privacy)

## Scope

**In**
- Tables: `accounts`, `identities`, `signals`, `my_signals`, `request_reasons`, `push_subscriptions`, `notify_state`, `snapshots`, `roles` (architecture § SQL), plus `sessions` (decided in [M2-00](00-decisions-and-prerequisites.md)), and a key-version column beside every HMAC value.
- Migrations as code, applied by a job or at start (choose and record); a down path for each.
- A Postgres service in `compose.yaml` under resource limits, for the `local` target.
- Conformance run against the container in CI.

**Out**
- Provisioning the managed database ([M2-16](16-infrastructure.md)).

## Requirements

- **Local run:** adds what it builds to `compose.yaml` and `scripts/local.sh` under resource limits, so `scripts/local.sh up` shows it ([environments § Local run](environments.md#local-run)).
- One signal per person per item enforced by the primary key on `signal_key`.
- No column holds a raw provider subject, email, IP address, or name from a provider (architecture § Personal data inventory).
- Only `write-api`, `job-batcher`, and `job-notify` link this adapter; `read-api` never does.
- Days, not timestamps, where the inventory says day (`created_on`, `created_day`).

## Acceptance criteria

- `adapters/postgres` passes every conformance suite for its ports against the Postgres container, locally and in CI.
- A concurrent double signal on one item yields exactly one row and one `ErrConflict`.
- A schema test fails if any column named in the inventory's "not stored" rows appears.
- **Cloud phase:** the adapter passes the same suites against the managed database in `dev`.

## Open questions

- Serverless Postgres vendor and resume latency ([M2-00](00-decisions-and-prerequisites.md)).
