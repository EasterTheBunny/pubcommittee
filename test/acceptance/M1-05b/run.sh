#!/bin/sh
# M1-05b acceptance (local phase): version order by stage rank, code addresses, referral collapse.
# Run inside the dev container with the repository mounted at /src (cwd = repository root or below).
# The browser criterion (bill page of H.R. 4313 without scripts, 308 for /v/rfs) is the "referral collapse"
# test of test/acceptance/M1-16/e2e/real-bills.spec.ts, run by test/acceptance/M1-16/run.sh.
set -eu
cd "$(dirname "$0")/../../.."
[ -f go.mod ] || {
	echo "skip: Go module not present (go.mod); this script runs in the dev container with the repository mounted"
	exit 0
}
fail=0
check() { # name, command...
	name=$1; shift
	if "$@" >/tmp/m1-05b-check.log 2>&1; then echo "pass: $name"; else echo "FAIL: $name"; sed 's/^/    /' /tmp/m1-05b-check.log | head -30; fail=1; fi
}

# Stage rank table: total order over the 54 govinfo codes, H.R. 3427 same-day pair, list out of BILLSTATUS order, unknown code.
check "rank table and list order" go test -count=1 -tags memory -run 'VersionRank|PublicVersions|Referrals' ./internal/data/
check "billstatus lists versions by rank" go test -count=1 ./internal/billstatus/
# Every adapter lists by rank, leaves out unknown codes and collapsed referrals, and keeps the original.
check "adapters: list order, unknown code, referral set" go test -count=1 -tags memory ./internal/adapters/memory/ ./internal/adapters/fsobject/
# Collapse rule over the exit-bill fixtures and a synthetic rfs with a changed line.
check "collapse rule" go test -count=1 -run 'Collapse|Identical' ./internal/versiondiff/
# read-api: code addresses, 308 from a collapsed code, referral note on the bill and version.
check "read-api code addresses and redirect" go test -count=1 -tags memory ./internal/readapi/
check "contract: spec valid, generated code current" sh api/check.sh
check "contract harness" go test -count=1 -tags memory ./internal/contract/...
check "admin-api and pubctl take codes" go test -count=1 -tags memory ./internal/adminapi/ ./cmd/pubctl/
check "exports and feed by code" go test -count=1 -tags memory ./internal/exports/...

# job-ingest on the exit bills: three listed versions, rfs original stored, referral note, no task planned for rfs.
check "exit bills: three versions, rfs stored and collapsed, no task for rfs" sh -c '
	set -eu
	d=$(mktemp -d); trap "rm -rf $d $d.ji $d.js" EXIT
	export DATA_DIR=$d
	go build -tags local -o "$d.ji" ./cmd/job-ingest
	go build -tags local -o "$d.js" ./cmd/job-summarize
	"$d.ji" --law >/dev/null 2>&1
	"$d.ji" --track --bill us-119-hr-4313,us-119-hr-3062,us-119-hr-3427 >/dev/null 2>&1
	for b in 4313 3062 3427; do
		dir=$d/bills/us/119/hr/$b
		[ -f "$dir/v/rfs/source.xml" ]
		cmp "$dir/v/rfs/source.xml" fixtures/bills/us-119-hr-$b/v/rfs/source.xml
		grep -q "\"code\":\"rfs\",\"into\":\"eh\",\"type\":\"Referred in Senate\"" "$dir/referrals.json"
	done
	# planned tasks: H.R. 4313 has ih, rh, eh listed, so none is planned for rfs. Since M1-09c the
	# inline explanations of superseded versions are left out of the plan and counted apart:
	# planned plus left out is still the tasks of three versions, 12, and rfs is in neither.
	"$d.js" --submit --dry-run --bill us-119-hr-4313 >"$d.out" 2>/dev/null
	planned=$(sed -n "s/.*\"planned\":\([0-9]*\).*/\1/p" "$d.out")
	total=$planned
	for n in $(grep -o "\"tasks\":[0-9]*" "$d.out" | sed "s/.*://"); do total=$((total + n)); done
	[ "$total" = 12 ]
	! grep -q "\"version\":\"rfs\"" "$d.out"
	rm -f "$d.out"
'
[ "$fail" = 0 ]
