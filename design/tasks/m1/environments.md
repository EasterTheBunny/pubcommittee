# M1 Environment Stages

How the development environment grows as M1 tasks complete: what runs on a local machine, where a cloud account first becomes necessary, and who does each step. M1 is built in two phases: a **local phase** in which every task is built and accepted with no cloud service, and a **cloud phase** that adds the cloud adapters, infrastructure, and delivery, and rechecks the hosted criteria. The **operator** is the person who owns the cloud accounts, credentials, and public attribution. An **agent** is a coding agent working in the repository on the operator's machine.

## Stages

| Stage | Runs on | Build target and adapters | Needs |
| --- | --- | --- | --- |
| **L0 — Local, offline** | Local machine | `memory`; `fixturesrc`, `fakegen`, `localkeys` | Go, Docker, Node, npm |
| **L1 — Local, filesystem stores** | Local machine, `compose.yaml` | `local`; `fsobject`, `fsdocs`, `localkeys`, `localauth`, OpenTelemetry Collector | L0 plus Docker. No emulators |
| **L2 — Local, live public sources** | Local machine with internet | `local`; `govinfo`, `uscode` | L1 plus outbound HTTPS. No account or key: govinfo and the Law Revision Counsel are public |
| **L3 — Local, live model** | Local machine | `local` with `adapters/claude` | An operator-supplied Anthropic API key for the first-party client, on the local machine only; never in CI. Costs money |
| **E1 — Emulated cloud stores** (cloud phase) | Local machine and CI | `emulator`; `gcs` and `firestore` against the Firestore emulator and the Cloud Storage stand-in from [M1-00](00-storage-emulator-exploration.md) | L1 plus emulator images |
| **C1 — Cloud `dev`** | Google Cloud `dev` project and a web host | `gcp` | Projects, billing, IaC state, shared registry, federated credentials for the private repository, the deploy trigger App, units deployed by [M1-17](17-continuous-delivery.md) |
| **C2 — Cloud `staging` and `prod`** | Same, per environment | `gcp` | C1 repeated from the same modules, plus promotion by digest behind approvals ([M1-17](17-continuous-delivery.md)) |

**The switch to cloud.** The local phase covers L0–L3 and needs no cloud account: live generation uses an Anthropic API key, not a cloud project. Every task except [M1-15](15-infrastructure-and-deploy.md) and [M1-17](17-continuous-delivery.md) has its local acceptance closed there, and the [M1-16](16-acceptance.md) local gate proves every M1 exit criterion on a local machine. The cloud phase starts with [M1-00](00-storage-emulator-exploration.md) and [M1-03b](03b-cloud-storage-adapters.md) at E1, then M1-15 and M1-17 at C1 and C2, and then rechecks each task's "cloud phase" criteria and the M1-16 hosted gate.

## Tasks by stage

| Task | Build and test | Acceptance needs | Why it cannot finish earlier |
| --- | --- | --- | --- |
| [M1-00](00-storage-emulator-exploration.md) Storage emulator exploration | E1 | E1 | Cloud phase: needed only for `adapters/gcs` |
| [M1-01](01-backend-foundation.md) Backend foundation | L0, L1 | L1 | — |
| [M1-02](02-api-contract.md) API contract | L0 | L0 | — |
| [M1-03a](03a-local-storage-adapters.md) Local storage adapters | L0, L1 | L1 | — |
| [M1-03b](03b-cloud-storage-adapters.md) Cloud storage adapters | E1 | E1, then **C1** (real-bucket tests) | Cloud phase |
| [M1-04](04-bill-xml-parser.md) Parser | L0 (corpus fetched once at L2) | L0 | — |
| [M1-05](05-manual-bill-pull-and-storage.md) Manual pull and storage | L0, L1, L2 | L2; cloud phase **C1** | Local: exit bills pulled into the local store. Cloud: "in `dev`" |
| [M1-06](06-selection-policy.md) Selection policy | L0, L1 | L1; cloud phase **C1** | Local: on the local store. Cloud: "in `dev`" |
| [M1-07](07-us-code-and-citations.md) US Code and citations | L0, L2 | L2 | Resolution rate is measured on local data |
| [M1-08](08-version-history-and-stale.md) Version comparison | L0 | L0 | — |
| [M1-09](09-ai-generation.md) Generation | L0 (`fakegen`), L3 | **L3**; cloud phase **C1** | Local: summaries written to the local store with the API key. Cloud: "in `dev`" through Claude on Google Cloud |
| [M1-10](10-read-api.md) `read-api` | L0, L1 | L1; cloud phase **C1** | Cloud: load smoke test through the API edge |
| [M1-11](11-exports-feed-machine-notice.md) Exports and feed | L0, L1 | L1; cloud phase **C1** | Cloud: "after a run in `dev`" |
| [M1-12](12-administration.md) Administration | L0, L1 with `localauth` | L1; cloud phase **C1** | Cloud: IAM invoker check and non-operator refusal |
| [M1-13](13-web-ui-real-data.md) Web UI | L0, L1 | L1; cloud phase **C1** | Cloud: hosted on Netlify |
| [M1-14](14-installable-app.md) Installable app | L1 | L1 (desktop install and offline on `localhost`); cloud phase **C2** | Cloud: mobile install needs HTTPS on a real host |
| [M1-15](15-infrastructure-and-deploy.md) Infrastructure | IaC written and validated locally | **C1**, **C2** | It creates them |
| [M1-16](16-acceptance.md) Acceptance | L1–L3 | Local gate L3; hosted gate **C2** | Local gate proves the exit criteria; hosted gate serves invited readers |
| [M1-17](17-continuous-delivery.md) Continuous delivery | Workflows written locally; public dispatch tested against a no-op private workflow | **C1**, **C2** | Deploys need the environments and registry from M1-15 |

