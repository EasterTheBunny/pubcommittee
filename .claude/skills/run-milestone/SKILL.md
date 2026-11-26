---
name: run-milestone
description: Run ready tasks from a milestone's task list in parallel, one task-orchestrator sub-agent per task, each in its own git worktree and its own resource-limited Docker containers; watch host resources and churn; run the whole application as a Docker Compose unit for acceptance. Use only when the user asks to run milestone tasks (for example "/run-milestone m1", "run the next batch of M1 tasks", "run M1-04 and M1-08").
disable-model-invocation: true
---

# Run milestone tasks

You are the **orchestrator** (depth 1 of 3). You pick tasks, give each a resource budget and a model, launch one `task-orchestrator` agent per task (depth 2), watch resources and churn, integrate the results, and run milestone acceptance on the whole application. Each task-orchestrator splits its task into small sub-tasks for `task-worker` agents (depth 3). Workers cannot spawn agents.

```
orchestrator (this session)   budgets, churn, integration, compose acceptance
  └─ task-orchestrator         one per task; own worktree; Sonnet by default; owns its containers
       └─ task-worker          one per sub-task; Haiku; runs commands in its task's containers
```

Arguments: a milestone (`m1`); optionally task ids (`M1-04 M1-08`); `--max N` (parallel tasks, default 3); `--phase local|cloud` (default `local`); `--accept` (run compose acceptance only).

## Git and signing

Agent commits are **unsigned, local, and never pushed**. The user signs only the commit that lands on `main`, which they make themselves.

- Every agent commit and merge passes the flag on the command itself, so a signing key cached by gpg-agent or ssh-agent is never used and global config is never changed:
  ```sh
  git -c commit.gpgsign=false commit -m "<message>"
  git -c commit.gpgsign=false merge --no-ff task/<id>
  ```
- Never pass `-S`, never run `git config` to change signing, never unlock or ask for a key or passphrase. If git still prompts for a passphrase or a pinentry window, stop and report it.
- Agent commits live only on `task/*` and `<milestone>-integration`. Never commit to `main`, never push, never open a pull request.
- Commit messages name the task (`M1-04: wave 2 — anchors and depth`) and end with the attribution line the session gives for commits.

## 1. Read the plan

1. `design/tasks/README.md`: task table (dependencies), phases, order graph, boundaries.
2. `design/tasks/STATUS.md`: state, model, and churn of every task.
3. `design/tasks/<milestone>/environments.md`: stages and the operator-versus-agent table.

Do not read every task file yourself; task-orchestrators read their own.

## 2. Preflight

Stop and tell the user, in full sentences, if any of these hold:
- **Uncommitted changes under `design/`, `architecture/`, `app/`, `cmd/`, `internal/`, `api/`, `infra/`, `test/`, `docker/`, or `.claude/`.** Worktrees start from a commit. Ask the user to commit; do not commit for them unless they say so.
- **No integration branch.** Work integrates on `<milestone>-integration`, never `main`. Ask before creating it from `main`.
- **Docker is not usable** (`docker info` fails) or Compose v2 is missing (`docker compose version`).
- **`deploy/`, `contacts/`, `.env*`, `.local-data/` could enter a worktree, a prompt, an image, or a commit.** Never copy them. You, the top-level agent, may read `deploy/` and run read-only `tofu init -lockfile=readonly`, `tofu validate`, `tofu plan`, `tofu show`, `tofu fmt -check` there; never edit it, propose changes as a unified diff for the operator; never `tofu apply`, another state-changing `tofu` command, or `gcloud`: print those for the operator. The one exception is the operator's `pubops` MCP tools: read tools freely, the others only with the operator's approval per call, and a live model run only after an approved estimate ([AGENTS.md](../../../AGENTS.md)). Never read `contacts/`, `.env*`, or `.local-data/`, and never pass `deploy/` content or plan output to a task-orchestrator.

Then check out the integration branch, so every worktree starts from it. Pick a run id (`r<yyyymmdd-hhmm>`); every container in this run carries the label `pubcommittee.run=<run id>`.

## 3. Resource budget

Measure the host and set the budget before every batch:

