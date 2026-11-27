#!/bin/sh
# M1-09c acceptance (local phase): inline explanations on the latest version only. fakegen only, no model call.
# Run inside the dev container with the repository mounted at /src (cwd = repository root or below).
# Cloud phase (not run here): the dry run in dev for H.R. 4313 shows the reduced task count before any paid run.
set -eu
cd "$(dirname "$0")/../../.."
[ -f go.mod ] || {
	echo "skip: Go module not present (go.mod); this script runs in the dev container with the repository mounted"
	exit 0
}
fail=0
check() { # name, command...
	name=$1; shift
	if "$@" >/tmp/m1-09c-check.log 2>&1; then echo "pass: $name"; else echo "FAIL: $name"; sed 's/^/    /' /tmp/m1-09c-check.log | head -30; fail=1; fi
}

check "planning rule, left-out counts, stored outputs stay, single version, --explain-version" \
	go test -count=1 -tags memory -run 'Latest|Superseded|SingleVersion|ExplainVersion|SubmitCollect' ./internal/generate/
check "--explain-version flag rules" go test -count=1 -tags memory -run 'ExplainVersion' ./cmd/job-summarize/

check "H.R. 4313 dry run: explanations for eh only, left out for ih and rh; --explain-version ih; zz refused" sh -c '
	set -eu
	d=$(mktemp -d); trap "rm -rf $d $d.ji $d.js" EXIT
	export DATA_DIR=$d
	go build -tags local -o "$d.ji" ./cmd/job-ingest
	go build -tags local -o "$d.js" ./cmd/job-summarize
	"$d.ji" --law >/dev/null 2>&1
	"$d.ji" --track --bill us-119-hr-4313 >/dev/null 2>&1
	"$d.js" --submit --dry-run --bill us-119-hr-4313 2>/dev/null >"$d.out"
	grep -q "\"explanations_left_out\"" "$d.out"
	grep -q "\"version\": *\"ih\"" "$d.out"
	grep -q "\"version\": *\"rh\"" "$d.out"
	! grep -q "\"version\": *\"eh\"" "$d.out"
	"$d.js" --submit --dry-run --bill us-119-hr-4313 --explain-version ih 2>/dev/null >"$d.out2"
	! grep -q "\"version\": *\"ih\"" "$d.out2"
	grep -q "\"version\": *\"rh\"" "$d.out2"
	! "$d.js" --submit --dry-run --bill us-119-hr-4313 --explain-version zz >/dev/null 2>&1
	rm -f "$d.out" "$d.out2"
'
exit $fail
