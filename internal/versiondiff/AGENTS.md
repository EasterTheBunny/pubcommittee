# AGENTS.md — internal/versiondiff/

Purpose: compare two official versions of a bill and classify each old line unchanged, moved, changed, or removed.
Skills: `tool-go`, `tool-golangci-lint`, `tool-bill-xml`, `tool-docker`.
Owns: the comparison library and the step that stores comparisons. Does not own: the stored types and ports (`internal/data/comparisons.go`), the parser (`internal/billxml`), the route (`internal/readapi/compare.go`), the plain-language summary (M1-09).

## Rules
- The rules are listed in `doc.go`. Any change to a rule that changes output for the same two texts needs a new `AlgorithmVersion`; stored comparisons are write-once per `(bill, from, to, method)` where method is the parser version plus `AlgorithmVersion`.
- Output depends only on the two texts: no clock, no randomness, no map-order effects. `Meta.AsOf` is the publication time of the newer version.
- White space never counts as a change; punctuation, case, and wording do.
- `ChangedThreshold` (word Dice 0.5) separates changed from removed.
- `Step` writes consecutive pairs and every older version against the latest. Nothing runs it automatically in the local phase; a command calls it over stored data.

## Commands
- `docker exec pc-<id>-dev go test ./internal/versiondiff/...`
- `docker exec pc-<id>-dev go tool golangci-lint run ./internal/versiondiff/...`

## Gotchas
- Line numbers `n` are version-wide, so one inserted line shifts every later anchor; classification never compares anchors, only text and section pairing.
- A line that differs only by its leading enumerator (`(b)` to `(c)`, `SEC. 3.` to `SEC. 4.`) is moved, not changed.
