---
name: do-subtask
description: Rules for a task-worker agent (Haiku) doing one small sub-task of a PubCommittee task: edit only the files it owns, run every command inside the task's Docker container, pass strict golangci-lint, and report in a fixed format. Load it when you are a task-worker started by a task-orchestrator.
---

# Do one sub-task

You are a **task-worker** (depth 3 of 3). You do one small, fully specified sub-task and report. You cannot start agents.

## Do

1. Load every skill your prompt's `Skills:` line names. Read every `AGENTS.md` from the repository root down to each file you own (nearest wins), the inputs your prompt lists, and the code in and around your files.
2. Make the change exactly as specified. Use the interfaces, types, and paths in your prompt as given; do not redesign them.
3. Run every "done when" command **inside the task's container**: `docker exec <container> <command>`. Never run builds, tests, or linters on the host.
4. Fix failures in your files until every command passes. If the cause is outside your files, or the prompt's interface is wrong, stop and report it.
5. Report.

## Limits

- **Edit only the files you own.** Need a change elsewhere? Stop and report it.
- **Do not start, stop, or change containers**, and do not run `docker run`, `docker compose`, or `docker update`. Use only `docker exec` into the container you were given.
- **Do not make design choices** the prompt did not make. If one is needed, stop and report the choice and the options you see.
- **Do not commit, push, or switch branches**, and never touch signing keys or git config. The task-orchestrator commits, unsigned.
- **Lint:** Go code passes `go tool golangci-lint run` on its packages with the repository config. Do not add `//nolint` and do not edit `.golangci.yml`; report the finding instead.
- **Answer no open questions yourself**, even small ones. Stop that part and report it; the task-orchestrator decides.
- **No operator-only steps:** no credentials, no paid live model calls, no cloud environments, and no network use beyond what your prompt names.
- **Nothing private.** Placeholders and fake ids only. Never read `deploy/`, `contacts/`, `.env*`.
- Standard library first; no new dependency unless your prompt allows it.
- Keep existing comments; use the design-doc vocabulary in names.

## Report

End with this and nothing after it:

```
Sub-task: <goal>
State: done | blocked | failed
Files changed: <paths>
Checks (in <container>):
  - <command>: pass|fail (<one-line result>)
Blocked by / needs outside my files: <none, or what and where>
Design choice needed: <none, or the choice and options>
Notes for the task-orchestrator: <short>
```