## Walkthrough

**Local phase**

1. **L0.** Agent builds M1-01 and M1-02, then M1-04, M1-06, and M1-08 in parallel, all on the `memory` target with fixtures. `read-api` (M1-10) and the UI (M1-13) run against `memory`. The M0 demo is archived to `demo/` at the start of M1-13.
2. **L1.** Agent builds M1-03a and the `compose.yaml` data volume. Pulls (M1-05), exports (M1-11), and `admin-api` (M1-12, with `localauth`) run on the filesystem stores. The UI and service worker (M1-14) run against the local `read-api`.
3. **L2.** M1-04 fetches the corpus once from govinfo and commits it as example data in `fixtures/`, naming the exit bills. Later tasks use `fixturesrc` offline; `govinfo` and `uscode` are pointed at the live public sources only to prove the adapters and to measure citation resolution (M1-07).
4. **L3.** Operator supplies the Anthropic API key and approves spend. Agent runs `job-summarize` on the exit bills (M1-09). CI never reaches this stage: it runs `fakegen`, with live runs behind a switch that is off by default. The M1-16 local gate closes here.

**Cloud phase**

5. **E1.** Agent runs M1-00 to choose the Cloud Storage stand-in, then builds M1-03b: `gcs` and `firestore` pass the same conformance suites as the filesystem adapters.
6. **C1.** Operator bootstraps the cloud accounts. Agent writes the IaC in M1-15 and runs plans, and the operator approves applies. Operator creates the deploy trigger GitHub App and the private repository's environments; agent writes the M1-17 workflows. From then on every merge to `main` deploys to `dev` through the private repository. Operator runs the `pubctl` actions that need public attribution. The `dev` acceptance criteria of M1-05, 06, 09, 10, 11, and 12 are checked there.
7. **C2.** Same modules for `staging` and `prod`; M1-17 promotes the `dev` digests behind approvals. M1-14 mobile install and the M1-16 hosted gate run in `staging`.

## Operator and agent

| Step | Operator | Agent |
| --- | --- | --- |
| Install local tooling that needs system access (Go, Docker, Node) | Yes | Proposes the commands |
| Install project-pinned tools (Go tools, npm dev dependencies, Docker images) | — | Yes |
| Code, tests, fixtures, `compose.yaml`, OpenAPI spec, public IaC modules in `infra/` | Reviews | Yes |
| Run the local stack and reset its data directory | — | Yes |
| Deployment repository code at `deploy/` (environment roots, workflows) | Reviews; owns the repository | Yes, with placeholders where values are private |
| Fetch public sources (govinfo, US Code, bill DTDs) | — | Yes, within the politeness rules |
| Provide the Anthropic API key and approve spending runs (L3, local only), including grader runs | Yes | Runs once approved |
| Grade the operator sample (about 30 statements against their cited lines) once in M1 | Yes | Prepares the sample and reports agreement |
| Create billing, Google Cloud projects, and IaC state bucket; accept Claude on Google Cloud terms | Yes | Writes the bootstrap steps |
| Interactive sign-in (`gcloud auth login`, application default credentials, web host, GitHub) | Yes | — |
| Configure CI federated credentials, branch protection, secret scanning with push protection | Yes | Writes the workflow and IaC |
| Create the deploy trigger GitHub App, install it on the private repository only, store its id and key in the public `deploy-trigger` environment; create the private `dev`, `staging`, `prod` environments with reviewers | Yes | Writes the steps and the workflows ([M1-17](17-continuous-delivery.md)) |
| Approve `staging` and `prod` promotions | Yes | Starts them only when asked |
| `tofu plan` against an environment | Approves access | Yes |
| `tofu apply`, and any `prod` change | Yes, or explicitly approves each run | Runs only with approval |
| Private values: operator roster, project ids, environment values files | Yes, in the private deployment repository only | Never commits them to this repository; uses placeholders |
| Sign and land work on `main` (`git merge --squash m1-integration`, `git commit -S`) | Yes, with their own key | Commits unsigned on `task/*` and `m1-integration` only |
| `pubctl` actions (track, untrack, reparse, regenerate, publish policy) | Yes: the public log names the administrator who acted | Drafts the commands and reasons |
| Domain, DNS, and web host account | Yes | — |
| Open product decisions (licence, topic terms, exit bills, display policy) | Yes | Prepares options and evidence |

## Open questions

- **Cloud Storage stand-in** (cloud phase, E1): Google publishes no Cloud Storage emulator. Which stand-in does `adapters/gcs` test against? Answered by [M1-00](00-storage-emulator-exploration.md).
- **Firestore emulator** (cloud phase, E1): run in a container so CI and other machines match.
- **Tool choices** are recorded in [architecture § Tooling](../../../architecture/README.md#tooling): Go 1.27, Go tools through the `tool` directive, `openapi-typescript` in `app/`, cross-compiled multi-arch images, BuildKit SBOMs, GitHub Actions, OpenTofu, Playwright, the Firestore emulator in a container (cloud phase), the Firebase CLI in the deployment repository's CI only, and no Postgres until M2.
- **Answered:** the OpenAPI linter is Redocly CLI ([M1-02](02-api-contract.md)).
- **Still open:** DTD validation in Go tests or with `xmllint` in a CI container ([M1-04](04-bill-xml-parser.md)); a local pre-commit secret scanner beside CI scanning ([M1-15](15-infrastructure-and-deploy.md)).
