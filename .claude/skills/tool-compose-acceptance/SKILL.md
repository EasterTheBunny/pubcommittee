---
name: tool-compose-acceptance
description: Writing task-scoped acceptance scripts under test/acceptance/<task>/ and running services with Docker Compose under explicit resource limits; used by task-orchestrators for their own task and by the orchestrator for the whole stack. Load when writing or running acceptance checks.
---

# Acceptance scripts and Compose

## Scripts: `test/acceptance/<task>/run.sh`

- One directory per task (`test/acceptance/m1-10/`). A task writes only its own directory.
- Tests only that task's in-phase acceptance criteria; one clearly named check per criterion, printing `PASS <criterion>` or `FAIL <criterion>: <reason>`; exit non-zero on any failure.
- Reads its targets from environment variables, never hard-coded: `READ_API_URL`, `UI_URL`, `ADMIN_API_URL`, `DATA_DIR`. The same script runs against a task's own services and against the whole stack.
- POSIX `sh` plus `curl` and `jq`, or a Go test under `test/acceptance/<task>/` run with `go test`. No host tools beyond these.
- No secrets, no network beyond the stack and the public sources the task names.

## Compose under limits

Never run services without limits. Write an override (in `.task/`, ignored by git) that gives every started service limits:

```yaml
# .task/compose.limits.yaml
services:
  read-api:
    deploy: { resources: { limits: { cpus: "0.5", memory: 256M } } }
    mem_reservation: 128M
```

```sh
docker compose -p pc-<id> -f compose.yaml -f .task/compose.limits.yaml up -d --wait read-api
READ_API_URL=http://read-api:8080 docker compose -p pc-<id> -f compose.yaml -f .task/compose.limits.yaml \
  run --rm acceptance test/acceptance/<id>/run.sh
docker compose -p pc-<id> down -v
```

- Task-orchestrators: project `pc-<id>`, only the services the task needs, limits within the task budget.
- Orchestrator: project `pc-accept`, every service, limits within the batch budget, every `done-local` task's script.
- Always `down -v`, even after a failure. Containers carry the run and task labels (Compose `labels:` in the override).
