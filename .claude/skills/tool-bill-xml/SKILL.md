---
name: tool-bill-xml
description: Working with official Congress bill XML (govinfo BILLS and BILLSTATUS, usgpo/bill-dtd) and US Code XML in Go: element rules, anchors, depth, quoted blocks, non-prose units, golden-file tests, and source politeness. Load for parser, ingest, citation, or version-comparison work (M1-04, M1-05, M1-07, M1-08).
---

# Bill and law XML

Read first: [01 § Rules and invariants](../../../design/01-bill-ingestion-and-display.md#rules-and-invariants), [architecture § Bill source](../../../architecture/README.md#bill-source), and the task file.

## Invariants

- **Originals are bytes.** Store source XML byte for byte with source URL, retrieved-at, and SHA-256. Never re-encode, pretty-print, or normalise what is stored.
- **Parse at ingest, never on a request.** Output carries a parser version; same bytes and parser version give byte-identical output.
- **Anchors** are deterministic: derived from structure and the XML `id` where present, never random. They never change for a version.
- **Depth** comes from the XML nesting (section, subsection, paragraph, subparagraph, clause, subclause, item).
- **No dropped text.** Unknown elements are kept as text with a logged warning.

## Elements

- Bodies: `legis-body`, `resolution-body`, `preamble`.
- `quoted-block` is text inserted into existing law: keep it distinct from the bill's own structure.
- Non-prose (`table`, `formula`, `graphic`, `toc`): one addressable unit each, raw markup kept.
- Cross-references: prefer `external-xref` and `legal-doc` attributes over text patterns for citations.

## Go

- Stream with `encoding/xml.Decoder` (`Token`, `RawToken` where raw markup must be kept); do not load whole documents into DOM-like trees when a stream works.
- Keep character data exactly; normalise whitespace only in derived text, never in stored originals.
- Golden tests: `testdata/<bill>/source.xml` in, `testdata/<bill>/text.golden.json` out; `go test ./... -update` regenerates.
- Check "no dropped text": concatenated line text equals body text after whitespace normalisation.

## Fetching public sources

- Only the sources the task names: govinfo bulk data, the Office of the Law Revision Counsel, `usgpo/bill-dtd`.
- One request at a time per host, backoff on errors, conditional requests where supported, a descriptive user agent naming the project with no personal contact details.
- Fetch a corpus once into `testdata/` (or the local store) and test offline afterwards.
