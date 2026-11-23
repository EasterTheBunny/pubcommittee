#!/bin/sh
# M1-12 acceptance: administration (admin-api, pubctl), local phase.
# Run inside the dev container with the repository mounted at /src (cwd = repository root or
# below). Needs Go, curl, offline. Builds admin-api and read-api (local target) and pubctl,
# runs them on a temporary data directory, and drives them with pubctl and curl. Fake
# principals only: the local admin-api accepts X-Local-Principal to act as one.
# Cloud-phase criteria (Google IAM in dev, roster in dev) are deferred.
# Environment: DATA_DIR (optional) a data root; a fresh temporary one is used when unset.
set -eu
cd "$(dirname "$0")/../../.."
fail=0
log=$(mktemp)
check() { # name, command...
	name=$1; shift
	if "$@" >"$log" 2>&1; then echo "pass: $name"; else echo "FAIL: $name"; sed 's/^/    /' "$log" | head -30; fail=1; fi
}
tmp=$(mktemp -d)
root=${DATA_DIR:-$tmp/data}
mkdir -p "$root"
go build -tags local -o "$tmp/admin-api" ./cmd/admin-api
go build -tags local -o "$tmp/read-api" ./cmd/read-api
go build -o "$tmp/pubctl" ./cmd/pubctl
go build -tags local -o "$tmp/job-ingest" ./cmd/job-ingest

# Fixtures plus one invented war powers resolution (catalog only).
cp -r fixtures "$tmp/fixtures"
sed -e 's#<number>14</number>#<number>998</number>#' -e 's#joint-resolution/14#joint-resolution/998#' \
	-e 's#<title>[^<]*</title>#<title>Directing the removal of United States Armed Forces pursuant to section 5(c) of the War Powers Resolution (invented).</title>#' \
	fixtures/catalog/us-119-hjres-14.xml >"$tmp/fixtures/catalog/us-119-hjres-998.xml"

export DATA_DIR=$root FIXTURES_DIR=$tmp/fixtures INGEST_SOURCE=fixtures
PORT=18081 METRICS_PORT=19081 "$tmp/admin-api" >"$tmp/admin.log" 2>&1 &
apid=$!
PORT=18082 METRICS_PORT=19082 "$tmp/read-api" >"$tmp/read.log" 2>&1 &
rpid=$!
trap 'kill $apid $rpid 2>/dev/null || true' EXIT
for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do
	curl -sf http://127.0.0.1:18081/healthz >/dev/null && curl -sf http://127.0.0.1:18082/healthz >/dev/null && break
	sleep 0.5
done
adm=http://127.0.0.1:18081
pc() { "$tmp/pubctl" --url "$adm" "$@"; }
code() { curl -s -o /dev/null -w '%{http_code}' "$@"; } # status of a request

check "go tests: admin handler (authorisation, refusals, log atomicity, roster), pubctl" \
	go test -tags memory -count=1 ./internal/adminapi/... ./cmd/pubctl/...
check "dependency rules: read-api has no admin-api, admin-api has no read-api" sh test/ci/deps.sh

check "publish selection policy v1 through pubctl" sh -c "'$tmp/pubctl' --url $adm selection-policy publish --builtin v1 --reason 'initial policy' | grep -q '\"version\": 1'"
check "track without a reason is refused" sh -c "! '$tmp/pubctl' --url $adm bills track us-119-hr-3062 2>/dev/null"
check "track a catalog bill with a reason" sh -c "'$tmp/pubctl' --url $adm bills track us-119-hr-3062 --reason 'pilot seed: acceptance' >'$tmp/track.json'"
check "the bill shows the administrator reason" sh -c "grep -rq administrator '$root/catalog'"
check "its text is ingested" sh -c "[ \$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:18082/v1/bills/us-119-hr-3062/v/1) = 200 ]"
check "its bill document lists the reason" sh -c "curl -sf http://127.0.0.1:18082/v1/bills/us-119-hr-3062 | grep -q administrator"
check "the public log shows the operator's public name and the reason" sh -c "curl -sf http://127.0.0.1:18082/v1/moderation-log >'$tmp/log.json' && grep -q 'Local Operator' '$tmp/log.json' && grep -q 'pilot seed: acceptance' '$tmp/log.json'"
check "the public log holds no principal" sh -c "! grep -q 'local-operator' '$tmp/log.json'"

