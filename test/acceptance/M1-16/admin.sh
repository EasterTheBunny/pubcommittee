#!/bin/sh
# M1-16 local gate: administration is publicly logged. Runs in the dev container (Go, curl), with the
# repository mounted at /src and DATA_DIR the SAME data root the read-api container serves, so the
# log entries are read back from another container (M1-03a cross-container visibility).
# Starts admin-api (local target; loopback requests act as the fixed local operator) and drives it
# with pubctl. Placeholder reasons only. Leaves the catalog as it found it (track, then untrack).
# Environment: DATA_DIR (required), READ_API_URL, APP_URL.
set -eu
cd "$(dirname "$0")/../../.."
: "${DATA_DIR:?DATA_DIR is required}"
api=${READ_API_URL:-http://read-api:8080}
app=${APP_URL:-http://app:3000}
bin=$(mktemp -d)
go build -tags local -o "$bin/admin-api" ./cmd/admin-api
go build -o "$bin/pubctl" ./cmd/pubctl
export DATA_DIR INGEST_SOURCE=fixtures FIXTURES_DIR=fixtures
PORT=18081 METRICS_PORT=19081 "$bin/admin-api" >"$bin/admin.log" 2>&1 &
pid=$!
trap 'kill $pid 2>/dev/null || true' EXIT
for _ in $(seq 1 30); do curl -sf http://127.0.0.1:18081/healthz >/dev/null && break; sleep 0.5; done
pc() { "$bin/pubctl" --url http://127.0.0.1:18081 "$@"; }
fail=0
ok() { echo "PASS $1"; }
bad() { echo "FAIL $1"; fail=1; }
tag="m1-16-$(date -u +%s)"

pc bills track us-119-hjres-1 --reason "acceptance $tag: select a catalog bill" >/dev/null 2>&1 && ok "pubctl selects a catalog bill with a reason" || bad "track"
pc bills track us-119-hjres-1 2>/dev/null && bad "track without a reason is refused" || ok "track without a reason is refused"
pc bills reingest us-119-hr-3427 --reason "acceptance $tag: re-run ingestion" >/dev/null 2>&1 && ok "pubctl re-runs ingestion with a reason" || bad "reingest"
pc bills untrack us-119-hjres-1 --reason "acceptance $tag: undo the selection" >/dev/null 2>&1 && ok "pubctl reverses the selection with a reason" || bad "untrack"
pc jobs >/dev/null 2>&1 && pc ingest failures >/dev/null 2>&1 && ok "jobs and ingest failures answer" || bad "jobs"

# The log is newest first by day; within a day, generation entries (full timestamp) sort before
# administrator entries (day only), so read every page.
log=""
cur=""
for _ in $(seq 1 20); do
	pg=$(curl -s "$api/v1/moderation-log?limit=200${cur:+&cursor=$cur}")
	log="$log$pg"
	cur=$(echo "$pg" | sed -n 's/.*"next":"\([^"]*\)".*/\1/p')
	[ -n "$cur" ] || break
done
for what in "select a catalog bill" "re-run ingestion" "undo the selection"; do
	echo "$log" | grep -q "acceptance $tag: $what" && ok "read-api (another container) publishes: $what" || bad "log lacks: $what"
done
echo "$log" | grep -qiE '@|principal|token' && bad "log leaks a principal or token" || ok "the public log holds no principal or token"
# The web app's /log page, following "Older entries" links like a reader would (no scripts).
found=0
href="/log"
for _ in $(seq 1 20); do
	page=$(curl -s "$app$href")
	if echo "$page" | grep -q "acceptance $tag"; then found=1; break; fi
	href=$(echo "$page" | sed -n 's/.*href="\([^"]*\)" rel="next".*/\1/p' | sed 's/&amp;/\&/g' | head -1)
	[ -n "$href" ] || break
	case $href in ./*) href="/${href#./}" ;; \?*) href="/log$href" ;; esac
done
[ "$found" = 1 ] && ok "the web app's /log page shows the actions with their reasons (following older-entries links)" || bad "/log page lacks the actions"
code=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$api/admin/v1/bills/us-119-hjres-1/track")
[ "$code" = 404 ] || [ "$code" = 405 ] && ok "read-api exposes no admin route ($code)" || bad "read-api answered $code on an admin route"
[ "$fail" -eq 0 ] || { echo "admin checks: FAILED"; exit 1; }
echo "admin checks: passed"
