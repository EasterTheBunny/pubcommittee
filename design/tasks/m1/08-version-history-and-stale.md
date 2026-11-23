# M1-08 — Version History and Stale Classification

## Goal

Compute, for every pair of consecutive official versions of a tracked bill, a mapping from old lines to new lines and a classification of each old anchor as unchanged, moved, changed, or removed. M1 has no review items yet, so the output serves "changed since" notices on old links and is ready for M2 re-anchoring of comments and suggestions.

## Read first

- [02 § New official version handling](../../02-versions-and-review-lifecycle.md#new-official-version-handling)
- [06 § Link behavior](../../06-sharing.md#link-behavior) (pinned, changed since)
- `app/src/lib/anchors.ts` (`isStale`, which this replaces with computed data)
- [MILESTONES § Prior work, M1](../../MILESTONES.md#m1--real-bill-text-read-only-1) (swap points)

## Scope

**In**
- Version mapping package: input two parsed versions (M1-04), output a line and section mapping with a classification per old anchor and per old line range.
- Range classification: a range is unchanged only if every line is; moved if the same text appears contiguously elsewhere; changed otherwise; removed if no counterpart exists. Return the best new location when one exists.
- A step that runs after a new version is stored ([M1-05](05-manual-bill-pull-and-storage.md)) and writes the comparison write-once per `(bill, from, to, parser version)`. Which unit runs it in M1 is an open question.
- Comparison resource for `read-api` (shape agreed in M1-02): per old anchor, its classification and new anchor.
- Range lookup used by canonical addresses: given an old address, report whether its words changed in the latest version and where they are now.
- Test fixtures with synthetic anchored items to prove the M2 re-anchoring contract.

**Out**
- Re-anchoring workflows, support carry-over, notifications: M2 and later.
- The version change summary written in plain language (M1-09 generates it from this data and the texts).

## Deliverables

- Mapping and classification package with tests
- Stored comparisons for every consecutive version pair of tracked bills
- Comparison data available to `read-api` and the UI

## Requirements

- Classification of a given version pair is deterministic.
- Classification depends on text, not on position alone: a renumbered but identical subsection is *moved*, not *changed*.
- Whitespace and formatting-only differences do not count as changes; punctuation and wording do.
- Every item and link records the version it was authored against; nothing silently migrates ([02 § Rules](../../02-versions-and-review-lifecycle.md#rules-and-invariants)).
- Prior versions stay readable.

## Suggested breakdown

1. Normalisation rules for comparing line text.
2. Section-level alignment (headings, citation labels, XML ids), then line-level alignment within and across sections.
3. Classification of single lines and ranges; best new location.
4. Storage and trigger from the new-version hand-off.
5. Old-address lookup for "changed since".
6. Tests: real multi-version bills (introduced vs. reported), plus synthetic edge cases (split section, merged paragraphs, renumbering, deleted section).

## Acceptance criteria

- For the M1 multi-version exit bill, every old anchor has a classification, and a manual check of 50 sampled anchors agrees with it.
  - **Met (operator decision, 2026-11-22):** the exit-bill fixtures have 18 to 45 lines per version, so 50 distinct sampled anchors do not exist across the changed pairs. The one differing pair (hr-4313 `ih` to `rh`) was checked in full and agrees with the classification.
- Opening an old-version line address whose words changed yields the notice data and a new location.
- Tests show a renumbered identical subsection classified *moved*.

## Open questions

- Which unit computes comparisons in M1? **Decided for the local phase:** each step runs as its own command over stored data; nothing chains steps automatically ([README § Example data and local triggers](../README.md#example-data-and-local-triggers)). Automatic chaining is a cloud-phase question. Remaining for the cloud phase: [architecture § Bill source § Recommendation](../../../architecture/README.md#recommendation) hands new versions to stale classification through the queue and `worker`, which arrive in M3.
- What rule or threshold separates *changed* from *removed*? Record it in [02](../../02-versions-and-review-lifecycle.md).
  - **Answered (M1-08):** word-multiset Dice similarity of at least 0.5; recorded in 02. Not yet tuned on real borderline pairs (fixtures too small).
- Are comparisons kept for non-consecutive version pairs?
  - **Answered (M1-08):** consecutive pairs plus each older version against the latest are stored; other pairs can be computed with `versiondiff.Compare`.
