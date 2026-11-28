# M2 Environment Stages

How the development environment grows as M2 tasks complete. As in [M1](../m1/environments.md), M2 is built in two phases. The **local phase** builds every M2 feature and runs it on one machine with `scripts/local.sh`, so the operator can sign in as a test user and use the whole prototype before any cloud work. The **cloud phase** then adds the managed services, real sign-in providers, and the domain, and checks each task's cloud-phase criteria in `dev` and `staging`.

## Stages

| Stage | Runs on | Build target and adapters | Needs |
| --- | --- | --- | --- |
| **L0 — Local, offline** | Local machine | `memory`; M1's `fixturesrc`, `fakegen`, `localkeys`; memory write ports | Go, Docker, Node, npm |
| **L1 — Local stack** | Local machine, `compose.yaml` and `scripts/local.sh` | `local`; M1's filesystem stores plus `adapters/postgres` against a Postgres container, `localkeys` for HMAC and encryption, a stand-in identity provider with seeded test accounts, `write-api`, `job-batcher`, `job-notify` | L0 plus Docker. No provider account, no domain, no cloud |
| **L2 — Local with a browser push service** | Local machine with internet | L1 with `adapters/webpush` and a development VAPID key pair generated locally | A current browser on `localhost` (a secure context); the browser vendor's push service. No cloud account |
| **L3 — Local, live model** | Local machine | As in M1, only for tasks that regenerate platform content; M2 adds no model use | M1's operator-supplied API key; never in CI |
| **C1 — Cloud `dev`** | Google Cloud `dev` project | `gcp`; `adapters/kmsgcp`, managed serverless Postgres, Secret Manager, the domain, Google and Facebook sign-in | [M2-00](00-decisions-and-prerequisites.md) cloud decisions and operator steps; [M2-16](16-infrastructure.md) |
| **C2 — Cloud `staging` and `prod`** | Same, per environment | `gcp` | C1 repeated; promotion by signed tags ([M1-17b](../m1/17b-release-approval-gate.md)) |

**The switch to cloud.** The local phase covers L0–L2 and needs no cloud account, domain, or provider client: sign-in uses the stand-in identity provider, keys come from `localkeys`, and Postgres runs in a container. It closes with the [M2-18](18-acceptance.md) local gate, after which the operator reviews every feature on `scripts/local.sh up`. Only then does the cloud phase start.

## Local run

Every local-phase task that adds a service, job, or route also adds it to `compose.yaml` and `scripts/local.sh`, under resource limits, so `scripts/local.sh up` always shows what is built so far:

- `scripts/local.sh up` serves the app, `read-api`, `write-api`, `admin-api`, Postgres, and the collector; `seed` also creates the test accounts and roles (a tester, a steward, a moderator) on the stand-in provider.
- `scripts/local.sh job job-batcher` and `job job-notify` run one batch by hand; `up` can also run them on a short local interval.
- Sign-in on `localhost` goes to the stand-in provider's account picker; no real provider account is used.

## Tasks by stage

| Task | Build and test | Acceptance needs | Cloud-phase part |
| --- | --- | --- | --- |
| [M2-00](00-decisions-and-prerequisites.md) Decisions | — | Local decisions before the tasks they block | Cloud decisions and operator steps before C1 |
| [M2-01](01-write-ports-and-domain.md) Write ports | L0 | L0 | — |
| [M2-02](02-postgres-adapter.md) Postgres adapter | L0, L1 | L1 | Managed Postgres in `dev` |
| [M2-03](03-cloud-keys.md) Cloud key adapter | — | **C1** | All of it |
| [M2-04](04-api-contract-writes.md) API contract | L0 | L0 | — |
| [M2-05](05-write-api-sign-in-sessions.md) `write-api` | L0, L1 | L1 (stand-in provider) | Google and Facebook sign-in in `dev` |
| [M2-06](06-review-items.md) Review items | L0, L1 | L1 | In `dev` |
| [M2-07](07-signals-and-markup-requests.md) Signals | L0, L1 | L1 | In `dev` |
| [M2-08](08-batcher-and-published-counts.md) Batcher | L0, L1 | L1 | Scheduled in `dev` |
| [M2-09](09-notifications.md) Notifications | L0, L1, L2 | L2 (inbox at L1) | Device notification on an installed app in `dev` |
| [M2-10](10-moderation-and-steward-actions.md) Moderation and steward actions | L0, L1 | L1 | In `dev` |
| [M2-11](11-brief-page.md) Brief page | L0, L1 | L1 | In `dev` |
| [M2-12](12-share-links-and-cards.md) Share links and cards | L0, L1 | L1 (cards checked from the rendered HTML) | Cards rendered by a social platform from `dev` |
| [M2-13](13-injection-and-inference-defenses.md) Defenses | L0, L1 | L1 | Rechecked in the hosted gate |
| [M2-14](14-exports-and-feed.md) Exports and feed | L0, L1 | L1 | After a run in `dev` |
| [M2-15](15-web-ui-interactive.md) Web UI | L0, L1 | L1 | Google sign-in flow in `dev` |
| [M2-16](16-infrastructure.md) Infrastructure | IaC validated locally | **C1**, **C2** | All of it |
| [M2-17](17-testers-and-charter.md) Testers and charter | Materials and charter at L1 | Operator walkthrough at L1; tester sessions at **C2** | Sessions with invited testers in `staging` |
| [M2-18](18-acceptance.md) Acceptance | L1, L2 | Local gate L2 | Hosted gate **C2** |
