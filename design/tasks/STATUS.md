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
| [M1-02](m1/02-api-contract.md) | local | done-local | `task/M1-02` | sonnet | none (1 sub-task, 0 redone, rewrite 0, 1 commit) | Run r20261122-1412, batch 2; merged. Live contract test (`CONTRACT_BASE_URL`) carried to M1-10, which adds the routes. Its `run.sh` needs `api/` and `app/`, so it runs in the dev container, not the compose acceptance service. v1 overlap period (12 months) in `api/MACHINE-API.md` is proposed, awaiting operator |
| [M1-03a](m1/03a-local-storage-adapters.md) | local | done-local | `task/M1-03a` | sonnet | none (1 sub-task, 0 redone, rewrite 0, 1 commit) | Run r20261122-1412, batch 2 (relaunched after worktree base fix); merged. Cross-container job visibility over HTTP carried to M1-05/M1-10 (no job or data route yet). Catalog filters and search are not on the port: M1-05 or M1-10. Agent ran one `go test` on the host (process deviation) |
| [M1-03b](m1/03b-cloud-storage-adapters.md) | cloud | todo | | | | |
| [M1-04](m1/04-bill-xml-parser.md) | local | done-local | `task/M1-04` | sonnet | none (2 sub-tasks, 0 redone, rewrite 0, 2 commits) | Run r20261122-1412, batch 2 (relaunched after worktree base fix); merged. Real-bill formula criterion deferred by the operator: no `<formula>` in the 119-1 House archive |
| [M1-05](m1/05-manual-bill-pull-and-storage.md) | local, cloud | done-local | `task/M1-05` | sonnet | none (4 sub-tasks, 0 redone, 1 commit) | Run r20261122-1412, batch 4; merged (mechanical conflicts with M1-08 resolved). Live govinfo pull of the 3 exit bills byte-identical to fixtures; 200-entry catalog sample in `fixtures/catalog/`. Cloud-phase criteria open |
| [M1-06](m1/06-selection-policy.md) | local, cloud | in-progress | `task/M1-06` | sonnet | | Run r20261122-1412, batch 5, worktree `.claude/worktrees/m1-06` |
| [M1-07](m1/07-us-code-and-citations.md) | local | in-progress | `task/M1-07` | sonnet | | Run r20261122-1412, batch 4, worktree `.claude/worktrees/m1-07`. Part 1: adapter, provision store, extraction, resolution, read port; job-ingest step after M1-05 |
| [M1-08](m1/08-version-history-and-stale.md) | local | in-progress | `task/M1-08` | sonnet | none (part 1: 0 workers, 1 commit, rewrite 0) | Run r20261122-1412. Part 1 merged (library `internal/versiondiff`, storage, read port, compare route). Part 2 after M1-05: run `versiondiff.Step` from job-ingest over tracked bills; additive `changed_since` in api/openapi.yaml for old line addresses; 50-anchor manual check accepted as met by the operator (2026-11-22): fixtures have 18-45 lines per version, the one differing pair (hr-4313 ih to rh) was read in full |
| [M1-09](m1/09-ai-generation.md) | local, cloud | todo | | | | Live run needs the operator's API key and spend approval |
| [M1-10](m1/10-read-api.md) | local, cloud | done-local | `task/M1-10` | sonnet | none (2 sub-tasks, 0 redone, 1 commit; rewrite 1.80 from regenerating `api.gen.go` without client, below the 3-commit rule) | Run r20261122-1412, batch 3; merged. Summary, changes, compare, citations, feed, exports need read ports from M1-07/08/09/11. Cloud-phase criteria open |
| [M1-11](m1/11-exports-feed-machine-notice.md) | local, cloud | todo | | | | |
| [M1-12](m1/12-administration.md) | local, cloud | todo | | | | |
| [M1-13](m1/13-web-ui-real-data.md) | local, cloud | in-progress | `task/M1-13` | sonnet | | Run r20261122-1412, batch 4, worktree `.claude/worktrees/m1-13` |
| [M1-14](m1/14-installable-app.md) | local, cloud | todo | | | | |
| [M1-15](m1/15-infrastructure-and-deploy.md) | cloud | todo | | | | |
| [M1-16](m1/16-acceptance.md) | local, cloud | todo | | | | Local gate, then hosted gate |
| [M1-17](m1/17-continuous-delivery.md) | cloud | todo | | | | |
