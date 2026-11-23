#!/bin/sh
# Run the whole PubCommittee app locally: web app, read-api, admin-api, OpenTelemetry Collector,
# and the jobs that fill the local data directory from the example data in fixtures/.
#
#   scripts/local.sh build        build every image (and pull the collector image)
#   scripts/local.sh up           serve in the foreground; Ctrl+C stops and removes the stack
#   scripts/local.sh up -d        serve in the background; stop with `scripts/local.sh down`
#   scripts/local.sh down         stop and remove every container and network of the stack
#   scripts/local.sh seed         (re)fill .local-data from fixtures/ (also run by `up` when empty)
#
# Everything runs in Docker with the limits set in compose.yaml. No model calls: summaries come from
# the fake generator. Data stays in .local-data/ (git-ignored); delete it to start over.
# Environment: COMPOSE_PROJECT (default pubcommittee), RATE_LIMITS (default raised for one browser).
set -eu
cd "$(dirname "$0")/.."

LOCAL_UID=$(id -u)
LOCAL_GID=$(id -g)
RATE_LIMITS=${RATE_LIMITS:-text=6000,live=3000,bulk=1200,discovery=6000}
PUBLIC_UI_URL=${PUBLIC_UI_URL:-http://localhost:3000}
export LOCAL_UID LOCAL_GID RATE_LIMITS PUBLIC_UI_URL

project=${COMPOSE_PROJECT:-pubcommittee}
exit_bills=us-119-hr-4313,us-119-hr-3062,us-119-hr-3427
serve="read-api otel-collector web admin-api"

dc() { docker compose -p "$project" --profile app --profile jobs "$@"; }
job() { dc run --rm --no-deps "$@"; }

usage() {
	sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'
	exit "${1:-0}"
}

build() {
	dc build
	dc pull --ignore-buildable --quiet
}

seed() {
	# Created here, owned by you, so Docker does not create it as root.
	mkdir -p .local-data
	echo "seed: US Code provisions and selection policy"
	job job-ingest --law
	job job-ingest --publish-policy v1 --reason "local run"
	echo "seed: catalog sample (status only; bills without text report a failed pull and keep their entry)"
	args=""
	for f in fixtures/catalog/us-*.xml; do args="$args --bill $(basename "$f" .xml)"; done
	# shellcheck disable=SC2086
	job job-ingest $args >/dev/null 2>&1 || true
	echo "seed: exit bills, every version, citations and comparisons"
	job job-ingest --bill "$exit_bills" --track
	job job-ingest --evaluate >/dev/null
	echo "seed: explanations and summaries (fake generator)"
	job job-summarize --submit --bill "$exit_bills" >/dev/null
	job job-summarize --collect >/dev/null
	echo "seed: exports and change feed"
	job job-exports >/dev/null
}

down() {
	dc down --remove-orphans
}

# Runs once on any exit of `up` in the foreground: Ctrl+C, a failed service, or the shell closing.
cleanup() {
	trap - EXIT INT TERM HUP
	echo
	echo "stopping and removing the local stack..."
	dc down --remove-orphans --timeout 10 >/dev/null 2>&1 || true
	echo "stopped. Data kept in .local-data/."
}

up() {
	[ -d .local-data/bills ] || seed
	if [ "${1:-}" = "-d" ]; then
		dc up -d --wait --remove-orphans $serve
		echo "serving: app http://localhost:3000  read-api http://localhost:8080  admin-api http://localhost:8081"
		echo "stop with: scripts/local.sh down"
		return
	fi
	trap cleanup EXIT
	trap 'exit 130' INT TERM HUP
	echo "serving: app http://localhost:3000  read-api http://localhost:8080  admin-api http://localhost:8081"
	echo "press Ctrl+C to stop; containers and networks are removed on exit"
	dc up --remove-orphans $serve
}

case "${1:-}" in
build) build ;;
up)
	shift
	up "$@"
	;;
down) down ;;
seed) seed ;;
-h | --help | help) usage ;;
*) usage 2 ;;
esac
