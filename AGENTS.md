# AGENTS.md

Instructions for coding agents working in this repository. People should start with [README.md](README.md).

## Where things are

| Need | Read |
| --- | --- |
| Why a feature exists, how it behaves | [PRODUCT_DOC.md](PRODUCT_DOC.md), then `design/NN-*.md` |
| How it is built: stack, ports, adapters, storage, API, tooling | [architecture/README.md](architecture/README.md) |
| What to build now | [design/tasks/README.md](design/tasks/README.md), the task file, [design/tasks/STATUS.md](design/tasks/STATUS.md) |
| Environment stages and what only the operator may do | [design/tasks/m1/environments.md](design/tasks/m1/environments.md) |

## Rules for every agent

- **Global principles** in [design/README.md](design/README.md#global-principles) are invariants.
- **Open questions:** a task-orchestrator answers the ones inside its task's scope from evidence and reports them; product, policy, legal, and spending decisions are never assumed. Workers answer none; they stop and report.
- **Nothing private.** Use placeholders (`<domain>`, `<project>`) and fake ids. Never read, copy, mount, or commit `deploy/` (the private deployment repository), `contacts/`, `.env*`, or `.local-data/`. One exception: the top-level agent may read `deploy/` (next rule).
- **No operator-only steps:** credentials, signing keys, paid live model runs, cloud environments (except through `pubops`, next rule), `tofu apply`, `pubctl` actions, pushes, pull requests.
- **Cloud commands are printed, not run.** Only the top-level agent may read `deploy/` and run the read-only `tofu init -lockfile=readonly`, `tofu validate`, `tofu plan`, `tofu show`, and `tofu fmt -check` there. No agent edits `deploy/`: changes to it are proposed as a unified diff, printed or saved outside both repositories, and the operator applies and commits them. No agent runs `tofu apply` or any other state-changing `tofu` command (`destroy`, `import`, `state`, `taint`, `untaint`, `force-unlock`, `workspace`), or any `gcloud` or other cloud CLI command: it prints each one, complete and ready to paste, for the operator to run. One exception: the top-level agent may call the tools of the operator's `pubops` MCP server (private, in the deployment repository at `mcp/pubops/`), which act as a least-privilege `<env>-ops-runner` service account per environment and take no free-form arguments. Its read tools (`executions`, `reports`, `generated_status`) need no approval. Every tool that changes an environment (`run_job`, `onboard_bill`, `summarize_live`, `finish_bill`) needs the operator's approval on that call, and a live model run (`summarize_live`, including its `explain-version` mode for one older version's inline explanations, and `finish_bill`) also needs a dry run (for `explain-version`, `run_job`'s `explain-version-dry-run`) and an approved cost estimate first; `finish_bill` submits at most the approved task count. Tools that change `prod` are refused unless the operator starts the server for it. Task-orchestrators and workers never call `pubops`. The tools are the server's list; a new tool or mode is a change to this rule. Task-orchestrators and workers never read `deploy/`. Nothing from `deploy/` or from plan output goes into this repository, a worktree, an image, or a subagent prompt.
- **Agent commits are unsigned and local.** Commit with `git -c commit.gpgsign=false commit`, only on `task/*` and `<milestone>-integration`; never on `main`, never `-S`, never change git config. The user lands work on `main` with their own signed squash commit.
- **Builds, tests, and lint run in Docker containers** with explicit CPU and memory limits (skill `tool-docker`), never on the host.
- **Strict lint:** Go code passes `golangci-lint` with the repository config (skill `tool-golangci-lint`).
- **Names follow the design-doc vocabulary** ([design/README.md § Shared vocabulary](design/README.md#shared-vocabulary)).
- **Keep existing comments** in files you edit.

## Running tasks with agents

Three levels, defined in `.claude/`: the orchestrator (skill `run-milestone`), task-orchestrators (agent `task-orchestrator`, skill `run-task`), and workers (agent `task-worker`, skill `do-subtask`). See [design/tasks/README.md § Status and running tasks with agents](design/tasks/README.md#status-and-running-tasks-with-agents).

Tooling skills, loaded by whichever level does the work:

| Skill | For |
| --- | --- |
| `tool-docker` | Containers: `docker exec` for workers; limits, labels, cleanup for task-orchestrators |
| `tool-go` | `cmd/`, `internal/`, `go.mod` |
| `tool-golangci-lint` | Any Go change |
| `tool-openapi` | `api/openapi.yaml` and generated code |
| `tool-bill-xml` | Bill and US Code XML: parser, ingest, citations, version comparison |
| `tool-sveltekit` | `app/` |
| `tool-playwright` | UI, no-script, offline, and installability tests |
| `tool-compose-acceptance` | `test/acceptance/<task>/` scripts and Compose under limits |
| `tool-opentofu` | `infra/` (cloud phase) |

## Directory `AGENTS.md` files

A directory may hold its own `AGENTS.md` with rules that apply to files under it. Before editing a file, read every `AGENTS.md` from the repository root down to that file's directory; the nearest one wins where they differ. Keep each one short (under 40 lines) and specific to its directory:

```markdown
# AGENTS.md — <directory>

Purpose: <one sentence>.
Skills: <tool skills to load for work here>.
Owns: <what lives here>. Does not own: <what belongs elsewhere>.

## Rules
- <rule that is specific to this directory, with its source doc>

## Commands
- <build / test / lint command, run with docker exec>

## Gotchas
- <things that broke before>
```

A task that creates a directory with rules of its own (for example `internal/adapters/`, `cmd/`, `test/acceptance/`) adds its `AGENTS.md` in the same change. Task-orchestrators may edit `AGENTS.md` files inside their task's directories; changes to this root file are proposals for the orchestrator.
