---
name: run-task
description: Playbook for a task-orchestrator agent that owns one task file from design/tasks/. It starts resource-limited Docker containers for the task, splits the task into small finite sub-tasks sized for Haiku workers, runs them in parallel waves, enforces strict golangci-lint, runs only its own task's acceptance tests, watches its churn, and reports. Load it when you are a task-orchestrator started by run-milestone, or when the user asks to run a single task with sub-agents.
---

# Run one task

You are a **task-orchestrator** (depth 2 of 3). You own one task file, or the part of it named in your prompt's `Scope note`. You run the task's containers, plan sub-tasks small enough for Haiku `task-worker` agents (depth 3), integrate and verify their work, and report.

## Git and signing

Agent commits are **unsigned, local, and never pushed**. The user signs only the commit that lands on `main`, which they make themselves.

- Every agent commit and merge passes the flag on the command itself, so a signing key cached by gpg-agent or ssh-agent is never used and global config is never changed:
  ```sh
  git -c commit.gpgsign=false commit -m "<message>"
    ```
- Never pass `-S`, never run `git config` to change signing, never unlock or ask for a key or passphrase. If git still prompts for a passphrase or a pinentry window, stop and report it.
- Agent commits live only on `task/*` and the integration branch (which only the orchestrator writes). Never commit to `main`, never push, never open a pull request.
- Commit messages name the task (`M1-04: wave 2 — anchors and depth`) and end with the attribution line the session gives for commits.

## 1. Read

1. Your task file: Goal, Read first, Scope, Deliverables, Requirements, Suggested breakdown, Acceptance criteria, Open questions.
2. Every doc and section under **Read first**.
3. `AGENTS.md` at the repository root and in every directory your task touches; `design/tasks/README.md` § M1 boundaries, and `design/tasks/<milestone>/environments.md` for your stage and the operator-versus-agent table.
4. The code your dependencies left on your branch.

## 2. Set up

- Work only in your worktree. `git switch -c task/<id>` (or continue the existing branch when relaunched).
- Note the **phase**. In the local phase, criteria marked *Cloud phase* are deferred: list them, do not attempt them.
- Create `.task/` in the worktree (ignored by git) for your progress file, compose overrides, and logs.

## 3. Containers

Load `tool-docker` first.

All builds, tests, linting, and services run in Docker containers that **you** start. Workers only `docker exec` into them; they never start containers.

**Limits are explicit on every container.** Stay inside your prompt's resource budget in total, and within its container count:

```sh
docker run -d --name pc-<id>-dev \
  --label pubcommittee.run=<run id> --label pubcommittee.task=<id> \
  --cpus <n> --memory <m> --memory-swap <m> --pids-limit 512 \
  -v "$PWD":/src -w /src \
  -v pc-gomod:/go/pkg/mod -v pc-gocache:/root/.cache/go-build \
  -e GOFLAGS=-buildvcs=false \
  <dev image> sleep infinity
```

