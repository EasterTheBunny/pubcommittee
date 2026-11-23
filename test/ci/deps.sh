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
# 3. read-api never links admin-api; admin-api never links read-api or any
#    package that reads individual signals or request reasons (none exist in M1).
admin_forbidden='/internal/readapi|/internal/signals|/adapters/(signals|requestreasons)'
for tag in memory local gcp emulator; do
	if ! go list -deps -tags "$tag" ./cmd/read-api | forbid_in_list '/internal/adminapi'; then
		echo "FAIL: read-api ($tag) links adminapi" >&2
		exit 1
	fi
	if ! go list -deps -tags "$tag" ./cmd/admin-api | forbid_in_list "$admin_forbidden"; then
		echo "FAIL: admin-api ($tag) links read-api or a signal reader" >&2
		exit 1
	fi
done
echo "ok: read-api has no SQL or key adapter, no admin-api; admin-api has no read-api or signal reader; gcp builds have no local-only adapter"
