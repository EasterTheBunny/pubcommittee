# 04 — Bill Lineage and Competing Bills

## Purpose

A bill's number and name are not a reliable identity. Track the same legislation across reintroductions and chambers as one continuous review, and group bills that tackle the same issue differently as competing implementations without merging them.

## Scope

**In scope**
- Lineage links: reintroduced copies and cross-chamber companions
- Issue groups of competing bills, with neutral comparisons
- Automatic candidate detection; steward confirmation

**Out of scope**
- Parsing bill text (see [01](01-bill-ingestion-and-display.md))
- Steward permissions and sign-off mechanics in general (see [05](05-roles-and-stewardship.md))

## Cases

| Case | In the legislature | System behavior |
| --- | --- | --- |
| Reintroduced copy | Filed again, often in a new session, same or near-same text, new number or name | Linked to the earlier bill. Comments and suggestions carry over, re-anchored to the new text. Supporters notified and can withdraw |
| Cross-chamber companion | Same legislation under different names or numbers in each chamber | Joined into one review page. One set of signals. Each chamber's text is shown as a version |
| Competing bills | Several bills on the same issue with different details | Grouped under a shared **issue**. Never merged or marked duplicate. Each keeps its own review and signals |

## Domain concepts

- **Review** — the user-facing unit that collects signals. Normally one bill; a lineage link makes several bills share one review.
- **Lineage link** — `(earlier bill, later bill, kind: reintroduction | companion)`, with confirming steward(s) and timestamp.
- **Issue** — a named topic grouping competing bills. Has a neutral comparison text and a list of member reviews.
- **Neutral comparison** — short steward-written text on how the bills differ. Requires two-party steward sign-off.
- **Link candidate** — a system suggestion with evidence scores and a proposed kind, awaiting steward decision.

## Candidate detection

The system compares each new bill against existing ones on two signals:

| Signal | Indicates |
| --- | --- |
| High text overlap | Reintroduced copy or companion |
| Overlap in provisions of existing law amended, but different text | Competing bill |

- The system **only suggests**. A steward confirms every link and every issue grouping.
- Rejected candidates are remembered so they are not re-suggested without new evidence.

## Behavior

### Linking a reintroduced copy
1. Steward confirms the candidate.
2. Items on the earlier bill are re-anchored to the later text using the same mechanics as a new official version ([02](02-versions-and-review-lifecycle.md)): unchanged lines carry over; changed lines go stale.
3. Supporters of carried-over items are notified and may withdraw.
4. The earlier bill's page points to the continuing review.

### Joining cross-chamber companions
1. Steward confirms.
2. Both bills present as one review page; each chamber's text appears as a version in the version history.
3. Signals on both bills are combined into one set. A reviewer who signalled on both keeps one signal per item (deduplicated).

### Issue pages
- Lists competing bills side by side.
- Shows the neutral comparison and each bill's support and opposition counts.
- Each member bill's page shows a banner naming its alternatives.
- The brief shows competing bills together ([07](07-legislator-brief-and-responses.md)).

## Rules and invariants

- Competing bills are never merged and never labelled duplicates.
- Every lineage link and issue grouping is attributed, public, and reversible by stewards, with the reversal logged.
- Issue comparisons require sign-off from stewards of two different parties.
- Unlinking restores separate reviews without losing history.

## Dependencies

- [01](01-bill-ingestion-and-display.md) supplies text and cited provisions.
- [02](02-versions-and-review-lifecycle.md) supplies re-anchoring.
- [05](05-roles-and-stewardship.md) supplies confirmation, sign-off, and logging.
- [07](07-legislator-brief-and-responses.md) presents competing bills together.

## Open questions

- How are signals reconciled when two already-active bills are joined as companions and a reviewer signalled differently on each?
- Can a bill belong to more than one issue?
- What happens to lineage when a reintroduced copy diverges substantially in a later version?
- Who names and describes an issue, and does the name itself need two-party sign-off?
