# M1-04 — Bill XML Parser and Anchors

## Goal

Turn an official Congress bill XML document into the parsed version every other feature reads: sections, lines with depth, blocks, and anchors that are stable for the life of the version. Answer the M1 open questions on the smallest line unit, non-prose content, and wrapped continuation lines, with tests on real bills.

## Read first

- [01 § Domain concepts](../../01-bill-ingestion-and-display.md#domain-concepts) (section, line, block, anchor) and [§ Open questions](../../01-bill-ingestion-and-display.md#open-questions)
- [01 § Kinds of bill change](../../01-bill-ingestion-and-display.md#kinds-of-bill-change)
- [architecture § Bill source § Recommendation](../../../architecture/README.md#recommendation) (parsing at ingest, element rules, parser version)
- [MILESTONES § Prior work, M1](../../MILESTONES.md#m1--real-bill-text-read-only-1) (`processBillLayer`, `BillLayer` hash, `nodesToStore`)
- `usgpo/bill-dtd` and the GPO bill XML user guide
- `app/src/lib/anchors.ts` (current fragment format `#s4-l12-13`)

## Scope

**In**
- Go package that takes original XML bytes and returns the parsed version: ordered sections with heading and citation label (`Sec. 4(b)`), lines with anchor, depth, text, and source XML id, and block boundaries.
- Element handling for `legis-body`, `resolution-body`, `preamble`, `quoted-block`, and the structural levels (title, subtitle, chapter, section, subsection, paragraph, subparagraph, clause, subclause, item, list items).
- Non-prose units: tables, formulas, graphics, tables of contents stored as single addressable units with their raw markup.
- Quoted blocks (text being inserted into existing law) kept distinct from the bill's own structure, so citations and explanations can tell them apart.
- Anchor scheme: deterministic, derived from structure and the XML `id` where present; never random. Record the scheme.
- Parser version stamped on every output; re-parse produces a new parser version beside the old one.
- Golden-file tests on a corpus of real bills covering amendatory text, insertions, repeals, freestanding text, tables, and long bills; validation against the DTD.
- **Example data and exit bills.** Choose the corpus, fetch it once from govinfo with a small fetch script (the `govinfo` adapter itself is M1-05), and commit it as the repository's example data under `fixtures/` ([README § Example data and local triggers](../README.md#example-data-and-local-triggers)). Content does not matter; coverage does. Name three of the bills as the M1 exit bills: at least one with several official versions, and together covering amendatory text, insertions, repeals, freestanding text, a table, and a formula.
- **Prefer short bills.** Every live generation run costs money in proportion to bill length, so exit bills stay short: each official version at most about 30 KB of XML, several versions where possible. Larger bills may join the parser corpus (long-bill tests), never the exit set.
- **Exit bills (decided)** (119th Congress, found from the govinfo `BILLS` listing on 2026-11-22). Use these three; if one cannot be fetched or parsed, report it as blocking rather than substituting:

  | Bill | Versions | Size per version | Covers |
  | --- | --- | --- | --- |
  | `us-119-hr-4313` Hospital Inpatient Services Modernization Act | `ih`, `rh`, `eh`, `rfs` | 8–11 KB | Amendatory text: quoted blocks, striking and inserting; US Code references; text changed between `ih` and `rh`; a healthcare bill (pilot topic) |
  | `us-119-hr-3062` Promoting Cross-border Energy Infrastructure Act | `ih`, `rh`, `eh`, `rfs` | 18–20 KB | A repeal, striking and inserting, many US Code references |
  | `us-119-hr-3427` Water Resources Technical Assistance Review Act | `ih`, `rh`, `eh`, `rfs` | 8–10 KB | Freestanding text (no quoted blocks or amendments) |

- **Tables and formulas** are rare in short bills; none appeared among about 135 short bills checked. Find the shortest bill with a `<table>` and one with a formula by downloading the session's bulk archive once (`https://www.govinfo.gov/bulkdata/BILLS/119/1/hr/BILLS-119-1-hr.zip`, one request) into `.local-data/` and scanning it offline. Add those bills to the parser corpus; they need not be exit bills.

**Out**
- The `govinfo` adapter and pulls (M1-05), citation extraction (M1-07), version diff (M1-08).
- Amendment XML (`amend.dtd`): see open questions.

## Deliverables

- Parser package with golden tests and a fuzz test for malformed input
- `text.json` schema matching the M1-02 line-range resource
- Test corpus under a fixtures directory (public-domain official XML), also usable by `adapters/fixturesrc`
- Written answers to the line-unit, non-prose, and continuation-line questions

## Requirements

- **Anchors never change for a given version**; parsing the same bytes with the same parser version yields byte-identical output.
- **Depth comes from the official XML structure**, so blocks are the legislature's structure ([01 § Rules](../../01-bill-ingestion-and-display.md#rules-and-invariants)).
- A selection never splits a sentence: wrapped continuation text at the same depth belongs to one line.
- Anchors are compatible with the UI's existing fragment format, or a mapping is defined.
- Parser never runs on a reader's request; it runs in ingest only.
- Robust to malformed or unexpected input: unknown elements are kept as text with a logged warning, never dropped silently.

## Suggested breakdown

1. Survey: collect 20–30 real bill XML files across bill types and versions; list elements actually used.
2. Decide and document the line unit (source element per line vs. sentence vs. clause), with examples from the corpus.
3. Structural walk producing sections and lines with depth; heading and citation labels.
4. Non-prose and quoted-block handling.
5. Anchor scheme and stability tests (same input, same output; ids preserved).
6. Block computation and tests (findings section example in 01).
7. Golden files, DTD validation, fuzzing.
8. `text.json` writer matching the contract.

## Acceptance criteria

- `fixtures/bills/` holds the corpus with `fixtures/README.md` listing each bill's id, versions, source URL, retrieval date, SHA-256 per file, and what it covers; the three exit bills are marked; the size caps hold.

- Corpus parses with no dropped text: concatenated line text equals the document's body text after whitespace normalisation.
- Golden tests pass; output is deterministic across runs and platforms.
- Every line has an anchor, a depth, and (where the XML has one) a source id.
- At least one bill with a table and one with a formula parse into single addressable units.
- Written answers recorded for the three open questions.

## Open questions

- What is the smallest line unit: the source's own line numbering, sentence, or clause ([01 § Open questions](../../01-bill-ingestion-and-display.md#open-questions))? Answer from the corpus, move it to decided behavior in 01, and summarise it in [architecture § Bill source](../../../architecture/README.md#bill-source).
- How are tables, formulas, and other non-prose content made addressable? Prior work suggests one unit stored raw; confirm on the corpus.
- How are wrapped continuation lines told apart from separate items?
- What anchor scheme is used, and how does it map to the UI fragment form `#s4-l12-13`? Record it in [06](../../06-sharing.md) and [10 § Canonical addresses](../../10-machine-access.md#canonical-addresses).
- Which package holds the parser? The architecture names domain packages (`ingest` among them) but no parser package.
- Does any chosen exit bill need amendment XML (`amend.dtd`)? Prefer exit bills that do not.
- DTD validation method: **Decided:** this task chooses Go tests or `xmllint` in a CI container, and proposes it for [architecture § Tooling](../../../architecture/README.md#tooling).
