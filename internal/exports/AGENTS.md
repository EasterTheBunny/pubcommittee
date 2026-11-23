# AGENTS.md — internal/exports/

Purpose: builds the bulk exports and the change feed from stored data (M1-11).
Skills: `tool-go`, `tool-golangci-lint`, `tool-docker`.
Owns: export file formats, export ids, feed event ids and titles. Does not own: the ports (`internal/data`), routes (`internal/readapi`), wiring (`cmd/job-exports`).

## Rules
- As-of times come from stored records, never the clock, so a rebuild is byte-identical and write-once exports stay no-ops ([design/10 § Bulk exports](../../design/10-machine-access.md#bulk-exports)).
- Every export file has the header (`export`, `kind`, `as_of`, `licence`, `licences`, `content_class`) and every record a `content_class`.
- Feed ids are stable per event; a revision gets a new id. Titles come from templates: no source, user, or administrator text.
- Export ids match `^[a-z0-9][a-z0-9._-]*$` (spec); a new per-version export needs an id function in `format.go` and a line in `cmd/job-exports/README.md`.

## Commands
- `docker exec pc-<id>-dev go test -tags memory ./internal/exports/`
- `docker exec pc-<id>-dev go tool golangci-lint run ./internal/exports/...`
