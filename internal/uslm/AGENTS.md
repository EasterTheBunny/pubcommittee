# AGENTS.md — internal/uslm/

Purpose: parse one US Code title XML file (USLM, Law Revision Counsel) into `data.LawSection` records.
Skills: `tool-go`, `tool-golangci-lint`, `tool-bill-xml`, `tool-docker`.
Owns: the streaming parser and its test data. Does not own: fetching (`internal/adapters/uscode`), storage, citations.

## Rules
- Stream the file and keep one section tree at a time; real titles are over 100 MB.
- Same bytes plus same release give identical output. Notes, source credits and tables of contents are not part of provision text.
- Ids use hyphens for every dash; a repeated section number is stored as `<id>-dup<N>`.

## Commands
- `docker exec pc-<id>-dev go test ./internal/uslm/...`
