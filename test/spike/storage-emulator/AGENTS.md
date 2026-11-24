# AGENTS.md — test/spike/storage-emulator

Purpose: throwaway spike (M1-00) that tests Cloud Storage stand-ins; handed to M1-03b as a starting point.
Skills: `tool-go`, `tool-docker`.
Owns: its own Go module (not the backend module), `storage_test.go`, `run.sh`.
Does not own: `adapters/gcs` (M1-03b).

## Rules
- Use only the public `cloud.google.com/go/storage` JSON API client, configured through `STORAGE_EMULATOR_HOST` only.
- No cloud credentials, no real bucket. Stand-in images are pinned by digest.
- Test 14 uses the testbench's `/retry_test` API and `x-retry-test-id` header; it skips elsewhere.
- Test 17 is expected to fail on both stand-ins (real-bucket-only).

## Commands
- In the dev container: `STORAGE_EMULATOR_HOST=<host:port> STAND_IN=testbench go test -race -count=1 -v ./...`
- `./run.sh` starts both stand-ins with limits and runs the suite against each.

## Gotchas
- `fake-gcs-server` needs `-public-host` and `-external-url` set to its address for resumable uploads to work from another container.
