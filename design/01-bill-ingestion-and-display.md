# 01 — Bill Ingestion and Display

## Purpose

Turn each official bill version into reviewable, addressable text, and show it in two layers: what the bill would change in current law, and the bill's own text where people comment and suggest edits.

## Scope

**In scope**
- Selecting which bills enter review (the tracked set)
- Publishing the bill catalog, linked to the legislature's own pages
- Markup requests: people asking for a catalog bill to be brought into review
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

## Bill selection

Most bills never move past referral to a committee, and many measures are poor candidates for line-by-line public markup. The system therefore separates a **catalog** from a **tracked set**:

- **Catalog:** lightweight status metadata for every bill in the jurisdiction (identity, titles, sponsors, subject indexing, committee activity, related bills). Cheap to keep; nothing is reviewed.
- **Tracked set:** the bills that go through full ingestion, explanations, and review. Only tracked bills have review pages, comments, suggestions, signals, and briefs.

### What makes a good candidate
Public markup works on bills whose exact wording matters and can be improved line by line: bills and joint resolutions that create or amend law. It does not suit:
- simple and concurrent resolutions that are commemorative or procedural;
- war powers and similar privileged resolutions, which are essentially a yes-or-no question rather than drafting;
- other measures whose text is too short or fixed for line edits to mean anything.

These kinds are excluded from tracking by default. A bill-level signal could apply to them, but the product is not built for them (see open questions).

