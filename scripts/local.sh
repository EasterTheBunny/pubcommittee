#!/bin/sh
# Run the whole PubCommittee app locally: web app, read-api, admin-api, OpenTelemetry Collector,
# and the jobs that fill the local data directory from the example data in fixtures/.
#
#   scripts/local.sh build        build every image (and pull the collector image)
#   scripts/local.sh up           serve in the foreground; Ctrl+C stops and removes the stack
#   scripts/local.sh up -d        serve in the background; stop with `scripts/local.sh down`
#   scripts/local.sh down         stop and remove every container and network of the stack
#   scripts/local.sh seed         (re)fill .local-data from fixtures/ (also run by `up` when empty)
#   scripts/local.sh job <job> [flags]   run one job (job-ingest, job-exports, job-summarize) as you
#   scripts/local.sh test [--with-admin] acceptance checks against the running app (`up` or `up -d`)
#   scripts/local.sh data [fake|live]    show or switch the data set (restarts a running stack)
#   scripts/local.sh data live --refresh copy the live-model outputs again, replacing the local live copy
#
# Everything runs in Docker with the limits set in compose.yaml. No model calls: by default summaries
# come from the fake generator; `data live` switches to the stored outputs of the M1-09 live-model run.
# Data stays in .local-data/ (git-ignored); delete it to start over.
# Environment: COMPOSE_PROJECT (default pubcommittee), RATE_LIMITS (default effectively unlimited: one local user).
set -eu
cd "$(dirname "$0")/.."

LOCAL_UID=$(id -u)
LOCAL_GID=$(id -g)
RATE_LIMITS=${RATE_LIMITS:-text=100000,live=100000,bulk=100000,discovery=100000}
PUBLIC_UI_URL=${PUBLIC_UI_URL:-http://localhost:3000}
export LOCAL_UID LOCAL_GID RATE_LIMITS PUBLIC_UI_URL

project=${COMPOSE_PROJECT:-pubcommittee}
exit_bills=us-119-hr-4313,us-119-hr-3062,us-119-hr-3427
serve="read-api otel-collector web admin-api"

dc() { docker compose -p "$project" --profile app --profile jobs "$@"; }
job() { dc run --rm --no-deps "$@"; }

usage() {
	sed -n '2,17p' "$0" | sed 's/^# \{0,1\}//'
	exit "${1:-0}"
}

build() {
	dc build
	# The dev image the acceptance checks run in.
	docker compose -p "$project" --profile acceptance build acceptance
	dc pull --ignore-buildable --quiet
}

# The jobs and admin-api run as you and take a lock file in .local-data/. A directory that Docker
# created as root (a bind mount of a missing path, or an older run) makes that fail with
# "permission denied"; hand everything in it back to you, through a container so no sudo is needed.
ensure_data_dir() {
	dir=${1:-.local-data}
	# Created here, owned by you, so Docker does not create it as root.
	mkdir -p "$dir"
	if [ ! -w "$dir" ] || [ -n "$(find "$dir" ! -user "$LOCAL_UID" -print -quit 2>/dev/null)" ]; then
		echo "local data: $dir/ has files you do not own (created by Docker as root); giving them back to $(id -un)"
		docker run --rm -v "$PWD/$dir:/data" alpine@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf \
			chown -R "$LOCAL_UID:$LOCAL_GID" /data
	fi
}

seed() {
	ensure_data_dir
	# The jobs push metrics to the collector; without it each job waits out its flush timeout.
	dc up -d --wait otel-collector >/dev/null
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
	[ -f .local-data/.dataset ] || printf 'fake\n' >.local-data/.dataset
}

down() {
	dc down --remove-orphans
}

# One job, files written as you. Starts the collector only when the stack is not already up,
# and then removes what it started on exit, Ctrl+C included.
run_job() {
	case "${1:-}" in
	job-ingest | job-exports | job-summarize) ;;
	*)
		echo "job: one of job-ingest, job-exports, job-summarize" >&2
		exit 2
		;;
	esac
	ensure_data_dir
	if [ -z "$(dc ps -q --status running otel-collector)" ]; then
		trap 'trap - EXIT INT TERM HUP; dc down --remove-orphans >/dev/null 2>&1 || true' EXIT
		trap 'exit 130' INT TERM HUP
		dc up -d --wait otel-collector >/dev/null
	fi
	job "$@"
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
	ensure_data_dir
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

