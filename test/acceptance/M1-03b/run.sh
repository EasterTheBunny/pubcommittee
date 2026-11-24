#!/bin/sh
# M1-03b acceptance: cloud storage adapters (adapters/gcs, adapters/firestore).
# Needs the Go toolchain and the module at $PWD, and both emulators from the
# emulator profile of compose.yaml:
#   STORAGE_EMULATOR_HOST   Cloud Storage stand-in, for example gcs-emulator:4443
#   FIRESTORE_EMULATOR_HOST Firestore emulator, for example firestore-emulator:8080
# Checks: every conformance suite against the emulators, write-once conflicts,
# stable catalog cursors for a filter plus keyword query, log-plus-change
# failure injection, injected-transport transient errors, the emulator and gcp
# builds, and the gcp dependency exclusions.
# Not run here (cloud phase C1): tests against a real bucket and database. To run
# them the operator sets PC_GCS_TEST_BUCKET (an existing dev test bucket) with
# Application Default Credentials and runs:
#   go test -tags memory -count=1 -run TestRealBucket -v ./internal/adapters/gcs/
set -eu

for v in STORAGE_EMULATOR_HOST FIRESTORE_EMULATOR_HOST; do
	eval "val=\${$v:-}"
	if [ -z "$val" ]; then
		echo "FAIL: $v is not set; start the emulator profile (docker compose --profile emulator up -d)" >&2
		exit 1
	fi
done
export PC_REQUIRE_EMULATORS=1

go test -tags memory -race -count=1 ./internal/adapters/gcs/... ./internal/adapters/firestore/... ./internal/cloudports/...
echo "ok: conformance suites, spike behaviours, transient errors, and cloud adapter tests pass against the emulators"

go test -tags memory -count=1 -v -run 'TestSecondWriteIsConflict|TestConcurrentCreateOneWinner' ./internal/adapters/gcs/ | grep -E '^--- PASS'
echo "ok: a second write to an immutable path fails with ErrConflict"

go test -tags memory -count=1 -v -run 'TestQueryCatalogFilterAndKeyword' ./internal/adapters/firestore/ | grep -E '^--- PASS'
echo "ok: filter plus keyword catalog query returns stable cursors"

go test -tags memory -count=1 -v -run 'TestApplyLoggedLogFailureLeavesNothing' ./internal/adapters/firestore/ | grep -E '^--- PASS'
echo "ok: a failed log write leaves no change"

for tag in gcp emulator; do
	go build -tags "$tag" -o /dev/null ./cmd/read-api ./cmd/job-ingest ./cmd/job-exports ./cmd/job-summarize ./cmd/admin-api ./cmd/accuracy-report
done
echo "ok: every service and job builds for gcp and emulator"
sh test/ci/deps.sh
