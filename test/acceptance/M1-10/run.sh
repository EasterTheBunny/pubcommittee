#!/bin/sh
# M1-10 acceptance: read-api routes, caching, conditional GET, canonical
# addresses, and documents written by another container.
# Runtime part (needs the stack on the local target with the fixture set
# written by the seed service: `docker compose --profile seed run --rm seed`):
#   curl checks against READ_API_URL (default http://localhost:8080).
# Code part (needs the Go toolchain and the module in $PWD): in-process
# contract tests over every route (memory), the live contract test against
# READ_API_URL, rate limiting, logs, dependency rules, size budget.
# Environment: READ_API_URL; READ_API_LOG (optional file holding the
# service's log lines, checked for client addresses).
set -eu
api=${READ_API_URL:-http://localhost:8080}
bill=us-119-hr-4127

fail() { echo "FAIL: $*" >&2; exit 1; }

get() { curl -s -D /tmp/m110.head -o /tmp/m110.body -w '%{http_code}' "$@"; }

expect() { # name want got
	[ "$3" = "$2" ] || fail "$1: status $3, want $2"
	echo "ok: $1"
}

has() { # name file pattern
	grep -q -- "$3" "$2" || fail "$1: missing $3"
	echo "ok: $1"
}

expect "discovery" 200 "$(get "$api/v1")"
expect "spec served" 200 "$(get "$api/v1/openapi.yaml")"

# Documents written by the seed container are served by this one.
expect "bill written by another container" 200 "$(get "$api/v1/bills/$bill")"
has "bill carries review state and provenance" /tmp/m110.body '"review_state":"open"'
has "bill carries provenance" /tmp/m110.body '"provenance"'

# curl can fetch any line range by canonical address: anchors, depth, text, provenance.
expect "line range" 200 "$(get "$api/v1/bills/$bill/v/rh/s/s4/l/1-3")"
for want in '"anchor":"s4-l1"' '"depth":1' '"text":"The Secretary' '"block_to"' '"provenance"' '"content_class":"official"'; do
	has "line range has $want" /tmp/m110.body "$want"
done
has "official text is immutable" /tmp/m110.head 'immutable'

etag=$(sed -n 's/^[Ee][Tt]ag: *//p' /tmp/m110.head | tr -d '\r')
[ -n "$etag" ] || fail "no ETag"
expect "If-None-Match answers 304" 304 "$(get -H "If-None-Match: $etag" "$api/v1/bills/$bill/v/rh/s/s4/l/1-3")"

# Version-less address resolves to latest and says so.
expect "latest section" 200 "$(get "$api/v1/bills/$bill/s/s4")"
has "latest is stated" /tmp/m110.body '"resolved_from_latest":true'
has "latest is pinned in Content-Location" /tmp/m110.head '/v1/bills/us-119-hr-4127/v/rh/s/s4'
# Old version points to its comparison.
expect "old version" 200 "$(get "$api/v1/bills/$bill/v/ih")"
has "old version links to compare" /tmp/m110.body '/compare/rh'

# Untracked catalog entries carry no bill text.
expect "catalog" 200 "$(get "$api/v1/bills?scope=catalog&q=naming")"
has "catalog finds the untracked entry" /tmp/m110.body 'us-119-hr-9'
if grep -q '"sections"\|"lines"' /tmp/m110.body; then fail "untracked entry carries bill text"; fi
echo "ok: untracked entry carries no bill text"

expect "unknown bill is a problem" 404 "$(get "$api/v1/bills/us-119-hr-1")"
has "problem type" /tmp/m110.body '/problems/not-found'
expect "CORS preflight" 204 "$(curl -s -o /dev/null -w '%{http_code}' -X OPTIONS -H 'Origin: https://example.test' -H 'Access-Control-Request-Method: GET' "$api/v1/bills")"
expect "write refused" 405 "$(curl -s -o /dev/null -w '%{http_code}' -X POST "$api/v1/bills")"

if [ -n "${READ_API_LOG:-}" ] && [ -f "$READ_API_LOG" ]; then
	if grep -Eq '([0-9]{1,3}\.){3}[0-9]{1,3}' "$READ_API_LOG"; then fail "log holds an address"; fi
	echo "ok: logs hold no client addresses"
fi

if command -v go >/dev/null 2>&1 && [ -f go.mod ]; then
	go test -tags memory -count=1 ./internal/readapi/... ./internal/contract/... ./internal/httpx/... ./internal/adapters/memory/...
	echo "ok: contract tests over every route (memory), 304, cache classes, CORS, rate limits, logs"
	CONTRACT_BASE_URL=$api go test -tags memory -count=1 -run TestLiveRoutes ./internal/contract/...
	echo "ok: live contract test over every route against $api"
	go test -tags local -count=1 ./internal/adapters/fsdocs/...
	echo "ok: policy version conformance on the filesystem store"
	sh test/ci/deps.sh
	sh test/ci/size.sh
else
	echo "skip: Go toolchain or go.mod not present; code checks run in CI and in the task container"
fi
