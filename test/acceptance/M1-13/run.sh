#!/bin/sh
# M1-13 acceptance: the M1 web app on real data, and the archived M0 demo.
# Runs from the repository root in a container that has Node 24 and network access to a read-api
# (any target: memory with SEED_FIXTURES=1, or local after the seed service ran).
# Environment:
#   READ_API_URL  read-api the app reads from (default http://localhost:8080)
#   APP_URL       an already running app; if unset the script builds and starts one on port 3000
#   PLAYWRIGHT=1  also run the Playwright UI tests (needs browsers; set BASE_URL and STUB_URL in
#                 the environment of the tests: see app/playwright.config.ts)
#   SKIP_DEMO=1   skip building demo/
set -eu
api=${READ_API_URL:-http://localhost:8080}
app=${APP_URL:-http://localhost:3000}
bill=us-119-hr-4127
tmp=${TMPDIR:-/tmp}/m113.$$
mkdir -p "$tmp"

fail() { echo "FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*"; }

# No mock data in the M1 app, and nothing imports the demo.
if grep -rIlE "lib/mock|demo\.svelte|from '.*/demo/" app/src app/tests 2>/dev/null | grep -q .; then
	fail "app/ references mock data or the demo"
fi
[ ! -e app/src/lib/mock ] || fail "app/src/lib/mock exists"
ok "no mock data in app/"

if [ -z "${SKIP_DEMO:-}" ]; then
	(cd demo && npm ci >/dev/null 2>&1 && npm run check >/dev/null 2>&1 && npm run build >/dev/null 2>&1) \
		|| fail "demo/ does not check and build"
	[ -f demo/build/index.html ] || fail "demo build has no index.html"
	ok "archived M0 demo checks and builds from demo/"
fi

if [ -z "${APP_URL:-}" ]; then
	(cd app && npm ci >/dev/null 2>&1 && npm run check >/dev/null 2>&1 && npm run lint >/dev/null 2>&1 \
		&& npm run build >/dev/null 2>&1) || fail "app does not check, lint, and build"
	ok "app checks, lints, and builds"
	(cd app && READ_API_URL=$api PUBLIC_API_URL=$api PORT=3000 node build >"$tmp/app.log" 2>&1 &)
	n=0
	until curl -fs "$app/policy" >/dev/null 2>&1; do
		n=$((n + 1)); [ $n -lt 30 ] || fail "app did not start"; sleep 1
	done
fi

get() { curl -s -D "$tmp/head" -o "$tmp/body" -w '%{http_code}' "$@"; }
expect() { [ "$3" = "$2" ] || fail "$1: status $3, want $2"; ok "$1"; }
has() { grep -q -- "$3" "$2" || fail "$1: missing $3"; ok "$1"; }

# Pages render server-side with real text, without scripts.
expect "home" 200 "$(get "$app/")"
expect "catalog" 200 "$(get "$app/catalog")"
has "catalog shows an official link" "$tmp/body" 'example.test'
expect "bill page" 200 "$(get "$app/bills/$bill")"
has "line anchor in server HTML" "$tmp/body" 'id="s4-l1"'
has "line text in server HTML" "$tmp/body" 'The Secretary shall publish clinic hours'
has "content class on text" "$tmp/body" 'data-content-class="official"'
has "rel=alternate to read-api" "$tmp/body" 'rel="alternate" type="application/json"'
has "no summary: designed empty state" "$tmp/body" 'No summary is published'
expect "line range address" 200 "$(get "$app/bills/$bill/v/2/s/s4/l/1-3")"
expect "version-less section address" 200 "$(get "$app/bills/$bill/s/s4")"
expect "old-version address" 200 "$(get "$app/bills/$bill/v/1/s/s4/l/2")"
has "old-version notice" "$tmp/body" 'data-notice='
expect "policy" 200 "$(get "$app/policy")"
expect "log" 200 "$(get "$app/log")"
expect "unknown bill" 404 "$(get "$app/bills/us-119-hr-0")"

# Untracked entries never show text.
expect "untracked bill" 200 "$(get "$app/bills/us-119-hr-9")"
if grep -q 'data-line=' "$tmp/body"; then fail "untracked bill shows lines"; fi
ok "untracked bill shows no text"

# Content negotiation: 303 to the read-api resource.
code=$(curl -s -o /dev/null -D "$tmp/head" -w '%{http_code}' -H 'Accept: application/json' "$app/bills/$bill/v/2/s/s4/l/1-3")
[ "$code" = 303 ] || fail "Accept: application/json answered $code, want 303"
grep -qi "^location: .*/v1/bills/$bill/v/2/s/s4/l/1-3" "$tmp/head" || fail "303 Location is not the read-api resource"
ok "Accept: application/json answers 303 to the read-api resource"

# The web host sets no cookies.
get "$app/bills/$bill" >/dev/null
if grep -qi '^set-cookie' "$tmp/head"; then fail "app set a cookie"; fi
ok "no cookies"

if [ -n "${PLAYWRIGHT:-}" ]; then
	(cd app && npx playwright test --reporter=line) || fail "Playwright tests"
	ok "Playwright UI tests"
fi
echo "M1-13 acceptance: all checks passed"
