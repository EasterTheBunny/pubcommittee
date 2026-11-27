# 02 — Versions, Amendments, and Review Lifecycle

## Purpose

Define when a bill is open for review, what happens when the legislature publishes a new official version, and how review items stay attached to the right text across versions.

## Scope

**In scope**
- Review states of a bill
- Handling new official versions (amendments, substitutes, cross-chamber versions)
- Stale detection, re-anchoring, and carry-over of support

**Out of scope**
- Parsing new versions (see [01](01-bill-ingestion-and-display.md))
- Linking separate bills into one review (see [04](04-bill-lineage-and-competing-bills.md))
- Matching official amendments to citizen suggestions for notifications (see [07](07-legislator-brief-and-responses.md), "Closing the loop")

## Lifecycle

```
Introduced ──► In review ──► Closed
                 ▲   │        (final vote, enacted, or dead)
                 └───┘
        new official version
     (reopens touched lines)
```

| State | Entered when | Effect |
| --- | --- | --- |
| In review | Bill introduced and first version published | Commenting, suggesting, and signalling open |
| In review (version updated) | Legislature publishes a new official version | Review continues on the new version; items on changed lines go stale |
| Closed | Final vote taken, bill enacted, or bill dies (e.g. end of session) | Read-only; signals frozen; history preserved |

There is **no voting window**. Signals accumulate for as long as the bill is in review.

## Review cycle steps

1. **Bill opened for review** — first official version ingested and published.
2. **Citizen review** — line comments, suggested edits, bill-level signal ([03](03-review-features-and-support.md)).
3. **Signals collected** — continuously, no deadline.
4. **Delivered to legislators** — periodic and event-triggered briefs ([07](07-legislator-brief-and-responses.md)).
5. **Legislator responds** — public replies per item ([07](07-legislator-brief-and-responses.md)).

The amendment loop returns to step 2 for the lines that changed.

## New official version handling

A bill's versions are ordered by legislative stage (introduced, reported, engrossed, and so on), never by the date on them, and each is addressed by its own stage code, so an address never changes when another version arrives. A version that only records a referral to the other chamber, with the same text as the version before it, is not a version of its own: that version shows a note of the referral and its date (M1-05b).

When a new version arrives:

1. Compute a text mapping from the previous version to the new one, at the line/section level.
   - **As built (M1-08, `internal/versiondiff`, method `versiondiff/1`):** lines are classified by text and section pairing, not by anchor number. A line with the same text in a paired section with the same label is *unchanged*. A same-text line whose section label differs, or that sits out of order or in another section, is *moved*; so is a line that differs only by its leading enumerator (`(b)` to `(c)`, `SEC. 3.` to `SEC. 4.`). Otherwise, old and new lines with word-multiset Dice similarity of at least 0.5 are paired best-first, one to one, as *changed*; an old line with no such pair is *removed*. Whitespace-only differences are not changes; punctuation and wording are. Comparisons are stored for consecutive version pairs and for each older version against the latest.
2. Classify each existing anchored item (comment thread, suggested edit, rider tag):

| Classification | Meaning | Treatment |
| --- | --- | --- |
| Unchanged | Target lines identical in new version | Auto-move anchor; no notice needed |
| Moved | Same text, different position | Auto-move anchor; show "moved" note |
| Changed | Target text altered | Mark **stale** |
| Removed | Target text no longer present | Mark **stale (target removed)** |

3. For stale suggested edits:
   - The **author** re-anchors the suggestion to the new text. Stewards may assist with mechanical re-anchoring where meaning is clearly unchanged ([05](05-roles-and-stewardship.md)).
   - If meaning still holds, **support carries over** and supporters are notified so they can withdraw.
   - If meaning changed, **support resets**.
   - Every revision and re-anchor is kept in history.
4. Line comment threads on changed lines remain readable, anchored to the old version, with a pointer to the new location if one exists.
5. The bill-level signal persists across versions. Reviewers are notified of the new version and may change or withdraw their signal.

## Rules and invariants

- Every item always records the version it was authored against.
- Support never silently migrates across a change in meaning.
- Prior versions remain viewable; nothing is deleted on version change.
- Closed bills are read-only but fully browsable and shareable.

## Dependencies

- [01](01-bill-ingestion-and-display.md) emits new versions.
- [03](03-review-features-and-support.md) owns the items being re-anchored.
- [06](06-sharing.md) uses version pinning for shared links.
- [04](04-bill-lineage-and-competing-bills.md) reuses re-anchoring when linking a reintroduced copy.

## Open questions

- Who judges "meaning still holds": the author alone, or author with steward confirmation? What happens if the author is inactive?
- How long does a stale suggestion stay in the brief before re-anchoring? Should it be excluded until re-anchored?
- What exact events close a bill in each chamber (e.g. passing one chamber vs. final passage), and does review continue between chambers?
- Should the bill-level signal be prompted for reconfirmation after a substantial new version?
