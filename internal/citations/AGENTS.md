# AGENTS.md — internal/citations/

Purpose: extract references to existing law from a stored bill version and resolve them to US Code provisions with a confidence state.
Skills: `tool-go`, `tool-golangci-lint`, `tool-bill-xml`, `tool-docker`.
Owns: extraction, resolution, excerpts, freshness flags, `Step`. Does not own: the stored types and ports (`internal/data/citations.go`), the US Code parser (`internal/uslm`), the release ingest (`internal/lawingest`), the routes (`internal/readapi/citations.go`).

## Rules
- Unknown is better than wrong: a missing subdivision, an "et seq." cite, or a section number shared by two sections is `ambiguous`; nothing found is `unresolved` with a reason. Nothing is dropped except an Act-section phrase directly followed by its own markup citation.
- Markup (`external-xref` `legal-doc` and `parsable-cite`) comes before text patterns; text never repeats a markup cite.
- Any change to extraction or resolution that changes output for the same input needs a new `RulesVersion`; sets are write-once per `(version, method)`, and the method includes the release point.
- The act table learns only from the same version's own paired citations; no unofficial mapping goes in.
- Output depends only on the version, its source, the release, and `later`: no clock, no map-order effects.

## Commands
- `docker exec pc-<id>-dev go test -tags memory ./internal/citations/...`
- `docker exec pc-<id>-dev go run -tags memory ./scripts/measure-citations -law .local-data/uscode/119-142 -labels test/acceptance/M1-07/labels.json`

## Gotchas
- US Code ids use en dashes (`1395cc–7`); stored ids and bill `parsable-cite` use hyphens. `NormalizeSection` converts.
- Two sections can share a number (10 U.S.C. 130g); `uslm` stores the second as `130g-dup2`.
- A repealed section may carry no `status` attribute; its heading says "Repealed".
