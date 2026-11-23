#!/bin/sh
# M1-16 acceptance, local gate, HTTP part: runs in the dev container (Go, Node 24, curl) with the
# repository mounted at /src, against a running stack (see stack.sh): read-api serving the data
# root DATA_DIR, and the SvelteKit app reading from it.
# Environment: DATA_DIR (required), READ_API_URL (default http://read-api:8080),
#   APP_URL (required: the web app, http://app:3000 in stack.sh; unset skips), PUBLIC_API_URL (what the app advertises; default READ_API_URL).
# The browser checks (no scripts, installability, offline, page cost) run in the Playwright
# container; stack.sh drives both. Cloud-phase criteria (the hosted gate) are not attempted.
set -eu
cd "$(dirname "$0")/../../.."
# The whole-stack acceptance loop (compose service "acceptance") has no app; this script then has nothing to check.
[ -n "${APP_URL:-}" ] || { echo "skip: M1-16 needs the web app (APP_URL); run test/acceptance/M1-16/stack.sh"; exit 0; }
export READ_API_URL=${READ_API_URL:-http://read-api:8080}
fail=0
node test/acceptance/M1-16/check.mjs || fail=1
sh test/acceptance/M1-16/admin.sh || fail=1
[ "$fail" -eq 0 ] || { echo "M1-16 acceptance (HTTP part): FAILED"; exit 1; }
echo "M1-16 acceptance (HTTP part): all checks passed"