```sh
nproc                                                     # CPUs
awk '/MemAvailable/ {print int($2/1024)}' /proc/meminfo   # available MiB
docker stats --no-stream --format '{{.Name}} {{.CPUPerc}} {{.MemUsage}}'
```

- **Reserve for the host:** the larger of 2 CPUs and 25% of CPUs, and the larger of 4 GiB and 25% of available memory.
- **Budget** = what remains, minus what containers already running (any label) use.
- **Per task, by default:** 2 CPUs and 4 GiB in total across all of that task's containers, at most 3 containers at once. Give more only to a task that runs the compose stack or a large build (for example M1-04 corpus tests, M1-13 UI tests): up to 4 CPUs and 6 GiB.
- **Parallel tasks** = the smallest of `--max`, the number of ready tasks, and how many per-task budgets fit in the budget.

Pass each task its budget in the launch prompt. While tasks run, check usage on every notification:

```sh
docker stats --no-stream --filter label=pubcommittee.run=<run id> \
  --format '{{.Name}} {{.CPUPerc}} {{.MemUsage}} {{.MemPerc}}'
```

- Above 85% of the budget's memory, or host `MemAvailable` below the reserve: launch nothing new. If it persists, lower the largest task's limits with `docker update --cpus <n> --memory <m> --memory-swap <m> <container>`, or `docker pause` that task's containers, and tell the user which.
- Below 50% for the whole batch: the next batch may run one more task.

## 4. Pick the batch

A task is **ready** when:
- its `STATUS.md` state is `todo`, or `blocked` and the user says the blocker is cleared;
- every dependency is `done-local` (local phase) or `done` (cloud phase) and merged into the integration branch;
- it belongs to the requested phase. In the local phase, never pick M1-00, M1-03b, M1-15, or M1-17, and never ask a task to meet a criterion marked *Cloud phase*.

Take up to the parallel-task count. Prefer tasks on different parallel tracks (README § Order). If two picked tasks will clearly edit the same files, run them in different batches. Show the user the batch, each task's budget and model, and why each is ready, before launching.

## 5. Model choice

- **Task-orchestrators: Sonnet** (`model: "sonnet"`), the default in their agent definition. Use the lower-usage model unless churn says otherwise (§ 7).
- **Workers: Haiku**, fixed in their agent definition. Task-orchestrators keep sub-tasks small enough for it.
- **Scale up** to `model: "opus"` only by the churn rule, or when the user asks. Record the model in `STATUS.md`.

## 6. Launch

Mark each picked task `in-progress` in `STATUS.md` with branch `task/<id>` and its model. Launch every task-orchestrator **in one message**, so they run in parallel:
- `subagent_type: "task-orchestrator"`, `isolation: "worktree"`, `run_in_background: true`, `model` as chosen.
- Prompt (fill in every field; the agent starts cold):
  ```
  Task: <id> — <title>
  Task file: design/tasks/<milestone>/<file>.md
  Phase: <local|cloud>
  Branch: create task/<id> in your worktree and commit your work there.
  Run id: <run id>
  Resource budget: <cpus> CPUs, <memory> GiB, at most <n> containers at once
  Churn thresholds: sub-task redo ratio 0.30; any sub-task failing twice; rewrite ratio 0.50 after 3 commits
  Dependencies already merged: <ids>
  Parallel tasks in this batch (do not edit their files): <ids and their main paths>
  Scope note (if split): <the part of the task this run owns, or "whole task">
  Load the run-task skill and follow it. Report in its format.
  ```

If a task needs a live model run (stage L3), stop and ask the user to approve it with the estimated cost; then pass `Live model run: approved, budget <$n>` in that task's prompt. Without that line, no task makes model calls.

Do not do task work yourself while they run. Wait for completion notifications; never predict their results.

## 7. Churn

Churn is work redone instead of finished. Measure it for each task from its report and from its branch, which is visible from here because worktrees share the repository:

```sh
# rewrite ratio: lines deleted / lines added on the task branch, after its first commit
git log --format= --numstat <integration>..task/<id> | awk '{a+=$1; d+=$2} END {if (a) print d/a}'
git rev-list --count <integration>..task/<id>   # commits
```

| Signal | Threshold |
| --- | --- |
| Sub-task redo ratio (re-dispatched or failed sub-tasks / all sub-tasks) | above 0.30 |
| One sub-task failing | twice |
| Rewrite ratio, once the branch has 3 or more commits | above 0.50 |
| Task-orchestrator reports `State: churn` | always |

