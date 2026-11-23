#!/bin/sh
# M1-08 acceptance (local phase): version comparison, stale classification, compare route.
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
	if "$@" >/tmp/m1-08-check.log 2>&1; then echo "pass: $name"; else echo "FAIL: $name"; sed 's/^/    /' /tmp/m1-08-check.log | head -30; fail=1; fi
}

# Every old anchor of every exit-bill pair has a class and a consistent mapping; the real
# ih to rh change of hr-4313 shows as moved lines.
check "exit bills: every old anchor classified" go test -count=1 -run 'TestExitBills|TestIdentical' ./internal/versiondiff/
# Renumbered identical subsection or section is moved; whitespace is no change; punctuation is.
check "classification rules and determinism" go test -count=1 -run 'Renumbered|Whitespace|Punctuation|Split|Deleted|Deterministic' ./internal/versiondiff/
# Old-address lookup ("changed since"): class and new location of a range.
check "range lookup gives class and new location" go test -count=1 -run 'Lookup|MovedRange' ./internal/versiondiff/
# Stored comparisons: write-once conformance on memory and local object store.
check "comparison storage conformance (memory)" go test -count=1 -tags memory -run Comparisons ./internal/adapters/memory/
check "comparison storage conformance (fsobject)" go test -count=1 -tags memory -run Comparisons ./internal/adapters/fsobject/
# The compare route serves the stored comparison, 404 otherwise.
check "compare route" go test -count=1 -tags memory -run CompareVersions ./internal/readapi/
# changed_since on pinned old versions (additive contract change), served from the stored comparison.
check "changed_since on section and line answers of an older version" go test -count=1 -tags memory -run ChangedSince ./internal/readapi/
check "contract: spec valid, generated code current" sh api/check.sh
check "contract harness" go test -count=1 -tags memory ./internal/contract/...
# job-ingest wiring (part 2): a pull of the tracked exit bills stores the comparisons of every consecutive
# pair and of every older version against the latest; --analyze rebuilds them over stored data.
check "job-ingest: comparisons stored for every exit-bill pair, rebuilt over stored data" sh -c '
	set -eu
	d=$(mktemp -d); trap "rm -rf $d" EXIT
	export DATA_DIR=$d
	go build -tags local -o "$d.ji" ./cmd/job-ingest
	"$d.ji" --law >/dev/null 2>&1
	"$d.ji" --track --bill us-119-hr-4313,us-119-hr-3062,us-119-hr-3427 2>/dev/null >"$d.out"
	# 4 versions each: 3 consecutive pairs plus 2 older-versus-latest pairs
	[ "$(grep -c "\"comparisons\": 5" "$d.out")" = 3 ]
	count() { find "$d/bills" -path "*/compare/*" -name "*.json" ! -name "*.meta.json" | wc -l; }
	[ "$(count)" = 15 ]
	find "$d/bills" -type d -name compare -exec rm -rf {} +
	[ "$(count)" = 0 ]
	"$d.ji" --analyze 2>/dev/null >"$d.out"
	[ "$(count)" = 15 ]
	rm -f "$d.ji" "$d.out"
'
check "job-ingest: pipeline steps" go test -count=1 -tags memory -run Analysis ./internal/ingest/
exit $fail
