---
name: tool-docker
description: How PubCommittee agents use Docker for builds, tests, and services. Workers: running commands with docker exec in the task's container. Task-orchestrators: starting containers with explicit CPU and memory limits, labels, caches, and network rules, and cleaning up. Load before running any build, test, lint, or service.
---

# Docker for agents

## Workers: exec only

Run every command in the container your prompt names:

```sh
docker exec <container> go test ./internal/data/...
docker exec -w /src/app <container> npm run check
docker exec -e CGO_ENABLED=0 <container> go build -tags memory ./...
```

- The worktree is mounted at `/src`. Paths in commands are container paths.
- Never run `docker run`, `docker compose`, `docker update`, `docker rm`, or `docker stop`.
- Container missing or unhealthy? Stop and report it.
- Do not run `git` inside the container; the worktree's `.git` points outside the mount.

## Task-orchestrators: start, limit, label, clean

Every container:

```sh
docker run -d --name pc-<id>-<role> \
  --label pubcommittee.run=<run id> --label pubcommittee.task=<id> \
  --cpus <n> --memory <m> --memory-swap <m> --pids-limit 512 \
  -v "$PWD":/src -w /src \
  -v pc-gomod:/go/pkg/mod -v pc-gocache:/root/.cache/go-build \
  -e GOFLAGS=-buildvcs=false \
  <image> sleep infinity
```

- `--memory-swap` equal to `--memory`, so a container cannot swap past its limit.
- The sum of limits across your containers stays within your budget; at most the container count in your prompt.
- **Images:** `docker/dev.Dockerfile` when it exists (`docker build -f docker/dev.Dockerfile -t pc-dev:<short sha> .`); before then the official `golang` image at the pinned Go version and the official `node` image. Record the digest: `docker image inspect --format '{{index .RepoDigests 0}}' <image>`.
- **Network:** fetch modules first (`go mod download`), then test offline: `docker network disconnect bridge pc-<id>-dev`, or a second container with `--network none`.
- **Never mount** `deploy/`, `contacts/`, `.env*`, `.local-data/`, or the Docker socket.
- **Usage:** `docker stats --no-stream --filter label=pubcommittee.task=<id>`.
- **Clean up** always: `docker ps -aq --filter label=pubcommittee.task=<id> | xargs -r docker rm -f`. Keep the shared `pc-gomod` and `pc-gocache` volumes.

## Live model runs (stage L3)

Only when the orchestrator's prompt says the operator approved a live run, and only for that run:
- The key lives in `.env.anthropic` at the repository root (ignored by git and Docker builds). Never read, print, copy, or mount it. Pass it to the one container that makes model calls with `--env-file <repo root>/.env.anthropic`; no other container gets it.
- Never run `env`, `printenv`, `docker inspect`, or `docker compose config` against that container, and never log request headers: each would print the key.
- Remove the container as soon as the run ends.
- Never export `ANTHROPIC_API_KEY` in a host shell: Claude Code itself would switch to API billing.

## Image builds (`docker/Dockerfile`)

- One multi-stage file with `SERVICE` and `TARGET` build arguments; the builder cross-compiles (`--platform=$BUILDPLATFORM`, `GOOS`/`GOARCH` from `TARGETOS`/`TARGETARCH`); the final stage is distroless static, non-root, pinned by digest.
- Local check of one unit: `docker build -f docker/Dockerfile --build-arg SERVICE=read-api --build-arg TARGET=local -t pc-read-api:dev .`
- The build context must not contain private files: `.dockerignore` excludes `deploy/`, `contacts/`, `.local-data/`, `.env*`, `.task/`.
