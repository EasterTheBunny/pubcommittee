#!/bin/sh
# M1-02 acceptance: the spec is valid OpenAPI 3.1, generated Go and TypeScript
# code is current and compiles, spec rules and the contract harness pass, and
# the versioning policy is published beside the spec.
# Run from the repository root inside the dev container, after `npm ci` in app/.
# Environment: CONTRACT_BASE_URL (optional) points the live contract test at a
#              running read-api; unset skips it (read-api routes arrive in M1-10).
set -eu

# The compose acceptance service mounts only ./test; without the spec and app there is nothing to check.
if [ ! -f api/openapi.yaml ] || [ ! -f app/package.json ]; then
	echo "skip: M1-02 needs api/openapi.yaml and app/package.json (run it inside the dev container with the repository mounted)"
	exit 0
fi

step() { printf 'ok: %s\n' "$1"; }

sh api/check.sh
step "spec lints as OpenAPI 3.1 and generated code is current"

go build ./api/... ./internal/contract/...
step "generated Go compiles"

(cd app && npx --no-install tsc --noEmit --ignoreConfig --skipLibCheck --strict src/lib/api/schema.d.ts)
step "generated TypeScript compiles"

go test -tags memory ./internal/contract/...
step "spec rules (content class on every text schema, no author routes, ETag, problem types) and harness"

grep -q 'Versioning and deprecation policy' api/MACHINE-API.md
step "versioning and deprecation policy published beside the spec"
