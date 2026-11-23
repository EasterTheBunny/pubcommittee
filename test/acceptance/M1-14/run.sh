#!/bin/sh
# M1-14 acceptance: installable app on real data, offline reading, bounded caches.
# Runs from the repository root in a container that has Node 24 and the Playwright browsers
# (or with BASE_URL pointing at an app that Playwright can reach as localhost: service workers
# need a secure context, and only localhost is one without HTTPS).
# Environment:
#   READ_API_URL  read-api the app reads from (default http://localhost:8080)
#   BASE_URL      an already running app (default http://localhost:3000). If nothing answers
#                 there, the script builds and starts one on port 3000.
# Cloud phase (deferred): install on a current mobile browser and a desktop browser in staging.
set -eu
[ -f app/package.json ] || {
	echo "skip: app/ not present (app/package.json); this script runs in a Node container with the repository mounted"
	exit 0
}
api=${READ_API_URL:-http://localhost:8080}
export BASE_URL=${BASE_URL:-http://localhost:3000}
tmp=${TMPDIR:-/tmp}/m114.$$
mkdir -p "$tmp"

fail() { echo "FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*"; }

# Static checks on what ships.
grep -q "M0\|mock" app/static/manifest.webmanifest && fail "manifest still mentions M0 or mock data"
if grep -nE "In M0|mock bills|demo-updates" app/src/lib/placeholders.ts app/src/service-worker/*.ts; then
	fail "M0-era wording remains in the placeholders or the service worker"
fi
ok "no M0 wording in manifest, placeholders, or service worker"
grep -q "no-store" app/src/service-worker/policy.ts || fail "policy does not respect Cache-Control"
grep -q "v1\\\\/me" app/src/service-worker/policy.ts || fail "credentialed routes are not excluded"
ok "credentialed routes excluded and Cache-Control respected"

(cd app && npm ci >/dev/null 2>&1 && npm run check >/dev/null 2>&1 && npm run lint >/dev/null 2>&1 \
	&& npm run build >/dev/null 2>&1) || fail "app does not check, lint, and build"
ok "app checks, lints, and builds"
[ -f app/build/client/service-worker.js ] || fail "build has no service-worker.js"
ok "service worker is built"

if ! (command -v curl >/dev/null && curl -fs "$BASE_URL/policy" >/dev/null 2>&1) \
	&& ! node -e "fetch('$BASE_URL/policy').then(r=>process.exit(r.ok?0:1),()=>process.exit(1))"; then
	(cd app && READ_API_URL=$api PUBLIC_API_URL=$api PORT=3000 node build >"$tmp/app.log" 2>&1 &)
	n=0
	until node -e "fetch('$BASE_URL/policy').then(r=>process.exit(r.ok?0:1),()=>process.exit(1))"; do
		n=$((n + 1)); [ $n -lt 30 ] || fail "app did not start"; sleep 1
	done
fi

# Manifest, installability errors from the browser, offline version page, offline label, bounds.
(cd app && npx playwright test tests/pwa.spec.ts tests/sw-policy.spec.ts --project=desktop --reporter=line) \
	|| fail "installability and offline tests"
ok "installability, offline version page, offline catalog label, eviction policy"
echo "M1-14 acceptance: all local-phase checks passed (cloud phase: mobile and staging install deferred)"