When a task crosses a threshold, act in this order, one step per occurrence:
1. **Split.** Divide the remaining work into two or more narrower scopes, using the task's Suggested breakdown and the report. Relaunch each as its own task-orchestrator on Sonnet, with a `Scope note`, starting from the task branch. Run the parts in sequence if they share files.
2. **Scale up.** Relaunch the remaining work on `model: "opus"`, starting from the task branch, with the report's churn notes in the prompt.
3. **Stop.** Mark the task `blocked` with the churn record and ask the user.

Record every churn event (signal, value, action) in the task's `STATUS.md` notes.

## 8. Integrate

For each finished task-orchestrator, read its report.
- **`done-local` or `done`:** review `git diff <integration>...task/<id>`. Check scope, that it touches no shared doc, and that `golangci-lint` passed with the repository config. Merge into the integration branch with `git -c commit.gpgsign=false merge --no-ff task/<id>`. Resolve conflicts only when mechanical; otherwise ask.
- **`blocked`:** keep the branch; record the blocker; list what the user must decide.
- **`churn`:** apply § 7.
- **`failed`:** keep the branch; record why; retry in this run only if the cause is clearly transient.

Then sweep leftovers: `docker ps -a --filter label=pubcommittee.task=<id> -q | xargs -r docker rm -f`, and the same for volumes except the shared caches `pc-gomod` and `pc-gocache`.

**Shared docs are yours alone.** Task-orchestrators never edit `architecture/README.md`, `design/*.md`, `design/tasks/README.md`, or `STATUS.md`. Apply accepted doc proposals yourself, one at a time, after merging, and commit them on the integration branch, unsigned (§ Git and signing). Answers to open questions that need a user decision go to the user.

## 9. Compose acceptance

After merging a batch, or with `--accept`, run the whole application on the integration branch as one Compose project:

```sh
docker compose -p pc-accept -f compose.yaml -f .task/compose.limits.yaml up -d --build --wait
docker compose -p pc-accept run --rm acceptance        # or each test/acceptance/<task>/run.sh in a runner container
docker compose -p pc-accept down -v
```

- Write `.task/compose.limits.yaml` (ignored by git) giving **every** service `deploy.resources.limits` (`cpus`, `memory`) and `mem_reservation`, sized so the whole stack fits the budget in § 3. Never run the stack without limits.
- The stack runs on the `local` build target with the filesystem stores and no cloud service (local phase).
- Run every task's acceptance script under `test/acceptance/<task>/` that is `done-local` or `done`, plus the M1-16 checks once they exist. Until `compose.yaml` exists (M1-01), skip this step and say so.
- Report each script's result. A failure that a task's own scoped run did not catch goes back to that task as a new run.
- Always tear the project down, even after a failure.

## 10. Report

Tell the user, briefly:
- per task: state, branch, model, acceptance results with evidence, churn values and any action taken;
- compose acceptance results;
- resource peaks for the batch, and any limits lowered or containers paused;
- open questions, grouped by who must answer them; doc changes applied and proposed;
- operator-only steps needed (credentials, spend approval, `tofu apply`, `pubctl`, cloud accounts);
- the next ready batch, with its proposed budgets and models.

Hand-off to the user, when they want to land work on `main`. They run it themselves, with their own signing key:

```sh
git switch main
git merge --squash <milestone>-integration
git commit -S
```

Only that signed commit reaches `main`. Never merge into `main`, push, or open a pull request yourself, even if asked to sign: signing is the user's step.

## Rules

- Depth is three. Launch only `task-orchestrator` agents.
- Every container in a run has explicit CPU and memory limits and the run label.
- Never run operator-only steps (environments.md § Operator and agent), paid live model runs, or anything against a cloud environment. A task that needs one reports `blocked`. Only you, the top-level agent, may use `pubops` (AGENTS.md); never pass its tools or results to a task-orchestrator.
- Open questions: a task answers the ones inside its own scope with evidence (implementation choices, or what its task file says to answer); product, policy, legal, and spending decisions go to the user ([run-task § Rules](../run-task/SKILL.md)). Never assume an answer to the second kind.
