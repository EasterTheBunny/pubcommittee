---
name: tool-openapi
description: Editing api/openapi.yaml (OpenAPI 3.1) to PubCommittee's API conventions and regenerating Go server types (oapi-codegen) and TypeScript types (openapi-typescript) inside the task container. Load for any change to the API contract or its generated code.
---

# OpenAPI contract

`api/openapi.yaml` is the source of truth ([architecture § API contract](../../../architecture/README.md#api-contract)). Generated code is never edited by hand.

## Conventions

- `/v1` in every path; `snake_case` fields; RFC 3339 UTC timestamps.
- Collections: `{ "items": [...], "next": "<cursor>" }`; `?cursor=` only, never offsets.
- Every record carrying text has `version`, `as_of`, `provenance`, `content_class` (`official` | `platform` | `user`). Summary fields allow only `official` or `platform`.
- Errors: RFC 9457 `application/problem+json` with `type` one of `not-found`, `conflict`, `invalid`, `rate-limited`, `unavailable`; `429` has `Retry-After`.
- Every `GET` documents `ETag` and `304`; official version text documents `Cache-Control: immutable`.
- API addresses mirror page addresses (`/v1/bills/{bill}/v/{n}/s/{section}/l/{range}`).
- No route lists or filters by author. `admin-api` routes are not in this spec.
- Reuse `components/schemas`; one schema per domain concept, named in design-doc vocabulary.

## Regenerate (inside the container)

```sh
docker exec <container> go tool oapi-codegen -config api/oapi-codegen.yaml api/openapi.yaml
docker exec -w /src/app <container> npx openapi-typescript ../api/openapi.yaml -o src/lib/api/schema.d.ts
docker exec <container> go build ./...
docker exec -w /src/app <container> npm run check
```

## Checks

- Spec validates as OpenAPI 3.1 with the repository's linter (see [architecture § Tooling](../../../architecture/README.md#tooling)).
- Generated code is current: regenerate, then the task-orchestrator checks `git diff --exit-code` on the host.
- Contract tests validate live responses against the spec.
