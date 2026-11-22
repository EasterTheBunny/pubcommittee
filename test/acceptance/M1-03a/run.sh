#!/bin/sh
# M1-03a acceptance: local storage adapters.
# Runtime part (needs the stack): read-api runs on the local target against
# the shared data directory.
# Code part (needs the Go toolchain and the module at /src or $PWD): the
# conformance suites, write-once conflicts, two concurrent processes creating
# one path, log-plus-change failure injection, stable catalog cursors,
# cross-store visibility, and the gcp dependency exclusion.
# Environment: READ_API_URL (default http://localhost:8080).
set -eu
api=${READ_API_URL:-http://localhost:8080}

check() {
	code=$(curl -s -o /dev/null -w '%{http_code}' "$2")
	if [ "$code" != "$3" ]; then
		echo "FAIL: $1 $2 returned $code, want $3" >&2
		exit 1
	fi
	echo "ok: $1"
}

check "read-api ready on local target" "$api/readyz" 200
check "read-api ping reads the policy document" "$api/v1/ping" 200

if command -v go >/dev/null 2>&1 && [ -f go.mod ]; then
	go test -tags local -count=1 ./internal/adapters/fsobject/... ./internal/adapters/fsdocs/... ./internal/adapters/localauth/...
	echo "ok: conformance, write-once, concurrent processes, failure injection, cursors"
	go test -tags local -count=1 -run 'TestConcurrentProcessesOneWinner|TestWriterVisibleToSecondStore|TestSecondStoreSeesWrites' \
		./internal/adapters/fsobject/... ./internal/adapters/fsdocs/...
	echo "ok: writes by one process or store are visible to another"
	sh test/ci/deps.sh
else
	echo "skip: Go toolchain or go.mod not present; code checks run in CI and in the task container"
fi
