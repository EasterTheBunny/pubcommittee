#!/bin/sh
# Contract checks: the spec lints as OpenAPI 3.1, and the committed generated
# code equals a fresh generation. Run from the repository root in the dev
# container, after `npm ci` in app/.
set -eu
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
if ! (cd api && ../app/node_modules/.bin/redocly lint openapi.yaml --config redocly.yaml); then
	echo "api: spec lint failed" >&2
	exit 1
fi
sh api/generate.sh "$tmp"
cmp "$tmp/api.gen.go" api/v1/api.gen.go || {
	echo "api: api/v1/api.gen.go is out of date; run sh api/generate.sh" >&2
	exit 1
}
cmp "$tmp/schema.d.ts" app/src/lib/api/schema.d.ts || {
	echo "api: app/src/lib/api/schema.d.ts is out of date; run sh api/generate.sh" >&2
	exit 1
}
echo "api: spec valid, generated code current"
