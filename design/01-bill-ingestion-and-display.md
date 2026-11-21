# 01 — Bill Ingestion and Display

## Purpose

Turn each official bill version into reviewable, addressable text, and show it in two layers: what the bill would change in current law, and the bill's own text where people comment and suggest edits.

## Scope

**In scope**
- Detecting and pulling official bill versions from the legislature's publication feed
- Parsing text into addressable sections and lines
- Loading and keeping current a consolidated copy of existing law
- Resolving bill citations to provisions of existing law
- Generating change explanations, and unofficial "as amended" previews where mechanical
- The two-layer bill view

**Out of scope**
- Comments, suggestions, and signals (see [03](03-review-features-and-support.md))
- Handling of version-to-version changes for review items (see [02](02-versions-and-review-lifecycle.md))
- Linking bills to each other (see [04](04-bill-lineage-and-competing-bills.md))

## Inputs

| Input | Launch source | Later |
| --- | --- | --- |
| Bill text | Official XML published by Congress for each bill version, collected automatically | Other legislatures' feeds; steward manual entry where no feed exists |
| Bill metadata | Same feed: number, title, chamber, sponsors, co-sponsors, status, actions | Same per jurisdiction |
| Existing law | US Code, published as XML | Each jurisdiction's consolidated code |

## Domain concepts

- **Bill** — identity within a jurisdiction and session; has metadata and an ordered list of official versions.
- **Official version** — immutable snapshot of text with a version type (introduced, reported, engrossed, etc.) and publication date. Never edited after load.
- **Section** — a structural unit of a version (title, section, subsection, paragraph...), with heading and citation label (e.g. "Sec. 4(b)").
- **Line** — the smallest addressable text unit. Every line has a stable anchor within its version.
- **Anchor** — `(bill, version, section path, line or line range)`. Must be stable for the lifetime of a version so links, comments, and suggestions never drift.
- **Provision** — an addressable unit of existing law, with its current text and the date the consolidated source was last current.
- **Citation link** — a resolved reference from a bill section to one or more provisions, with a confidence state: `resolved`, `ambiguous`, `unresolved`, `steward-confirmed`.
- **Change explanation** — attached to a (bill section, provision) pair. Contains: provision as it currently reads; what the bill does to it; why it matters; source flags (auto-generated, steward-checked, written from bill alone).
- **As-amended preview** — optional rendering of a provision with a mechanical instruction applied. Always labelled *unofficial*.

## Behavior

### Ingestion
1. Detect new bills and new versions of known bills from the feed.
2. Store the version as an immutable snapshot, including the original source document.
3. Parse into sections and lines; assign anchors.
4. Extract citations to existing law and attempt resolution against the current consolidated code.
5. Generate change explanations for each resolved citation.
6. Queue ambiguous or unresolved citations, and any provision whose consolidated source may lag recent enactments, for steward review.
7. Publish the version for review (hand-off to [02](02-versions-and-review-lifecycle.md)).

Where no machine-readable feed exists (later jurisdictions), a steward enters the text and the version is labelled *steward-entered*.

### Kinds of bill change
The parser and explainer must handle at least:
- **Amendatory instructions** — "in section 12, strike 'thirty' and insert 'sixty'". Mechanical; eligible for an as-amended preview.
- **Insertions of new provisions** — new sections or chapters added to existing law.
- **Repeals** — provisions struck entirely.
- **Freestanding text** — provisions that do not amend existing law (appropriations, findings, sense-of-Congress). Explained from the bill alone.

### Two-layer view
The two comparisons must be **visually distinct** so readers never confuse an explanation with a literal diff.

| Layer | Compares | Presentation |
| --- | --- | --- |
| Bill against law | Bill section vs. the provisions it touches | Explanation: current provision text, what changes, why it matters; optional unofficial preview |
| Citizen edit against bill | Suggested edit vs. bill lines | Literal side-by-side: deletions struck, insertions marked (rendered by [03](03-review-features-and-support.md)) |

### Existing law freshness
- Each provision records the effective date of the consolidated source.
- If the bill cites a provision changed by a recent enactment not yet in the consolidated source, the explanation is flagged and routed to a steward.
- Where no consolidated code exists, the explanation is written from the bill alone and marked as such.

## Rules and invariants

- Official versions are immutable once loaded.
- Anchors never change for a given version.
- Every change explanation shows its provenance (automatic, steward-checked, bill-only).
- An as-amended preview is never presented as official law.
- Steward corrections to explanations are attributed and logged (see [05](05-roles-and-stewardship.md)).

## Dependencies

- [02](02-versions-and-review-lifecycle.md) consumes new versions.
- [04](04-bill-lineage-and-competing-bills.md) uses text and cited provisions for candidate detection.
- [05](05-roles-and-stewardship.md) provides the steward queue for citation and explanation checks.
- [06](06-sharing.md) depends on stable anchors.

## Open questions

- What is the smallest line unit: the source document's own line numbering, sentence, or clause? Must be stable and meaningful for suggested edits.
- How are explanations generated at scale (rules, templates, assisted drafting), and what accuracy bar must they meet before display without steward review?
- How quickly must a new version appear after publication? The product doc marks speed as low priority at launch.
- How are tables, formulas, and other non-prose bill content made addressable?
