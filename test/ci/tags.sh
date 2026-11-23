#!/bin/sh
# A build with no target tag must fail to compile (wiring_none.go).
set -eu
if go build -o /dev/null ./cmd/... >/dev/null 2>&1; then
	echo "FAIL: build with no target tag succeeded" >&2
	exit 1
fi
for tag in memory local gcp emulator; do
	go build -o /dev/null -tags "$tag" ./cmd/...
done
echo "ok: no-tag build fails; memory, local, gcp, emulator build"
