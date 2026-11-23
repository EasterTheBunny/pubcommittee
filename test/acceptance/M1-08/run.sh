#!/bin/sh
# M1-08 acceptance (local phase): version comparison, stale classification, compare route.
# Run inside the dev container with the repository mounted at /src (cwd = repository root or below).
set -eu
cd "$(dirname "$0")/../../.."
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
exit $fail
