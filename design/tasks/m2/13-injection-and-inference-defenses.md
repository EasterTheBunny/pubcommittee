# M2-13 — Instruction-injection and Inference Defenses

**Phase:** local; checked again in the hosted gate.

## Goal

The defenses MILESTONES § M2 lists hold across every service, and tests prove them: content classes everywhere, no user text where machines summarize, generation that never reads user content, and counts that leak nothing about one person.

## Read first

- [10 § Instruction injection](../../10-machine-access.md#instruction-injection), [§ Privacy under bulk access](../../10-machine-access.md#privacy-under-bulk-access)
- [11](../../11-ai-security.md) (threats and defenses, rules and invariants)
- [03 § Count display](../../03-review-features-and-support.md#count-display), [09 § Inference threats](../../09-integrity-moderation-and-privacy.md#inference-threats)

## Scope

**In**
- A seeded injection corpus (comments and suggestions written to steer automated readers) committed as fixtures.
- Checks across `read-api`, exports, feed, cards, and the brief page: every text field labelled; seeded text never in a summary or preview field; labelled `user` in every machine response.
- A dependency and data-flow test that `job-summarize` and every generation input exclude review items.
- Inference checks: batch-window test ([M2-08](08-batcher-and-published-counts.md)); no author auto-support; no listing or search by author; coarse timestamps; notifications without position detail; public-content warning present before the first write.
- The rule against automated submission and injection text as spam, drafted for the charter ([M2-17](17-testers-and-charter.md)).

**Out**
- Behavioral automation detection and disclosure (M3, `job-detector`).

## Requirements

- Each check lives beside the code it guards and runs in CI; this task owns the cross-service suite in `test/acceptance/M2-13/`.

## Acceptance criteria

- The seeded injection corpus appears in no summary, preview, or card field in any response, and is labelled `user` in every machine response (MILESTONES § M2 exit).
- A test fails if any generation input path can read a review item.
- The single-account batch-window test passes against the running stack.

## Open questions

- Where a comment arguing with AI summaries becomes manipulation ([10 § Open questions](../../10-machine-access.md#open-questions)); a charter question for [M2-17](17-testers-and-charter.md).