# Data sets. "fake": seeded from fixtures/ with the fake generator. "live": a copy of the stored
# outputs of the M1-09 live-model run (no model call; the source is only read). The set in use is
# .local-data/, named in .local-data/.dataset; the other is parked in .local-data.<name>/.
# Resolved from the main checkout, so it also works from a git worktree; PUBCOMMITTEE_LIVE_DATA overrides it.
main_root=$(git worktree list --porcelain 2>/dev/null | sed -n '1s/^worktree //p')
live_src=${PUBCOMMITTEE_LIVE_DATA:-${main_root:-.}/.claude/worktrees/m1-09-validators/.local-data}

dataset() {
	if [ -f .local-data/.dataset ]; then
		cat .local-data/.dataset
	elif [ ! -d .local-data ]; then
		echo none
	elif ls .local-data/generated/batches 2>/dev/null | grep -q '^msgbatch_'; then
		# Copied by hand before this command existed: only the live run's batches carry API batch ids
		# (the fake generator's are named fake-...). File names only; no contents are read.
		echo live
	else
		echo fake
	fi
}

data_cmd() {
	cur=$(dataset)
	case "${1:-}" in
	"")
		echo "data: $cur"
		return
		;;
	fake | live) want=$1 ;;
	*) usage 2 ;;
	esac
	refresh=""
	case "${2:-}" in
	"") ;;
	--refresh)
		[ "$want" = live ] || usage 2
		refresh=1
		;;
	*) usage 2 ;;
	esac
	if [ "$cur" = "$want" ] && [ -z "$refresh" ]; then
		echo "data: already using $want (scripts/local.sh data live --refresh copies the live outputs again)"
		return
	fi
	if [ "$want" = live ] && { [ -n "$refresh" ] || [ ! -d ".local-data.live" ]; } && [ ! -d "$live_src/bills" ]; then
		echo "data: no live data: $live_src is missing" >&2
		exit 1
	fi
	was_running=""
	if [ -n "$refresh" ]; then
		# The live set is a copy (exports and the change feed are rebuilt from it), so dropping it loses
		# only local changes made while it was in use, such as pubctl track actions.
		[ -n "$(dc ps -q --status running read-api)" ] && was_running=1
		[ -z "$was_running" ] || dc down --remove-orphans >/dev/null 2>&1
		if [ "$cur" = live ]; then
			ensure_data_dir
			rm -rf .local-data
			cur=none
		elif [ -d .local-data.live ]; then
			ensure_data_dir .local-data.live
			rm -rf .local-data.live
		fi
	fi
	if [ "$cur" != none ] && [ -e ".local-data.$cur" ]; then
		echo "data: .local-data.$cur/ already exists; move it aside before switching" >&2
		exit 1
	fi
	running=""
	[ -n "$(dc ps -q --status running read-api)" ] && running=1
	[ -z "$was_running" ] || running=1
	[ -z "$running" ] || dc down --remove-orphans >/dev/null 2>&1
	if [ "$cur" != none ]; then
		ensure_data_dir
		printf '%s\n' "$cur" >.local-data/.dataset
		mv .local-data ".local-data.$cur"
	fi
	if [ -d ".local-data.$want" ]; then
		mv ".local-data.$want" .local-data
	elif [ "$want" = live ]; then
		echo "data: copying the live-model outputs from $live_src"
		cp -a "$live_src" .local-data
		ensure_data_dir
		printf 'live\n' >.local-data/.dataset
		echo "data: rebuilding exports and the change feed"
		(run_job job-exports >/dev/null)
	else
		mkdir -p .local-data
		printf 'fake\n' >.local-data/.dataset
		seed
		dc down --remove-orphans >/dev/null 2>&1
	fi
	printf '%s\n' "$want" >.local-data/.dataset
	if [ "$cur" = none ]; then
		echo "data: now using $want"
	else
		echo "data: now using $want (the other set is kept in .local-data.$cur/)"
	fi
	if [ -n "$running" ]; then
		dc up -d --wait --remove-orphans $serve >/dev/null
		echo "serving again: app http://localhost:3000"
	fi
}

playwright_image=mcr.microsoft.com/playwright:v1.65.1-noble@sha256:1507cbfc550a63a6b7396791ca4f00df17beb56342d2447b3f106c306f525579

