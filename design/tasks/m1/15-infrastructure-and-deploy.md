# M1-15 — Infrastructure

## Goal

**Phase:** cloud. Stand up `dev`, `staging`, and `prod` on Google Cloud with infrastructure as code, scale-to-zero throughout, ready for [M1-17](17-continuous-delivery.md) to deploy every M1 unit with short-lived federated credentials. Public, reusable modules live in this repository's `infra/`; environment roots, values, state configuration, and deploy workflows live in a private deployment repository ([architecture § Public modules, private deployment](../../../architecture/README.md#public-modules-private-deployment)). Nothing environment-specific or secret is committed to this repository.

## Read first

- [architecture § Hosting](../../../architecture/README.md#hosting) (including fixed-cost traps)
- [architecture § Environments and delivery](../../../architecture/README.md#environments-and-delivery), especially [§ Public modules, private deployment](../../../architecture/README.md#public-modules-private-deployment)
- [architecture § What stays private](../../../architecture/README.md#what-stays-private)
- [architecture § Service packaging](../../../architecture/README.md#service-packaging) and [§ Observability § Collection](../../../architecture/README.md#collection)
- [architecture § Administration § Service and access](../../../architecture/README.md#service-and-access)
- [M1-17](17-continuous-delivery.md) (what the deploy workflows need from this task)

## Scope

**In**
- **This repository:** `infra/modules/gcp/` reusable OpenTofu modules ([architecture § Tooling](../../../architecture/README.md#tooling)) with every real value as an input; `infra/examples/` placeholder root; CI that validates the modules, plans the example against fake values, and builds and tests images without pushing or deploying.
- **Private deployment repository** (skeleton exists; checked out at the ignored `deploy/` path): bootstrap root for the state bucket, workload identity pool, and deploy service accounts; environment roots for `dev`, `staging`, `prod` pinning module tags; values; operator roster; `plan` and `apply` workflows for infrastructure changes; the leak check that scans this repository for real values. Image build and deploy workflows belong to [M1-17](17-continuous-delivery.md).
- One shared Artifact Registry that all environments pull from, so [M1-17](17-continuous-delivery.md) promotes images by digest, with read access for each environment's Cloud Run service agents.
- Per environment: Google Cloud project, Cloud Storage buckets, Firestore (native mode) with indexes from [M1-03b](03b-cloud-storage-adapters.md), Cloud Run services (`read-api`, `admin-api`), Cloud Run jobs (`job-ingest`, `job-summarize`, `job-exports`, existing-law ingest); which of them get Cloud Scheduler triggers in M1 is an open question, since bill pulls are manual, Firebase Hosting as the API edge for `read-api`, service accounts per unit with least privilege, managed Prometheus sidecar for HTTP services and OTLP for jobs.
- Claude on Google Cloud enabled for the summarize job's service account (M1-09).
- `admin-api` IAM invoker bindings and operator list from the operator roster (`operators/operators.yaml`) in the deployment repository; the deploy service account gets invoker access for the roster route only.
- Deploy service accounts per environment with the roles [M1-17](17-continuous-delivery.md) needs (deploy Cloud Run services and jobs, push to the registry), federated to the private repository only. This repository's CI holds no cloud credentials.
- Web UI host environments per stage, configured only with API base URLs; whether CD or the host's own build deploys the UI is open in [M1-17](17-continuous-delivery.md).
- Secret scanning with push protection here, and a format-based pattern check that fails on `run.app` URLs, service-account emails, and project-id-shaped values in tracked files. The deployment repository scans this repository for its real values.
- Log retention set to days for backend request logs; client IPs stripped or truncated; web host log and analytics settings reviewed against the personal data inventory.

**Out**
- SQL, KMS keys, Cloud Tasks, Secret Manager entries for VAPID: M2+.
- Custom domain is needed before M2 sign-in; M1 may run on provider domains if `<domain>` is not chosen yet.

## Deliverables

- `infra/modules/gcp/`, `infra/examples/`, and module CI in this repository
- Deployment repository filled in from its skeleton: bootstrap, environment roots, values, operator README (bootstrap, apply, rollback)
- Secret and pattern scanning in CI
- Cost check showing idle cost near zero

## Requirements

- **No fixed-cost resources:** no load balancers, NAT gateways, minimum instances, or always-on databases.
- `read-api` service account has no SQL or key permissions (none exist in M1; enforce in IAM anyway).
- `admin-api` not routed through the API edge; only operator principals can invoke.
- Placeholders in docs; no real values in this repository. Modules have no real-value defaults, no hard-coded network specifics or defense tuning, and no sensitive outputs.
- The deployment repository pins module tags; no floating refs. The deployed application commit and digests are recorded by [M1-17](17-continuous-delivery.md).
- Each unit is its own Cloud Run service or job, so it deploys independently.

## Suggested breakdown

1. Fill in the deployment repository's bootstrap root (state bucket, workload identity pool, deploy service accounts).
2. Public modules: Cloud Run service, Cloud Run job plus scheduler, bucket, service account; example root and module CI; first tag.
3. `dev` environment and shared registry, ready for [M1-17](17-continuous-delivery.md) to deploy the placeholder `read-api`.
4. Firestore, buckets, indexes; Firebase Hosting rewrites and cache behavior.
5. Jobs and schedules; Claude on Google Cloud access.
6. `admin-api` with operator bindings.
7. Deploy service account roles for [M1-17](17-continuous-delivery.md); UI host environments.
8. Scanning and log retention; `staging` and `prod` from the same modules.

## Acceptance criteria

- `dev` and `staging` are created from the deployment repository alone, using tagged modules from this repository; `prod` applies cleanly.
- `infra/examples/` validates and plans in this repository's CI with fake values and no cloud credentials.
- Each environment's deploy service account can be assumed only from the private repository, with federated credentials and no stored keys.
- With no traffic for a day, billed compute is near zero.
- This repository's CI fails on a planted fake secret and on a planted project-id-shaped value; the deployment repository's scan fails on a planted real value.

## Open questions

- Which jobs run on a schedule in M1, and at what cadence, when bill pulls are manual ([M1-05](05-manual-bill-pull-and-storage.md))?
- Is `<domain>` chosen, or does M1 run on provider domains?
- What scrape interval and histogram buckets keep metrics cost down?
- Which project holds the shared registry (shared with [M1-17](17-continuous-delivery.md))?
- Are budget alerts wanted per project? The architecture does not mention them.
- Is a local pre-commit secret scanner wanted in addition to CI scanning?
