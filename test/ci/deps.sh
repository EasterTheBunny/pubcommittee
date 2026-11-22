#!/bin/sh
# Dependency rules checked on the real build graph.
#   1. read-api links no SQL or key adapter, for every target tag.
#   2. A gcp build of any binary links no local-only adapter.
set -eu
. "$(dirname "$0")/lib.sh"

sql_keys='/adapters/(postgres|kmsgcp|localkeys)|jackc/pgx|cloud\.google\.com/go/kms'
local_only='/adapters/(fsobject|fsdocs|fsutil|localkeys|localauth)'

for tag in memory local gcp emulator; do
	if ! go list -deps -tags "$tag" ./cmd/read-api | forbid_in_list "$sql_keys"; then
		echo "FAIL: read-api ($tag) links a SQL or key adapter" >&2
		exit 1
	fi
done
for pkg in $(go list ./cmd/...); do
	if ! go list -deps -tags gcp "$pkg" | forbid_in_list "$local_only"; then
		echo "FAIL: $pkg (gcp) links a local-only adapter" >&2
		exit 1
	fi
done
echo "ok: read-api has no SQL or key adapter; gcp builds have no local-only adapter"
