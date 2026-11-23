#!/bin/sh
# M1-09 acceptance (local phase, stage L0): AI generation on fakegen. No model call is made:
# the live criteria (summaries from the operator's key, grader runs, operator sample grading) are
# reported blocked on operator approval and are not run here.
# Run inside the dev container with the repository mounted at /src (cwd = repository root or below).
set -eu
cd "$(dirname "$0")/../../.."
[ -f go.mod ] || {
	echo "skip: Go module not present (go.mod); this script runs in the dev container with the repository mounted"
	exit 0
}
fail=0
check() { # name, command...
	name=$1; shift
	if "$@" >/tmp/m1-09-check.log 2>&1; then echo "pass: $name"; else echo "FAIL: $name"; sed 's/^/    /' /tmp/m1-09-check.log | head -30; fail=1; fi
}

check "store conformance (memory, fsobject)" go test -count=1 -tags memory -run Generated ./internal/adapters/memory/ ./internal/adapters/fsobject/
check "Generator conformance on fakegen" go test -count=1 -tags memory ./internal/adapters/fakegen/
check "claude adapter against a fake HTTP server (request shape, stop reasons, batches)" go test -count=1 -tags memory ./internal/adapters/claude/
check "submit and collect end to end; re-run submits nothing; invalid anchor and invented number not published" \
	go test -count=1 -tags memory -run 'SubmitCollect|InvalidAnchor|Refusal' ./internal/generate/
check "validators and planted errors" go test -count=1 -tags memory -run 'FactFidelity|Amendment|Neutrality|Plant|Good' ./internal/generate/
check "accuracy statistics, sample, CRS" go test -count=1 -tags memory ./internal/accuracy/
check "read-api summary and changes routes" go test -count=1 -tags memory -run 'Generated|BillSummaryFrom|VersionChangesFrom' ./internal/readapi/
check "CI never calls a model: live generator is refused without the switch" go test -count=1 -tags memory ./cmd/job-summarize/ ./cmd/accuracy-report/
check "contract: spec valid, generated code current" sh api/check.sh
check "contract harness" go test -count=1 -tags memory ./internal/contract/...
# Local run on the stored exit bills with fakegen: submit, collect, a re-run submits nothing.
check "job-summarize on the exit bills (fakegen): submit, collect, re-run" sh -c '
	set -eu
	d=$(mktemp -d); trap "rm -rf $d $d.ji $d.js $d.ar" EXIT
	export DATA_DIR=$d
	go build -tags local -o "$d.ji" ./cmd/job-ingest
	go build -tags local -o "$d.js" ./cmd/job-summarize
	"$d.ji" --law >/dev/null 2>&1
	"$d.ji" --track --bill us-119-hr-4313,us-119-hr-3062,us-119-hr-3427 >/dev/null 2>&1
	bills=us-119-hr-4313,us-119-hr-3062,us-119-hr-3427
	"$d.js" --submit --bill $bills 2>/dev/null >"$d.out"
	grep -q "\"submitted\": [1-9]" "$d.out"
	"$d.js" --collect 2>/dev/null >"$d.out"
	grep -q "\"ready\": " "$d.out"
	! grep -q "unverified\|invalid\|refusal" "$d.out"
	"$d.js" --submit --bill $bills 2>/dev/null >"$d.out"
	[ "$(grep -c "\"submitted\": 0" "$d.out")" = 3 ]
	# the live generator is refused without the switch
	! "$d.js" --generator claude --collect >/dev/null 2>&1
	go build -tags local -o "$d.ar" ./cmd/accuracy-report
	"$d.ar" --out "$d.acc" >/dev/null 2>&1
	grep -q "Planted errors" "$d.acc/report.md"
	grep -q "pending" "$d.acc/report.md"
	rm -f "$d.out"; rm -rf "$d.acc"
'
exit $fail
