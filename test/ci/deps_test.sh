#!/bin/sh
# Tests of the check itself: a deliberate violation must be reported.
set -eu
. "$(dirname "$0")/lib.sh"

sql_keys='/adapters/(postgres|kmsgcp|localkeys)|jackc/pgx|cloud\.google\.com/go/kms'
local_only='/adapters/(fsobject|fsdocs|localkeys|localauth)'

expect_violation() {
	if printf '%s\n' "$2" | forbid_in_list "$1" >/dev/null; then
		echo "FAIL: violation not detected: $2" >&2
		exit 1
	fi
}

expect_violation "$sql_keys" github.com/easterthebunny/pubcommittee/internal/adapters/postgres
expect_violation "$sql_keys" github.com/jackc/pgx/v5
expect_violation "$sql_keys" github.com/easterthebunny/pubcommittee/internal/adapters/kmsgcp
expect_violation "$sql_keys" github.com/easterthebunny/pubcommittee/internal/adapters/localkeys
expect_violation "$local_only" github.com/easterthebunny/pubcommittee/internal/adapters/fsdocs
expect_violation "$local_only" github.com/easterthebunny/pubcommittee/internal/adapters/fsobject

printf '%s\n' fmt github.com/easterthebunny/pubcommittee/internal/adapters/memory | forbid_in_list "$sql_keys" >/dev/null
echo "ok: dependency check detects violations and passes clean lists"
