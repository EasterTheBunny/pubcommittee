# M1-17 — Continuous Delivery

## Goal

Deliver every merged change on the public repository's `main` to `dev` with no manual steps, and promote the same images to `staging` and `prod` behind approvals. The deploy runs only in the private deployment repository, so no cloud credential, plan output, or environment value ever appears in the public repository or its public run logs. The public repository's only part is to send one signed trigger naming the commit. **Phase:** cloud. Start once [M1-15](15-infrastructure-and-deploy.md) has a `dev` environment: the first deliverable is the placeholder `read-api` reaching `dev`, and every task's cloud-phase criterion depends on it.

## Read first

- [architecture § Environments and delivery](../../../architecture/README.md#environments-and-delivery), especially [§ Public modules, private deployment](../../../architecture/README.md#public-modules-private-deployment) and [§ Continuous delivery](../../../architecture/README.md#continuous-delivery)
- [architecture § What stays private](../../../architecture/README.md#what-stays-private)
- [architecture § Service packaging](../../../architecture/README.md#service-packaging) (units, contract for every image, build targets)
- [M1-15](15-infrastructure-and-deploy.md) (environments, registry, service accounts this task deploys into)
- [environments](environments.md) (stages C1 and C2; operator and agent split)
- The private deployment repository's `README.md` (checked out at `deploy/`)
- GitHub documentation for `repository_dispatch`, `workflow_run`, GitHub Apps installation tokens, environments with required reviewers, and OpenID Connect to Google Cloud

## Scope

**In**
- **Trigger identity.** A GitHub App installed on the private deployment repository only, with the single permission `Contents: write` that `repository_dispatch` requires. Its id and private key are secrets of a `deploy-trigger` environment in the public repository, limited to `main` and release tags.
- **Public dispatch workflow** (`.github/workflows/dispatch-deploy.yml` in this repository):
  - runs on `workflow_run` of the backend CI workflow, only when that run succeeded, was a `push`, and was on `main`; and on push of a release tag (`v*`);
  - mints a short-lived installation token with `actions/create-github-app-token`, scoped to the private repository;
  - sends `repository_dispatch` with event type `deploy` and payload `{ "sha": "<commit>", "kind": "main" | "release", "tag": "<tag or empty>" }`;
  - prints nothing but the commit and kind.
- **Private workflows** (in the deployment repository):
  - `deploy-dev.yml`: on `repository_dispatch` of kind `main`, or manually with a commit. Verify the commit, run the leak check, build and push changed units, apply `dev` with their digests, smoke test, and record the deploy.
  - `promote.yml`: manually, or on `repository_dispatch` of kind `release`. Promote a recorded release's digests to `staging`, then `prod`, each behind its environment's approval.
  - `rollback`: `promote.yml` (or `deploy-dev.yml`) run with an earlier recorded commit; no rebuild.
- **Commit verification.** The private side refuses a payload whose commit is not on the public `main` (`git merge-base --is-ancestor`), or, for `release`, whose tag does not point at that commit. A dispatch alone never decides what is deployed.
- **Build.** For each unit, `docker/Dockerfile` with `SERVICE=<unit>` and `TARGET=gcp`, `linux/amd64` and `linux/arm64`, SBOM attestation, pushed to the shared Artifact Registry created by [M1-15](15-infrastructure-and-deploy.md). Units whose inputs did not change since the last recorded deploy are not rebuilt.
- **Deploy by digest.** Environments are applied with image digests, never tags. `staging` and `prod` receive exactly the digests `dev` ran.
- **Deploy record.** Each successful deploy writes `<env>.json` (`commit`, `kind`, `tag`, per-unit digests, time, run id) to a `deployed` branch of the private repository. That branch is the pin: it says what each environment runs, and rollback reads it.
- **Roster step.** When a change touches `operators/operators.yaml`, the apply workflow posts the public part of the roster (ids, names), the approving operator id, and a reason to `admin-api`'s roster route in each environment, and applies IAM only after the log write succeeds. The approver is taken from the merged pull request, never from the file: an `APPROVED` review on its final commit by a GitHub login that maps to an operator on the roster before the change. The reason is the pull request title. A roster change on `main` without such a pull request is refused ([architecture § Roster log](../../../architecture/README.md#roster-log)).
- **Smoke tests** after each apply: `/healthz` and `/readyz` on each HTTP unit through its normal route, one `read-api` route through the API edge, and a `--dry-run` execution of each job.
- **GitHub environments** in the private repository: `dev` (no approval), `staging` and `prod` (required reviewers, self-review prevented once a second operator exists), each holding the workload identity provider and that environment's deploy service account as variables.
- **Concurrency:** one deploy per environment at a time; a newer `dev` dispatch queues behind a running one rather than cancelling an apply.
- **Units in M1:** `read-api`, `admin-api`, `job-ingest`, `job-summarize`, `job-exports`, and the existing-law ingest unit once [M1-07](07-us-code-and-citations.md) settles whether it is its own image. Each is added to the deploy list when its task delivers an image; the placeholder `read-api` comes first.
- **Secret handling rules** for later milestones: infrastructure creates Secret Manager secrets without values; values are added outside the IaC tool, so they never reach state or logs.

**Out**
- Infrastructure modules, environments, registry, and IAM roles: [M1-15](15-infrastructure-and-deploy.md).
- Public CI (build, test, contract, dependency checks): [M1-01](01-backend-foundation.md), [M1-02](02-api-contract.md), [M1-13](13-web-ui-real-data.md).
- Secret values for M2 (VAPID, HMAC keys).
- UI deploy, unless the open question below moves it into CD.

## Deliverables

- GitHub App, installed on the private repository only; `deploy-trigger` environment in the public repository
- `.github/workflows/dispatch-deploy.yml` in this repository
- `deploy-dev.yml`, `promote.yml`, the `deployed` branch format, and smoke tests in the private repository
- Unit list (which units deploy, and how a new unit is added)
- Operator runbook in the private repository: set up the App and environments, approve a promotion, roll back

## Requirements

- **No cloud credentials in the public repository**, and no public workflow that can reach an environment ([architecture § What stays private](../../../architecture/README.md#what-stays-private)). The App key is the only deploy-related secret there, and it can only send a dispatch.
- **Private logs only.** Build, push, plan, apply, and smoke-test output appear only in the private repository's runs.
- **Least privilege.** The App has one permission on one repository. Deploy service accounts are per environment and federated ([M1-15](15-infrastructure-and-deploy.md)); no stored keys.
- **Untrusted payload.** The commit in a dispatch is verified against the public repository before anything is built.
- **Same bytes everywhere.** Promotion moves digests; nothing is rebuilt for `staging` or `prod`.
- **Each unit deploys independently** ([architecture § Units](../../../architecture/README.md#units)); one unit's failure does not redeploy the others.
- **Leak check before every deploy** ([architecture § What stays private](../../../architecture/README.md#what-stays-private)).
- **Secret values never in IaC state, the repositories, or logs.**
- Every deploy is recorded with commit and digests; nothing runs in an environment without a record.

## Suggested breakdown

1. Operator creates the GitHub App, installs it on the private repository only, and adds its id and key to the public `deploy-trigger` environment. Operator creates the private repository's `dev`, `staging`, and `prod` environments with variables and reviewers.
2. Public `dispatch-deploy.yml`, tested first against a no-op private workflow that only echoes the verified commit.
3. Commit verification and leak check in the private repository.
4. `deploy-dev.yml` for the placeholder `read-api`: build, push, apply by digest, smoke test, record. Needs the `dev` environment and registry from [M1-15](15-infrastructure-and-deploy.md).
5. `promote.yml` for `staging` and `prod`, reading the `deployed` branch.
6. Rollback drill in `dev` and `staging`.
7. Change detection so unchanged units are not rebuilt.
8. Add each unit as its task delivers an image; add job dry-run smoke tests.
9. Settle the UI deploy question; add it here if CD takes it.

## Acceptance criteria

- A merge to the public `main` deploys the changed units to `dev` with no manual step, and the public run shows only the commit and kind.
- A dispatch naming a commit not on the public `main` is refused before any build.
- `staging` and `prod` run the same digests as the `dev` deploy they were promoted from (checked against the `deployed` records).
- A `prod` promotion waits for an approval in the `prod` environment.
- A rollback to the previous record restores its digests without a rebuild.
- The public repository holds no cloud credential and no workflow with cloud permissions.
- A leak check failure stops the deploy.
- A roster change produces its public log entries before the IAM change, and a failed log write leaves IAM unchanged.
- A roster change approved only by a login not on the prior roster, or pushed without a pull request, is refused before any log write.

## Open questions

- Does the UI deploy through CD, or through Netlify's own build from `app/` with per-environment API base URLs ([M1-13](13-web-ui-real-data.md))? The archived demo stays on Netlify's own build from `demo/` either way.
- Does a release tag start promotion automatically (still gated by approvals), or is every promotion started by hand?
- Who approves `staging` and `prod` while there is one operator? GitHub can prevent self-review only once a second reviewer exists.
- Which project holds the shared Artifact Registry, and how do each environment's Cloud Run service agents get read access ([M1-15](15-infrastructure-and-deploy.md))?
- In what order are Firestore indexes and images applied when a change needs both ([M1-03b](03b-cloud-storage-adapters.md))?
- Is a post-deploy check of the public `read-api` contract ([M1-02](02-api-contract.md)) part of the smoke tests in `dev`?
