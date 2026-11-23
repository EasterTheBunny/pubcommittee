#!/bin/sh
# M1-11 acceptance (local phase): exports for every tracked version, the change feed, the
# well-known machine-access notice and crawler directives.
# Run inside the dev container with the repository mounted at /src (cwd = repository root or below).
# Code part: unit, conformance and route tests (memory), dependency rules, size budget.
# Local-stack part (needs only the Go toolchain): pulls two example bills into a scratch data
# directory, runs job-exports, starts read-api on the local target over it and reads the feed and
# exports with curl. Environment: M1_11_PORT (default 18111), NOTICE_URL (optional: a running UI
# whose /.well-known/machine-access.json and /robots.txt are checked as well).
set -eu
cd "$(dirname "$0")/../../.."
port=${M1_11_PORT:-18111}
api=http://127.0.0.1:$port
data=$(mktemp -d)
pid=
cleanup() { [ -z "$pid" ] || kill "$pid" 2>/dev/null || true; rm -rf "$data" /tmp/m111-*; }
trap cleanup EXIT
fail() { echo "FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*"; }
get() { curl -s -D /tmp/m111-head -o /tmp/m111-body -w '%{http_code}' "$@"; }
has() { grep -q -- "$2" "$1" || fail "$1 lacks $2"; }

go test -count=1 -tags memory ./internal/exports/ ./internal/data/... ./internal/readapi/ ./internal/contract/ \
	./internal/adapters/memory/ ./internal/adapters/fsobject/ ./internal/telemetry/ >/tmp/m111-test.log 2>&1 \
	|| { cat /tmp/m111-test.log; fail "go tests"; }
ok "unit, conformance and route tests"
sh test/ci/deps.sh >/dev/null && sh test/ci/tags.sh >/dev/null || fail "dependency rules or build tags"
ok "dependency rules and tag builds"

export DATA_DIR=$data INGEST_SOURCE=fixtures FIXTURES_DIR=fixtures
go build -tags local -o /tmp/m111-ingest ./cmd/job-ingest
go build -tags local -o /tmp/m111-exports ./cmd/job-exports
go build -tags local -o /tmp/m111-api ./cmd/read-api
/tmp/m111-ingest --publish-policy v1 >/dev/null 2>&1
/tmp/m111-ingest --bill us-119-hr-4313 --track >/dev/null 2>&1 || fail "pull of the example bill"
/tmp/m111-exports >/tmp/m111-run1.json 2>/dev/null || fail "job-exports run"
ok "pull, then job-exports"
/tmp/m111-exports >/tmp/m111-run2.json 2>/dev/null || fail "job-exports second run"
has /tmp/m111-run2.json '"feed_new": 0'
ok "second run adds no feed entries (idempotent per window)"

PORT=$port METRICS_PORT=$((port + 1)) OTEL_SERVICE_NAME=read-api RATE_LIMITS="text=0,live=0,bulk=0,discovery=0" \
	/tmp/m111-api >/tmp/m111-api.log 2>&1 &
pid=$!
for _ in 1 2 3 4 5 6 7 8 9 10; do [ "$(get "$api/healthz")" = 200 ] && break; sleep 1; done

[ "$(get "$api/v1/exports?limit=200")" = 200 ] || fail "exports index"
has /tmp/m111-body '"licence":"public-domain"'
has /tmp/m111-body '"licence":"CC0-1.0"'
has /tmp/m111-body '"as_of"'
has /tmp/m111-body '"content_class"'
versions=$(grep -o '"id":"bill-text\.[^"]*"' /tmp/m111-body | wc -l)
[ "$versions" -ge 1 ] || fail "no bill-text export"
ok "export index states as-of, licence and content class; $versions bill-text export(s)"
for href in $(grep -o '"href":"/v1/exports/[^"]*"' /tmp/m111-body | cut -d'"' -f4); do
	[ "$(get "$api$href")" = 200 ] || fail "export $href"
done
ok "every indexed export is served"
has /tmp/m111-head 'max-age=900'
[ "$(get "$api/v1/exports/bill-text.us-119-hr-4313.ih.json")" = 200 ] || fail "bill text export"
has /tmp/m111-body '"licences"'

[ "$(get "$api/v1/feed?limit=200")" = 200 ] || fail "feed"
has /tmp/m111-body '"kind":"version"'
has /tmp/m111-body '"kind":"bill-tracked"'
has /tmp/m111-body '"kind":"policy"'
ok "feed lists the new versions, the tracked bill and the policy"
for addr in $(grep -o '"address":"[^"]*"' /tmp/m111-body | cut -d'"' -f4 | sed 's/\\u0026/\&/g'); do
	[ "$(get "$api$addr")" = 200 ] || fail "feed address $addr"
done
ok "every feed entry reaches its item by canonical address"
since=$(date -u +%Y-%m-%dT00:00:00Z)
[ "$(get "$api/v1/feed?since=$since")" = 200 ] && has /tmp/m111-body '"kind":"version"'
[ "$(get "$api/v1/feed?since=2999-01-01T00:00:00Z")" = 200 ] && ! grep -q '"kind"' /tmp/m111-body || fail "since in the future"
ok "feed since a time"

[ "$(get "$api/robots.txt")" = 200 ] && has /tmp/m111-body 'Allow: /'
[ "$(get "$api/v1")" = 200 ] && has /tmp/m111-body '/.well-known/machine-access.json'
ok "API crawler directives and the discovery link to the notice"
if [ -n "${NOTICE_URL:-}" ]; then
	[ "$(get "$NOTICE_URL/.well-known/machine-access.json")" = 200 ] || fail "notice"
	has /tmp/m111-body '"automated_submission": "prohibited"'
	has /tmp/m111-body 'Untrusted data'
	[ "$(get "$NOTICE_URL/robots.txt")" = 200 ] && has /tmp/m111-body 'Disallow: /sign-in'
	ok "UI notice and robots.txt reachable without sign-in"
else
	echo "skip: UI notice (set NOTICE_URL; covered by app/tests/machine-access.spec.ts)"
fi