check "tracking a war powers resolution is refused with the policy reason" sh -c "! '$tmp/pubctl' --url $adm bills track us-119-hjres-998 --reason try 2>'$tmp/war.err'; grep -qi 'war powers' '$tmp/war.err'"

# Authorisation with fake principals.
check "a stranger is refused" sh -c "[ \$(curl -s -o /dev/null -w '%{http_code}' -H 'X-Local-Principal: user:fake-stranger' $adm/admin/v1/jobs) = 403 ]"
check "the deploy account is refused on any other route" sh -c "[ \$(curl -s -o /dev/null -w '%{http_code}' -H 'X-Local-Principal: local-deploy' $adm/admin/v1/jobs) = 403 ]"
check "an operator is refused on the roster route" sh -c "[ \$(curl -s -o /dev/null -w '%{http_code}' -X POST -d '{}' $adm/admin/v1/operators/roster) = 403 ]"

# Roster: bootstrap, change, repeat, bad approver.
roster() { # approver, operators-json
	curl -s -X POST -H 'X-Local-Principal: local-deploy' -d "{\"reason\":\"acceptance roster\",\"approver\":\"$1\",\"operators\":$2}" $adm/admin/v1/operators/roster
}
r1=$(roster "" '[{"id":"op-1","name":"Operator One"},{"id":"op-2","name":"Operator Two"}]')
echo "$r1" | grep -q '"bootstrap":true' && echo "pass: bootstrap roster labelled" || { echo "FAIL: bootstrap: $r1"; fail=1; }
r2=$(roster "op-1" '[{"id":"op-1","name":"Operator One"},{"id":"op-3","name":"Operator Three"}]')
[ "$(echo "$r2" | grep -o 'roster-' | wc -l)" -eq 2 ] && echo "pass: one added and one removed write exactly two entries" || { echo "FAIL: change: $r2"; fail=1; }
r3=$(roster "op-1" '[{"id":"op-1","name":"Operator One"},{"id":"op-3","name":"Operator Three"}]')
echo "$r3" | grep -q '"written":\[\]' && echo "pass: posting it again writes none" || { echo "FAIL: repeat: $r3"; fail=1; }
r4=$(curl -s -o /dev/null -w '%{http_code}' -X POST -H 'X-Local-Principal: local-deploy' -d '{"reason":"x","approver":"op-2","operators":[{"id":"op-1","name":"Operator One"}]}' $adm/admin/v1/operators/roster)
[ "$r4" = 403 ] && echo "pass: an approver not on the prior roster is refused" || { echo "FAIL: approver status $r4"; fail=1; }

check "ingest failures and jobs answer" sh -c "pc() { '$tmp/pubctl' --url $adm \"\$@\"; }; pc ingest failures >/dev/null && pc jobs >/dev/null"
check "reingest re-checks stored originals" sh -c "'$tmp/pubctl' --url $adm bills reingest us-119-hr-3062 --reason 'verify hashes' | grep -q '\"changed\": true'"
check "generations report unavailable until M1-09" sh -c "! '$tmp/pubctl' --url $adm generations create --task summary --bill us-119-hr-3062 --version 1 --reason r 2>'$tmp/gen.err'; grep -q 501 '$tmp/gen.err'"

echo "deferred (cloud phase): Google IAM refusal of a non-operator principal in dev; roster route in dev"
[ "$fail" -eq 0 ] || { echo "M1-12 acceptance: FAILED"; exit 1; }
echo "M1-12 acceptance: passed (local phase)"
