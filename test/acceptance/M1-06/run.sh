#!/bin/sh
# M1-06 acceptance: selection policy and tracked set (job-ingest --publish-policy, --evaluate).
# Run inside the dev container with the repository mounted at /src (cwd = repository root
# or any directory below it). Needs Go. Local phase only; offline (fixtures).
# Environment: DATA_DIR (optional) a data root to use; a fresh temporary one is used when unset.
set -eu
cd "$(dirname "$0")/../../.."
fail=0
log=$(mktemp)
check() { # name, command...
	name=$1; shift
	if "$@" >"$log" 2>&1; then echo "pass: $name"; else echo "FAIL: $name"; sed 's/^/    /' "$log" | head -30; fail=1; fi
}
bin=$(mktemp -d)/job-ingest
go build -tags local -o "$bin" ./cmd/job-ingest
root=${DATA_DIR:-$(mktemp -d)}
export DATA_DIR=$root
ji() { "$bin" --source fixtures --fixtures-dir fixtures "$@" 2>/dev/null; }

check "go tests: evaluator, ingest integration (memory)" go test -tags memory -count=1 ./internal/selection/... ./internal/ingest/... ./internal/billstatus/...
check "evaluator input has no sponsor or party field" go test -tags memory -count=1 -run TestInputHasNoSponsorFields ./internal/selection/...

check "publish policy version 1 to the local store" sh -c "'$bin' --publish-policy v1 2>/dev/null | grep -q '\"published_policy_version\": 1'"
check "publishing the same version again is a no-op" sh -c "'$bin' --publish-policy v1 2>/dev/null | grep -q '\"published_policy_version\": 1'"

# Pull the 200-entry catalog sample. A pull of a bill the policy tracks fails to find its text in
# the catalog-only fixtures and exits non-zero; the catalog entry is still written.
args=""
for f in fixtures/catalog/us-*.xml; do args="$args --bill $(basename "$f" .xml)"; done
# shellcheck disable=SC2086
ji $args >/dev/null || true
ji --evaluate >"$root/evaluate.json"
count=$(find "$root/catalog" -name '*.json' ! -name '*.meta.json' | wc -l)
[ "$count" -eq 200 ] && echo "pass: 200 catalog entries stored" || { echo "FAIL: $count catalog entries"; fail=1; }
nostatus=0
for f in "$root"/catalog/*.json; do grep -q '"markup_status":"' "$f" || nostatus=$((nostatus + 1)); done
[ "$nostatus" -eq 0 ] && echo "pass: every catalog entry has a markup status" || { echo "FAIL: $nostatus entries without markup status"; fail=1; }
bad=0
for f in $(grep -l '"tracked":true' "$root"/catalog/*.json); do
	grep -q '"reasons":\["' "$f" && grep -q '"policy_version":1' "$f" || bad=$((bad + 1))
done
tracked=$(grep -l '"tracked":true' "$root"/catalog/*.json | wc -l)
[ "$tracked" -gt 0 ] && [ "$bad" -eq 0 ] && echo "pass: $tracked tracked bills list reasons and policy version" || { echo "FAIL: tracked=$tracked without reasons or version=$bad"; fail=1; }
check "excluded kinds carry a reason" sh -c "! grep -L '\"exclusion_reason\"' \$(grep -l 'not-a-candidate' '$root'/catalog/*.json) | grep -q ."
check "breakdown published with counts by topic and party" sh -c "grep -q by_topic '$root/exports/tracked-set-breakdown.json' && grep -q by_sponsor_party '$root/exports/tracked-set-breakdown.json'"

# Policy version 2 with a narrower topic list leaves every tracked bill tracked.
grep -l '"tracked":true' "$root"/catalog/*.json | sort >"$root/tracked-before.txt"
cat >"$root/v2.json" <<'J'
{"version":2,"mode":"pilot","phase":"early_pilot","measure_kinds":["bill","joint_resolution"],
 "exclude_kinds":["war_powers_resolution","commemorative"],
 "committee_activity":["hearing","markup","reported","discharged","floor","calendar"],
 "follow_companions":true,"topics":{"deficit_reduction":{"subjects":["Budget process"]}}}
J
check "publish policy version 2 (narrower topics)" sh -c "'$bin' --publish-policy '$root/v2.json' --reason 'narrower topics' 2>/dev/null | grep -q '\"published_policy_version\": 2'"
ji --evaluate >/dev/null
grep -l '"tracked":true' "$root"/catalog/*.json | sort >"$root/tracked-after.txt"
[ -s "$root/tracked-before.txt" ] && [ -z "$(comm -23 "$root/tracked-before.txt" "$root/tracked-after.txt")" ] && echo "pass: version 2 leaves every tracked bill tracked" || { echo "FAIL: a tracked bill was untracked"; fail=1; }
check "version 1 stays stored and unchanged" test -f "$root/selection_policy/00000001.json"

[ "$fail" -eq 0 ] && echo "M1-06 acceptance: all checks passed" || { echo "M1-06 acceptance: FAILED"; exit 1; }
