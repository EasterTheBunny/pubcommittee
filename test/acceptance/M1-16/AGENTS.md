# AGENTS.md — test/acceptance/M1-16

Purpose: the M1 local-gate acceptance: the whole application on one Compose project, each exit criterion proved end to end.
Skills: `tool-compose-acceptance`, `tool-playwright`, `tool-docker`.
Owns: `stack.sh` (host driver), `run.sh`, `check.mjs`, `admin.sh`, `prepare.sh`, `compose.yaml`, `e2e/`, `EVIDENCE.md`. Does not own: other tasks' scripts.

## Rules
- Needs a data root in `.local-data/` from a live run; the script makes no model call and reads no key.
- Limits and labels live in the uncommitted `.task/compose.limits.yaml`; at most four containers at once.
- Playwright runs from `app/` with `NODE_PATH=/src/app/node_modules` and `--config ../test/acceptance/M1-16/e2e/playwright.config.ts`.
- `EVIDENCE.md` maps every criterion to a check; update it when a check changes.

## Commands
- `RUN_ID=<id> sh test/acceptance/M1-16/stack.sh` (host; `KEEP=1` leaves the stack up)

## Gotchas
- `prepare.sh` and `admin.sh` write to the data root (catalog, exports, log); run them on a copy.
- Shared Go caches are root-owned: chown the worktree back to 1000:1000 after container runs.
