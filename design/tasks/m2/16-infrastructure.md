# M2-16 — Infrastructure for Writes (cloud phase)

**Phase:** cloud.

## Goal

`dev`, `staging`, and `prod` gain what M2 needs, from the public modules and the private deployment repository: the domain, `write-api`, serverless Postgres, KMS keys, secrets, and schedules for `job-batcher` and `job-notify`, all scaling to zero.

## Read first

- [architecture § Hosting](../../../architecture/README.md#hosting), [§ Fixed-cost traps to avoid](../../../architecture/README.md#fixed-cost-traps-to-avoid), [§ Environments and delivery](../../../architecture/README.md#environments-and-delivery)
- M1-15 and M1-17 (modules, deployment repository, promotion by signed tags since M1-17b)
- [M2-00](00-decisions-and-prerequisites.md) (domain, Postgres vendor, edge rate limits)

## Scope

**In (public modules, `infra/`)**
- Modules or module changes for: serverless Postgres (vendor from M2-00), KMS key ring and keys, Secret Manager secrets (OAuth client secrets, VAPID private key), `write-api` service, `job-batcher` and `job-notify` jobs with Cloud Scheduler, custom domain on the hosting edge for `<domain>` and `api.<domain>`.
- `infra/examples/` plans with fake values and no credentials.

**In (deployment repository, proposed as diffs)**
- Each environment's units, IAM per unit (least privilege, as in [M2-03](03-cloud-keys.md)), schedules, and the domain.
- Per-reader rate limits at the edge, if [M2-00](00-decisions-and-prerequisites.md) brings them into M2.

**Out**
- Agents never run `tofu apply`, `gcloud`, or cloud CLIs; the operator does.

## Requirements

- No fixed-cost resource unless the operator approves it (for example a load balancer for Cloud Armor).
- `read-api` gets no SQL, KMS, or secret access.
- Writes are never cached at the edge; credentialed responses never cache.

## Acceptance criteria

- `infra/examples/` validates and plans in CI with fake values.
- `dev` serves the app at `<domain>` and `write-api` at `api.<domain>` over TLS; Postgres and KMS reachable only from the units that need them.
- With no traffic for a day, billed compute stays near zero.

## Open questions

- Postgres vendor, domain, edge rate limits ([M2-00](00-decisions-and-prerequisites.md)).
