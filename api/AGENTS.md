# AGENTS.md — api/

Purpose: the public read API contract and the code generated from it.
Skills: `tool-openapi`, `tool-go`, `tool-docker`.
Owns: `openapi.yaml`, `MACHINE-API.md`, `oapi-codegen.yaml`, `redocly.yaml`, `generate.sh`, `check.sh`, `v1/` (generated). Does not own: handlers (`cmd/read-api`, M1-10) or the harness (`internal/contract`).

## Rules
- The spec is written first; never edit `v1/api.gen.go` or `app/src/lib/api/schema.d.ts` by hand.
- Every schema that holds text requires `content_class` (marked `x-carries-text`); `internal/contract` tests enforce it.
- Only `read-api` GET routes are in the spec; `write-api`, `/auth`, `/v1/me*`, and `admin-api` stay out.
- No route or parameter lists or filters by author.
- Additive changes keep `/v1`; breaking changes need `/v2` ([MACHINE-API.md](MACHINE-API.md)).

## Commands
- `docker exec <c> sh api/generate.sh` regenerates Go and TypeScript.
- `docker exec <c> sh api/check.sh` lints the spec and fails when generated code is stale (needs `npm ci` in `app/`).
- `docker exec <c> go test ./internal/contract/...`

## Gotchas
- oapi-codegen needs unique type names: parameter components end in `Path` or `Query`.
- Avoid OpenAPI 3.0 `nullable`; omit optional fields instead.
- `app/package.json` overrides `openapi-typescript`'s TypeScript peer to the app's version.
