# AGENTS.md — internal/billxml/

Purpose: parse one official bill XML document (govinfo BILLS, `bill.dtd`) into sections, lines, depth and anchors (`text.json`).
Skills: `tool-go`, `tool-golangci-lint`, `tool-bill-xml`, `tool-docker`.
Owns: the parser, its golden files (`testdata/`), the DTD copies (`testdata/dtd/`). Does not own: fetching (M1-05 `govinfo` adapter), citations (M1-07), version diff (M1-08), the example data (`fixtures/`, README there).

## Rules
- Parsing runs in ingest only, never on a request. Same bytes plus same `ParserVersion` give byte-identical output.
- Any change to what `Parse` emits for existing input needs a new `ParserVersion` and regenerated goldens (`BILLXML_UPDATE=1 go test ./internal/billxml/ -run Golden`), reviewed line by line.
- Never drop text: unknown elements are kept as text with a warning in `Document.Warnings`. The corpus test compares line text with an independent flattening of the body.
- Anchors are `s<ordinal>-l<n>`: `s0` is the preamble, sections count from 1 in document order, `n` counts lines from 1 across the whole version.
- Depth comes from XML nesting; quoted blocks (`Quoted`) nest under the line that introduces them; table, formula, graphic and toc are one line each with `Raw` markup.
- Originals are never modified; `Raw` is a byte slice of the source.

## Commands
- `docker exec pc-<id>-dev go test ./internal/billxml/...`
- `docker exec pc-<id>-dev go test ./internal/billxml/ -run xxx -fuzz FuzzParse -fuzztime 30s`
- `docker exec pc-<id>-dev go tool golangci-lint run ./internal/billxml/...`
- `BILLXML_BULK_DIR=<dir of govinfo XML> go test ./internal/billxml/ -run Bulk` checks a larger local pull (never commit it).

## Gotchas
- A parsed reported-with-amendment bill has two `legis-body` elements; both are parsed in order. `amendment-doc` content is not parsed (warning).
- `<quote>` text is wrapped in curly quotes in line text; page breaks are dropped.
- Containers run as root: `chown -R` the worktree afterwards.
