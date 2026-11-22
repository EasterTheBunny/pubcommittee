# Task Status

State of every task, kept by the orchestrator (the `run-milestone` skill) and by people. Task-orchestrators never edit this file; they report, and the orchestrator records.

States: `todo`, `in-progress`, `blocked`, `churn`, `failed`, `done-local` (every local-phase criterion passes), `done` (every criterion passes, cloud phase included). Dependencies count as met at `done-local` in the local phase and at `done` in the cloud phase, once merged into the milestone's integration branch.

Model is the task-orchestrator's model (Sonnet by default; workers always use Haiku). Churn records each threshold crossed (redo ratio, repeated failures, rewrite ratio) and the action taken: split, scale up, or stop ([run-milestone § Churn](../../.claude/skills/run-milestone/SKILL.md)).

## M1

Integration branch: `m1-integration`.

| Task | Phase | State | Branch | Model | Churn | Notes |
| --- | --- | --- | --- | --- | --- | --- |
| [M1-00](m1/00-storage-emulator-exploration.md) | cloud | todo | | | | |
| [M1-01](m1/01-backend-foundation.md) | local | done-local | `task/M1-01` | sonnet | none (4 sub-tasks, 0 redone, rewrite 0, 1 commit) | Run r20261122-1412; merged into `m1-integration`. `compose.yaml` builds `read-api` with `TARGET=memory`; M1-03a switches it to `local` |
| [M1-02](m1/02-api-contract.md) | local | in-progress | `task/M1-02` | sonnet | | Run r20261122-1412, batch 2 |
| [M1-03a](m1/03a-local-storage-adapters.md) | local | in-progress | `task/M1-03a` | sonnet | | Run r20261122-1412, batch 2. Relaunched: first worktree was cut from `main`; branch fast-forwarded to `m1-integration` 6395c54 by the orchestrator with user approval, worktree `.claude/worktrees/m1-03a` |
| [M1-03b](m1/03b-cloud-storage-adapters.md) | cloud | todo | | | | |
| [M1-04](m1/04-bill-xml-parser.md) | local | in-progress | `task/M1-04` | sonnet | | Run r20261122-1412, batch 2. Relaunched: first worktree was cut from `main`; branch fast-forwarded to `m1-integration` 6395c54 by the orchestrator with user approval, worktree `.claude/worktrees/m1-04` |
| [M1-05](m1/05-manual-bill-pull-and-storage.md) | local, cloud | todo | | | | |
| [M1-06](m1/06-selection-policy.md) | local, cloud | todo | | | | |
| [M1-07](m1/07-us-code-and-citations.md) | local | todo | | | | |
| [M1-08](m1/08-version-history-and-stale.md) | local | todo | | | | |
| [M1-09](m1/09-ai-generation.md) | local, cloud | todo | | | | Live run needs the operator's API key and spend approval |
| [M1-10](m1/10-read-api.md) | local, cloud | todo | | | | |
| [M1-11](m1/11-exports-feed-machine-notice.md) | local, cloud | todo | | | | |
| [M1-12](m1/12-administration.md) | local, cloud | todo | | | | |
| [M1-13](m1/13-web-ui-real-data.md) | local, cloud | todo | | | | |
| [M1-14](m1/14-installable-app.md) | local, cloud | todo | | | | |
| [M1-15](m1/15-infrastructure-and-deploy.md) | cloud | todo | | | | |
| [M1-16](m1/16-acceptance.md) | local, cloud | todo | | | | Local gate, then hosted gate |
| [M1-17](m1/17-continuous-delivery.md) | cloud | todo | | | | |
