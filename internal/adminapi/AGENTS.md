# AGENTS.md — internal/adminapi/

Purpose: the handler of admin-api, the operator-only service `pubctl` calls.
Skills: `tool-go`, `tool-golangci-lint`, `tool-docker`.
Owns: routes, authorisation by role, reasons, public log writes, the roster log route. Does not own: wiring (`cmd/admin-api`), the CLI (`cmd/pubctl`), policy rules (`internal/selection`), the pull (`internal/ingest`).

## Rules
- Every route goes through `guard`: only a principal on the operator list reaches a route, the deploy principal reaches the roster route and nothing else, and operators do not reach the roster route ([architecture § Administration](../../architecture/README.md#administration)).
- Every mutating call needs a non-empty reason. One-document changes use `PlatformDocs.ApplyLogged`; changes that span documents log first (`Moderation.Record`) and run only if that succeeded.
- The public log names the operator id and public name (`data.LogOperator`), never a principal. Principals are never logged, echoed, or put in an error.
- Never import `internal/readapi`, and never add a port that reads individual signals or request reasons (`test/ci/deps.sh` and depguard check both).
- Corrections cover ingestion errors only: `CatalogPatch` has no field for tracked state, reasons, sponsor party, or text.
- No CORS headers, no route in `api/openapi.yaml`.

## Commands
- `docker exec pc-<id>-dev go test -tags memory ./internal/adminapi/...`
- `docker exec pc-<id>-dev go tool golangci-lint run ./internal/adminapi/...`

## Gotchas
- A bill's stored reasons live on the catalog entry and the bill document; `syncBillDoc` keeps the second in line, because the pull reads the bill document first.
- Roster log ids carry a zero-padded sequence (`roster-00000003-...`); the rebuild sorts on it, since log days are coarse.
