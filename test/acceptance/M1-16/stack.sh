#!/bin/sh
# M1-16 local gate driver (host): the whole application as one Compose project under limits.
#   read-api (local target, over .local-data) + the SvelteKit app (Node 24) + a dev container
#   (checks, pubctl, admin-api) + Playwright (browser checks), at most four containers at once.
# Needs .task/compose.limits.yaml (uncommitted; limits and labels for read-api, app, playwright,
# see compose.yaml here) and a data root in .local-data (the three exit bills, law, citations,
# comparisons and generated explanations from an earlier live run; never regenerated here).
# Environment: RUN_ID (label pubcommittee.run), KEEP=1 leaves the stack up.
# No model calls, no credentials: reads and writes only the data root.
set -eu
cd "$(dirname "$0")/../../.."
P=pc-m1-16
RUN_ID=${RUN_ID:-local}
F="-f compose.yaml -f test/acceptance/M1-16/compose.yaml -f .task/compose.limits.yaml"
[ -f .task/compose.limits.yaml ] || { echo "missing .task/compose.limits.yaml (limits for every service)" >&2; exit 2; }
[ -d .local-data/bills ] || { echo "missing .local-data (copy the data root of the live run)" >&2; exit 2; }
# shellcheck disable=SC2086
cleanup() {
	[ -n "${KEEP:-}" ] && return 0
	docker rm -f $P-dev >/dev/null 2>&1 || true
	docker compose -p $P $F --profile browser down -v >/dev/null 2>&1 || true
}
trap cleanup EXIT
# shellcheck disable=SC2086
docker compose -p $P $F up -d --build --wait read-api app playwright
docker rm -f $P-dev >/dev/null 2>&1 || true
docker run -d --name $P-dev --label pubcommittee.run="$RUN_ID" --label pubcommittee.task=M1-16 \
	--cpus 1.5 --memory 2g --memory-swap 2g --pids-limit 512 --network ${P}_default \
	-v "$PWD":/src -w /src -v pc-gomod:/go/pkg/mod -v pc-gocache:/root/.cache/go-build \
	-e GOFLAGS=-buildvcs=false -e DATA_DIR=/src/.local-data pc-dev:orch sleep infinity >/dev/null
docker exec $P-dev sh test/acceptance/M1-16/prepare.sh
docker exec -e APP_URL=http://app:3000 -e READ_API_URL=http://read-api:8080 $P-dev sh test/acceptance/M1-16/run.sh
docker exec -e NODE_PATH=/src/app/node_modules -e TIMINGS_FILE=/src/.task/m1-16-timings.jsonl -w /src/app \
	$P-playwright-1 npx playwright test --config ../test/acceptance/M1-16/e2e/playwright.config.ts
echo "M1-16 local gate: all checks passed"