# Acceptance checks against the running app, read-only unless --with-admin:
#   HTTP: test/acceptance/M1-01, M1-03a (runtime part), M1-16 check.mjs (exit criteria on the real bills)
#   browser: test/acceptance/M1-16/e2e (no scripts, labels, version history, catalog, install, offline)
#   --with-admin: test/acceptance/M1-16/admin.sh (pubctl track and untrack; adds public log entries)
# Code-level checks (go test, lint, spec) are not part of this; they run in CI and in the dev container.
test_stack() {
	with_admin=""
	[ "${1:-}" = "--with-admin" ] && with_admin=1
	web=$(dc ps -q --status running web)
	if [ -z "$web" ] || [ -z "$(dc ps -q --status running read-api)" ]; then
		echo "test: the app is not running; start it with scripts/local.sh up -d (or up in another terminal)" >&2
		exit 2
	fi
	trap 'trap - EXIT INT TERM HUP; docker rm -f "$project-test-http" "$project-test-admin" "$project-test-browser" >/dev/null 2>&1 || true' EXIT
	trap 'exit 130' INT TERM HUP
	fail=0
	env="-e READ_API_URL=http://read-api:8080 -e READ_API_METRICS_URL=http://read-api:9464 -e APP_URL=http://web:3000 -e PUBLIC_API_URL=http://localhost:8080"

	echo "== HTTP checks"
	# shellcheck disable=SC2086
	docker compose -p "$project" --profile acceptance run --rm --no-deps --name "$project-test-http" \
		-u "$LOCAL_UID:$LOCAL_GID" -e HOME=/tmp $env acceptance \
		sh -c 'f=0; for t in M1-01 M1-03a; do echo "-- $t"; sh test/acceptance/$t/run.sh || f=1; done; echo "-- M1-16"; node test/acceptance/M1-16/check.mjs || f=1; exit $f' ||
		fail=1

	if [ -n "$with_admin" ]; then
		echo "== admin checks (writes public log entries; leaves the catalog as it found it)"
		# shellcheck disable=SC2086
		docker compose -p "$project" --profile acceptance run --rm --no-deps --name "$project-test-admin" \
			-u "$LOCAL_UID:$LOCAL_GID" -e HOME=/tmp -e GOCACHE=/tmp/gocache -e GOMODCACHE=/tmp/gomod $env \
			-e DATA_DIR=/src/.local-data -v "$PWD:/src:ro" -v "$PWD/.local-data:/src/.local-data" acceptance \
			sh test/acceptance/M1-16/admin.sh || fail=1
	fi

	echo "== browser checks"
	skip=""
	# The two large-bill checks need us-119-hr-3115, which the default seed does not track.
	if [ "$(curl -s -o /dev/null -w '%{http_code}' http://localhost:8080/v1/bills/us-119-hr-3115)" != 200 ]; then
		skip="longest real bill|largest bill"
		echo "(skipping the two large-bill checks: us-119-hr-3115 is not tracked)"
	fi
	# Same network namespace as the web container, so the browser reaches the app as localhost
	# (service workers need a secure context). @playwright/test is cached in a volume.
	docker run --rm --name "$project-test-browser" --network "container:$web" --cpus 1.5 --memory 1792m \
		--label "com.docker.compose.project=$project" \
		-v "$PWD:/src:ro" -v "$project-playwright:/pw" -w /src/app \
		-e HOME=/tmp -e NODE_PATH=/pw/node_modules -e BASE_URL=http://localhost:3000 -e API_URL=http://read-api:8080 \
		-e PW_OUTPUT=/tmp/pw -e TIMINGS_FILE=/tmp/timings.jsonl -e SKIP="$skip" "$playwright_image" sh -c '
			[ -d /pw/node_modules/@playwright/test ] || (cd /pw && npm i --no-save --no-audit --no-fund @playwright/test@1.65.1 >/dev/null)
			set -- --config ../test/acceptance/M1-16/e2e/playwright.config.ts
			[ -z "$SKIP" ] || set -- "$@" --grep-invert "$SKIP"
			/pw/node_modules/.bin/playwright test "$@"' || fail=1

	if [ "$fail" -eq 0 ]; then echo "acceptance: all checks passed"; else echo "acceptance: FAILED (see above)"; fi
	return "$fail"
}

case "${1:-}" in
build) build ;;
test)
	shift
	test_stack "$@"
	;;
data)
	shift
	data_cmd "$@"
	;;
up)
	shift
	up "$@"
	;;
down) down ;;
seed)
	seed
	down >/dev/null 2>&1
	;;
job)
	shift
	run_job "$@"
	;;
-h | --help | help) usage ;;
*) usage 2 ;;
esac
