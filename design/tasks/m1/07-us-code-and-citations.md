# M1-07 — US Code and Citation Resolution

## Goal

Load the US Code from official release points, extract citations to existing law from tracked bill versions, and resolve each to provisions with a confidence state. Unresolved and ambiguous citations are listed publicly, freshness gaps are flagged, and the resolution rate is measured.

## Read first

- [01 § Domain concepts](../../01-bill-ingestion-and-display.md#domain-concepts) (provision, citation link and its states)
- [01 § Existing law freshness](../../01-bill-ingestion-and-display.md#existing-law-freshness)
- [architecture § Bill source](../../../architecture/README.md#bill-source) (Office of the Law Revision Counsel release points, existing-law run)
- [architecture § Scheduled jobs](../../../architecture/README.md#scheduled-jobs) (weekly existing-law ingest)
- USLM schema and the Law Revision Counsel's release point documentation

## Scope

**In**
- `adapters/uscode` implementing `LawSource`: list release points, fetch a title's XML.
- Existing-law ingest (a mode of `job-ingest` or its own job; decide): store each release point's provisions with the release date; never overwrite a stored release point.
- Provision addressing: title, section, subsection path, with stable ids and the current text.
- Citation extraction from parsed bill text: US Code references (`42 U.S.C. 1395w-4(b)`), Act-and-section references (`section 1848 of the Social Security Act`), and public law references. Use the bill XML's own markup (`external-xref`, `legal-doc` attributes) where present before falling back to text patterns.
- Resolution: popular-name and Act-section to US Code mapping (source of the mapping is an open question), resolution states `resolved`, `ambiguous`, `unresolved`; `steward-confirmed` reserved for later.
- Freshness: flag provisions that public laws enacted after the release point may have changed.
- Per-version outputs: citation links per section, list of unresolved and ambiguous citations, cited provision excerpts with release date, all stored write-once per version and parser version.
- Measurement: resolution rate on a hand-labelled sample.

**Out**
- Explanations of what a bill does to a provision (M1-09).
- As-amended previews: see open questions.
- Steward confirmation workflow (M3).

## Deliverables

- `internal/adapters/uscode`, citation extraction and resolution package, existing-law ingest run
- Stored provisions and per-version citation outputs matching the M1-02 citations resource
- Hand-labelled sample and a resolution-rate report

## Requirements

- Every provision shown carries its release-point date ([01 § Existing law freshness](../../01-bill-ingestion-and-display.md#existing-law-freshness)).
- Citations that cannot be resolved are listed, never hidden.
- Unknown is better than wrong: low-confidence matches are `ambiguous`, not `resolved`.
- Inputs are official sources only.
- Same politeness rules as M1-05.

## Suggested breakdown

1. Release point source confirmation and `LawSource` adapter.
2. USLM parsing into provisions; storage layout; release-date tracking.
3. Citation extraction from bill XML markup; text-pattern fallback.
4. Act-section to US Code mapping tables and resolver with confidence.
5. Freshness flags from later public laws.
6. Per-version citation output and unresolved list.
7. Labelled sample (at least the three exit bills plus a spread of others) and a measurement script.

## Acceptance criteria

- Current release point stored; a weekly run with no new release point changes nothing.
- Every citation in the three M1 exit bills has a state; unresolved ones appear in the public list.
- Resolution rate measured on the labelled sample and recorded, with precision of `resolved` reported separately from coverage.
- Freshness flags appear for provisions amended after the release point in the sample.

## Open questions

- What resolution rate and precision does the sample show, and what target is set for M3? Recorded as exit evidence through [M1-16](16-acceptance.md).
- Is existing-law ingest a `job-ingest` mode or a separate image ([architecture § Scheduled jobs](../../../architecture/README.md#scheduled-jobs))?
- Is the existing-law check scheduled weekly in a deployed environment (cloud phase)? **Decided for the local phase:** each step runs as its own command over stored data; nothing chains steps automatically ([README § Example data and local triggers](../README.md#example-data-and-local-triggers)). Automatic chaining is a cloud-phase question.
- **Example data:** only the cited provisions of the exit bills go in `fixtures/uscode/`, with release point, source URL, and SHA-256 of the title file they came from. Whole release points are downloaded into `.local-data/` and never committed.
- Which source maps Act-and-section references to US Code sections?
- As-amended previews: **Decided:** not in M1.