### Selection rules
1. **Committee action, not referral.** Nearly every bill is referred to a committee on introduction, so referral alone does not count. A bill qualifies once the legislature records committee activity beyond referral (a hearing, markup, a report, or discharge), a floor action, or placement on a calendar.
2. **Companions follow.** A bill identical to, or the other chamber's companion of, a tracked bill is tracked too ([04](04-bill-lineage-and-competing-bills.md)).
3. **Topic focus (pilot).** During the pilot, a qualifying bill must also match a focus topic, judged from the legislature's own subject indexing and the bill's title, never from our reading of its politics. Pilot topics: **healthcare, deficit reduction, and affordability**.
4. **Nominations.** Pilot offices and stewards may nominate a bill that the rules miss. Adding it needs sign-off by stewards from two parties ([05](05-roles-and-stewardship.md)) and is logged publicly.
5. **Administrator selection.** An administrator may add an allowed bill directly, through privileged system access rather than the product. Before stewards from two parties are active, this needs no other sign-off; afterwards it needs two-party steward sign-off like a nomination ([05](05-roles-and-stewardship.md#platform-administration)). Each selection is logged publicly with the administrator's name and a reason, and the bill shows "selected by an administrator" among its reasons.
6. **Once tracked, stays tracked** for the rest of its session. Changing the rules never removes a bill people are already reviewing.

### Modes
| Mode | Tracked when | Use |
| --- | --- | --- |
| Pilot | Committee action **and** a focus topic, plus companions and nominations | Pilot: small, focused data set |
| General | Committee action, plus companions and nominations | Public launch and after |

### Transparency
- The selection rules are a single, versioned, published policy. Every change is logged publicly with its reason ([09](09-integrity-moderation-and-privacy.md)).
- Every tracked bill records which policy version and which rules selected it.
- The site states the current scope plainly (for example, "bills with committee action on healthcare, deficit reduction, and affordability").
- Rules use only process signals and the legislature's own subject indexing. Sponsor party, position, and predicted outcome are never inputs.

## Public catalog

The catalog is published, so anyone can find any bill, see whether it is open for markup, and read its official text where the legislature publishes it.

**Each catalog entry shows**
- Number, title, chamber, session, sponsor, status, latest action, and the legislature's subject indexing.
- **Links to the legislature's own pages:** the bill's page and each published text version. The catalog never hosts or re-renders the text of an untracked bill; the official site is the source.
- **Markup status:**
  - *In review*: tracked; links to the bill's review page here.
  - *Eligible*: meets the measure-kind rule but not yet the rest of the policy (for example, no committee action yet, or outside the pilot topics). Can be requested.
  - *Not a markup candidate*: an excluded measure kind, with the reason (for example, "privileged resolution: a yes-or-no question rather than drafting").
- Why a tracked bill was selected (policy version and rules), and the public outcome of any markup request.

**Browsing.** Filter by status, chamber, committee activity, and subject; search by number or title. Catalog entries have stable addresses and are available to machines like every public page ([10](10-machine-access.md)).

## Markup requests

People can ask for an eligible catalog bill to be brought into review. Requests feed the steward nomination path in rule 4 of [Selection rules](#selection-rules); they never track a bill automatically.

**Making a request**
- Any eligible reviewer ([08](08-onboarding-and-verification.md)) can request markup of an *eligible* bill, once per bill, with an optional short reason ("why this bill would benefit from line-by-line review").
- Requests are withdrawable until a decision is made.
- *Not a markup candidate* bills cannot be requested; *in review* bills need no request.

**Privacy and counts**
- Who requested a bill is private, like a signal ([09](09-integrity-moderation-and-privacy.md)). Only the total is public.
- Request totals follow the same publication rules as support counts: published in batches, small totals shown as "fewer than N" ([11](11-ai-security.md)).
- Request reasons are shown to stewards without names. They are not published.

**Review**
- A bill enters the steward review queue when its requests reach a threshold, or when a pilot office or steward nominates it directly.
- Stewards decide against the published policy only: is the text suited to line-by-line markup, does it fall within the current scope, is there capacity. Acceptance needs sign-off by stewards from two parties ([05](05-roles-and-stewardship.md)).
- Every decision is public with its reason: *accepted* (the bill becomes tracked, with "requested" among its selection reasons) or *declined* (with the rule that applied). A declined bill can be requested again if its status changes, for example after committee action.
- Requesters are told the outcome if they asked to be notified.

**Integrity**
- Request surges are campaigns like any other: allowed, and disclosed with the decision ([09](09-integrity-moderation-and-privacy.md)). A surge alone never forces acceptance, and a decision never weighs who is likely to be requesting.
- Requests are writes by people; automated requests are handled as in [10](10-machine-access.md).
- Request totals are not a signal about the bill's merits and are not sent to legislators.

## Domain concepts

- **Bill** — identity within a jurisdiction and session; has metadata and an ordered list of official versions.
- **Official version** — immutable snapshot of text with a version type (introduced, reported, engrossed, etc.) and publication date. Never edited after load.
- **Section** — a structural unit of a version (title, section, subsection, paragraph...), with heading and citation label (e.g. "Sec. 4(b)").
- **Line** — the smallest addressable text unit. Every line has a stable anchor within its version, and a depth: lines nested under another line (subsections, paragraphs, list items) are deeper than it.
- **Block** — a line together with every following line nested under it. A block is always contiguous, so it is addressed like any line range. A line with nothing nested under it is a block of one line. Example: a findings section whose line 5 ("Congress finds the following:") has items on lines 6–8 forms the block 5–8; line 7 is a block on its own.
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
| Bill against law | Bill section vs. the provisions it touches | Explanation: current provision text, what changes, why it matters; optional unofficial preview. Shown directly above the lines it explains, naming the provision it changes, as one control per part that has content; one part is open at a time and pushes the text down. Readable without scripts (M1 operator feedback) |
| Citizen edit against bill | Suggested edit vs. bill lines | Literal side-by-side: deletions struck, insertions marked (rendered by [03](03-review-features-and-support.md)) |

### Existing law freshness
- Each provision records the effective date of the consolidated source.
- If the bill cites a provision changed by a recent enactment not yet in the consolidated source, the explanation is flagged and routed to a steward.
- Where no consolidated code exists, the explanation is written from the bill alone and marked as such.
- A section that only gives the Act its short title gets no explanation: its text says all there is to say.

## Rules and invariants

- Official versions are immutable once loaded.
- Anchors never change for a given version.
- Line depth comes from the official text's own structure, so blocks are the legislature's structure, not ours.
- Every change explanation shows its provenance (automatic, steward-checked, bill-only).
- Every auto-generated summary and change explanation carries a one-line note in small text: "Auto-generated; tested accuracy 75–85%. Verify with the bill text.", where "bill text" links to the lines it describes (M1).
- An as-amended preview is never presented as official law.
- Steward corrections to explanations are attributed and logged (see [05](05-roles-and-stewardship.md)).
- Only tracked bills enter review; the catalog is never reviewed.
- The catalog links to the legislature's own text for untracked bills and never republishes it.
- A markup request never tracks a bill by itself; acceptance is a two-party steward decision against the published policy, with a public reason.
- Who requested a bill is never shown to anyone, including stewards.
- Selection never uses sponsor party, position, or predicted outcome, and a tracked bill is never untracked by a rule change.

## Dependencies

- [02](02-versions-and-review-lifecycle.md) consumes new versions.
- [04](04-bill-lineage-and-competing-bills.md) uses text and cited provisions for candidate detection.
- [05](05-roles-and-stewardship.md) provides the steward queue for citation and explanation checks.
- [06](06-sharing.md) depends on stable anchors.

## Open questions

- What is the smallest line unit: the source document's own line numbering, sentence, or clause? Must be stable and meaningful for suggested edits.
  - **Decided (M1-04):** one line per source element that carries its own text. A level element's head line joins its enum, header, and first text with single spaces; its children nest one level deeper. Neither the source's own line numbering nor sentences are used.
- How are explanations generated at scale (rules, templates, assisted drafting), and what accuracy bar must they meet before display without steward review?
  - **Measured (M1-09):** tested accuracy of auto-generated summaries and explanations is 75–85% against the whole bill text, from operator grading of a hard sample: 20 of 30 statements were supported by the lines they cite (67%), and 3 to 6 more were correct but rested on lines of the bill they did not cite (77–87%) ([accuracy report](../test/accuracy/report.md)). Auto-generated text is shown with that range and a prompt to verify it against the bill text. The bar for display without steward review is still open.
- How quickly must a new version appear after publication? The product doc marks speed as low priority at launch.
- How are tables, formulas, and other non-prose bill content made addressable?
  - **Decided (M1-04):** tables, formulas, graphics, and tables of contents are one line each, with their kind, flattened text, and raw markup. Confirmed on real bills for tables; no real bill in the 119-1 House archive has a `<formula>`, so formulas are tested on constructed XML only.
- How are wrapped continuation lines (one sentence split across lines at the same depth) told apart from separate items, so a selection never splits a sentence?
  - **Decided (M1-04):** a sentence split across list items is finished by the source's continuation text, which is its own line at the parent's depth, flagged `continuation`, so a selection can extend over it. Punctuation after a quoted block is appended to the line before it.
- Should excluded measure kinds (privileged resolutions, simple resolutions) offer a bill-level signal only, or stay out of the product entirely?
- The legislature's subject indexing often arrives days or weeks after introduction. Is title matching enough in the meantime, or do pilot bills wait for indexing?
- Which subject terms define each focus topic, and who maintains that list?
- When exactly does the administrator phase rule end: the first two-party steward pair, or a fixed pilot date?
- What request threshold puts a bill in the steward queue, and should it scale with the number of reviewers?
- How many bills can stewards and the pilot absorb, and how is a capacity decline explained so it does not read as a judgment on the bill?
- When does the pilot topic filter end: at public launch, or when volume allows?