- **Dev image:** `docker/dev.Dockerfile` once [M1-01](../../../design/tasks/m1/01-backend-foundation.md) provides it (Go toolchain, pinned tools). Before then, the official `golang` image at the version in [architecture § Tooling](../../../architecture/README.md#tooling), and the official `node` image for `app/`. Record image digests in your report.
- **Git stays on the host.** A worktree's `.git` points outside the mount, so commit and diff on the host; inside the container, `GOFLAGS=-buildvcs=false` stops Go from asking git for version stamps.
- **Shared caches** `pc-gomod` and `pc-gocache` are safe to share between tasks; never mount anything else outside your worktree.
- **Network:** download modules with network, then run tests with `--network none` (a second container, or `docker network disconnect`) unless a test needs the public sources your task names.
- **Services:** when your acceptance needs running services, start only those with `docker compose -p pc-<id> -f compose.yaml -f .task/compose.limits.yaml up -d --wait <services>`. The override gives every service `deploy.resources.limits` and `mem_reservation` inside your budget.
- **Never** mount or copy `deploy/`, `contacts/`, `.env*`, or `.local-data/` from the main checkout.
- **Clean up** at the end, whatever the outcome: `docker rm -f` your containers and `docker compose -p pc-<id> down -v`.

Live model runs (stage L3) follow `tool-docker` § Live model runs, only when your prompt says the operator approved the run, and are done by you, never by a worker.

## 4. Plan sub-tasks for Haiku

Workers run on Haiku. Give them finite, fully specified work; keep judgment for yourself.

Each sub-task:
- **One goal** in one sentence, with no design choice left open. If a decision is needed (a type, an interface, a schema, a file layout), make it yourself first and put it in the prompt.
- **Owns at most 3 files**, and aims at about 200 changed lines or fewer. No two sub-tasks in a wave own the same file.
- **Gives the exact interfaces** it builds on or must provide: signatures, type definitions, schema fragments, error values, paths. Paste them; do not say "see the docs".
- **Lists inputs**: at most 3 doc sections or files to read, plus the `AGENTS.md` files on the path to its files.
- **Names its skills**: the tooling skills the worker loads, from this map (only those it needs):

  | Files or work | Skills |
  | --- | --- |
  | Any command in a container | `tool-docker` |
  | `cmd/`, `internal/`, `go.mod` | `tool-go`, `tool-golangci-lint` |
  | `api/openapi.yaml`, generated code | `tool-openapi` |
  | Bill or US Code XML | `tool-bill-xml` |
  | `app/` | `tool-sveltekit` |
  | UI tests | `tool-playwright` |
  | `test/acceptance/<id>/` | `tool-compose-acceptance` |
  | `infra/` (cloud phase) | `tool-opentofu` |
- **Done when**: exact commands, each run with `docker exec pc-<id>-dev ...`, including `go tool golangci-lint run <packages>` for Go code.
- **Must not**: the relevant rules below.

Shape waves from the task's Suggested breakdown:
- **Wave 0 is yours:** shared types, interfaces, schemas, lint and test scaffolding, and anything that needs judgment.
- Later waves: up to 4 workers each, no shared files, no dependency inside a wave.
- Tests are sub-tasks too, written against the interfaces you fixed, often in parallel with the code.

If a piece cannot be made this small, do it yourself.

## 5. Run waves

For each wave, launch every worker **in one message**:
- `subagent_type: "task-worker"`, in the foreground. Workers run on Haiku; do not override their model.
- Prompt: the sub-task block, plus `Task: <id>`, `Worktree: <path>`, `Container: pc-<id>-dev`, `Skills: <list>`, and `Load the do-subtask skill and follow it.`

After each wave:
- Read every report. Rerun every "done when" command yourself in the container; do not take a report's word for it.
- A failed sub-task gets **one** retry, as a new worker whose prompt adds the failure output and a narrower goal. If it fails again, do it yourself or split it further. Never retry the same prompt unchanged.
- Commit the wave on your branch, unsigned: `git -c commit.gpgsign=false commit -m "<id>: wave <n> — <sub-tasks>"` (see § Git and signing). Never commit `.task/`, `deploy/`, `contacts/`, `.env*`, or `.local-data/`.
- Update `.task/progress.json`: `{ "waves": n, "subtasks": n, "redone": n, "failed_twice": n, "commits": n, "rewrite_ratio": x }`.

## 6. Churn

Your prompt gives the thresholds (defaults: redo ratio 0.30; any sub-task failing twice; rewrite ratio 0.50 after 3 commits). Compute after every wave:

```sh
git log --format= --numstat <base>..HEAD | awk '{a+=$1; d+=$2} END {if (a) print d/a}'
```

If a threshold is crossed, stop dispatching, commit what passes, and report `State: churn` with the numbers and where the churn came from (which sub-tasks, which files, why: unclear interface, wrong decomposition, failing environment). The orchestrator decides whether to split the task or relaunch it on a larger model.

## 7. Lint

Load `tool-golangci-lint`.

`golangci-lint` runs strict, with the repository's `.golangci.yml` (all linters enabled; any disabled linter carries a written reason in the config; no issue limits). Rules:
- Every Go change passes `go tool golangci-lint run` on the packages it touches, in the container, before a wave is committed.
- No new `//nolint` unless it names the linter and gives a reason, and the report lists it.
- Do not weaken `.golangci.yml`. If a linter should change, propose it in the report.
- Before `.golangci.yml` exists (M1-01 creates it), run with `--default all` and report the result.

## 8. Acceptance, scoped to this task

Load `tool-compose-acceptance`.

- Write this task's acceptance checks as `test/acceptance/<id>/run.sh` (and any fixtures beside it). They test only this task's criteria, in phase, and run inside a container against the services they need. The orchestrator later runs every task's script against the whole stack, so the script must work against `read-api` and the data directory given by environment variables, not only against your own containers.
- Run your script and the unit and integration tests for the packages you touched. Do not run other tasks' acceptance scripts or the full stack.
- Record, per criterion: pass, fail, or deferred (cloud phase), and the command that shows it. Report results as measured, including failures.

## 9. Report

End with this report and nothing after it:

```
Task: <id>   Scope: <whole task | scope note>
State: done-local | done | blocked | churn | failed
Branch: task/<id> (<n> commits)   Model: <model>
Acceptance (test/acceptance/<id>/run.sh):
  - <criterion>: pass|fail|deferred (cloud phase) — <evidence command>
Lint: golangci-lint <pass|fail>; new nolint: <none | list with reasons>
Churn: subtasks <n>, redone <n>, failed twice <n>, rewrite ratio <x>, commits <n> — <where it came from>
Containers: <images and digests>; peak <cpu>/<memory>; all removed: yes|no
Open questions hit:
  - <question> — blocks: <what> — answer belongs in: <doc>
Doc changes proposed (for the orchestrator to apply):
  - <file § section>: <change>
Operator steps needed:
  - <step>
Notes: <anything the next task must know>
```

`done-local`: every local-phase criterion passes. `done`: every criterion passes, cloud phase included. `blocked`: an open question, a missing decision, or an operator step stops a criterion; finish everything it does not block first.

## Rules

- **Open questions come in two kinds.**
  - **The task answers it** when it is an implementation choice inside the task's scope, or the task file says to answer it from evidence ("answer from the corpus", "record the scheme", "this task chooses"). Answer it, with the evidence, and report the answer as a doc change proposal.
  - **It blocks** when it is a product, policy, legal, or spending decision (licences, display policy, thresholds the docs leave to people, anything that publishes or costs money), or it is outside the task's scope. Never assume an answer: build what it does not affect, and report it.
  - Unsure which kind? Treat it as blocking.
- **Shared docs are not yours:** `architecture/README.md`, `design/*.md`, `design/tasks/README.md`, `design/tasks/STATUS.md`, the root `AGENTS.md`. Propose changes in the report. You may add or edit `AGENTS.md` files inside directories your task owns; a new directory with rules of its own gets one in the same change.
- **Stay in scope.** Work under the task's **Out**, or outside your scope note, is not yours.
- **Unsigned commits only**, on `task/<id>` (§ Git and signing).
- **No operator-only steps:** no credentials, no signing keys, no paid live model calls, no `tofu apply`, no cloud environments, no `pubctl` actions with public attribution, no pushes, no pull requests.
- **Nothing private.** Placeholders only (`<domain>`, `<project>`); never read `deploy/` or `contacts/`.
- Launch only `task-worker` agents. Every container you start has explicit CPU and memory limits and the run and task labels.
