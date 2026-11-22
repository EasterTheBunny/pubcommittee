#!/bin/sh
# Regenerate the Go and TypeScript code from api/openapi.yaml.
# Run from the repository root inside the dev container:
#   sh api/generate.sh            write api/v1/api.gen.go and app/src/lib/api/schema.d.ts
#   sh api/generate.sh <dir>      write both files under <dir> instead (used by check.sh)
set -eu
go_out=api/v1/api.gen.go
ts_out=$PWD/app/src/lib/api/schema.d.ts
if [ $# -ge 1 ]; then
	mkdir -p "$1"
	go_out=$1/api.gen.go
	ts_out=$1/schema.d.ts
fi
go tool oapi-codegen -config api/oapi-codegen.yaml -o "$go_out" api/openapi.yaml
(cd app && npx --no-install openapi-typescript ../api/openapi.yaml -o "$ts_out" >/dev/null)
