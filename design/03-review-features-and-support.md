# 03 — Review Features and Support

## Purpose

Give reviewers three ways to respond to a bill (comment on lines, suggest exact changes, give an overall verdict) and define how support is counted.

## Scope

**In scope**
- Line comments and threads
- Suggested edits and their side-by-side display
- Bill-level support/oppose with reason tags, including rider tagging
- Support rules, minimum threshold, and how counts are displayed

**Out of scope**
- Who may perform these actions (see [05](05-roles-and-stewardship.md), [08](08-onboarding-and-verification.md))
- How items reach legislators (see [07](07-legislator-brief-and-responses.md))
- Abuse handling (see [09](09-integrity-moderation-and-privacy.md))

## Features at a glance

| Feature | Scope | Author writes | Signal |
| --- | --- | --- | --- |
| Line comment | One line or a range | Questions, concerns, context, local impact | None |
| Suggested edit | One line or a range | Exact replacement text or deletion, plus a short plain-language reason | Support only |
| Bill-level signal | Whole bill | Optional comment and reason tags | Support or oppose |

## Line comments

- Any eligible reviewer can comment on any line or line range.
- Selecting a line that has nested lines selects its whole block by default ([01](01-bill-ingestion-and-display.md)). A comment may then narrow it to a custom range (for example lines 5–7 of the block 5–8); a line with nothing nested under it can be selected alone.
- Threaded replies.
- No count, no reactions that act as a signal. Comments exist for understanding and debate.
- Steward replies may be marked **steward context** (sourced facts) — see [05](05-roles-and-stewardship.md).
- Activity measure per thread (e.g. participants, replies) is used to pick the **top concern** per section for the brief.

## Suggested edits

**Content**
- Target anchor: one line or a contiguous range, in a specific official version.
- **The target always includes nested lines.** If the range includes a line with lines nested under it, the target extends to include all of them: an edit to a lead-in line ("Congress finds the following:") changes what its items mean, so they are part of the edit. Edits may still target a nested line alone (line 7 of the block 5–8). The author is shown when and why the range was extended.
- Replacement text (may be empty for a deletion).
- Short plain-language reason.

**Display**
- Side-by-side: bill lines beside the replacement, deletions struck, insertions marked.
- Multiple suggestions on overlapping lines are shown together, ranked by support.
- Each shows its steward status: *well formed*, *not reviewed*, or a pending wording fix.

**Revision**
- The author may revise their suggestion. Every revision is kept and viewable.
- A revision that changes meaning resets support; a minor fix (e.g. accepting a steward wording fix) keeps it. Supporters are notified either way.

**Steward interactions** (performed under [05](05-roles-and-stewardship.md))
- Mark well formed
- Offer a wording fix; the author accepts or rejects
- Combine duplicates; supporters keep their support on the combined item

## Bill-level signal

- Each reviewer may mark the bill **Support**, **Oppose**, or leave it unmarked.
- Optional comment explaining the position. A signal comment is **never shown with its author**: it is private to the author and may reach the brief only as an anonymous excerpt, released in batches with other excerpts ([11](11-ai-security.md)).
- An Oppose may carry one or more reason tags:
  - **Too complex** — hard to follow or does too much at once.
  - **Unrelated riders** — the reviewer selects which sections they consider riders.
  - **Disagree with the policy** — opposition to substance.
- Reviewers **cannot** propose splitting or restructuring the bill.

### Rider tags
- Tags are per-section selections attached to an Oppose.
- Aggregated per section: count, and share of opposers tagging it.
- Stewards may attach a factual context note to a heavily tagged section; tags themselves are never removed ([05](05-roles-and-stewardship.md)).

## Support rules

1. One support per reviewer per suggested edit.
2. One support-or-oppose per reviewer per bill.
3. All signals are withdrawable while the bill is in review.
4. Reviewers may support any number of items, including competing suggestions on the same lines.
5. Suggested edits have **no opposition, no majority, and no pass/fail**.
6. An item becomes **brief-eligible** at a minimum support count (example: 25 supporters in the jurisdiction).
7. Signals are frozen when the bill closes.

### Count display
Every count is shown two ways:
- Raw number.
- Share of reviewers of this bill (denominator: reviewers who opened/reviewed the bill).

Counts are published in batches at a fixed interval, never updated per event, so a change cannot be tied to one person's action. Counts below a small threshold are shown as "fewer than N" ([11](11-ai-security.md)).

### Why suggested edits are support-only
The only way to push back on a suggestion is to write a better one or explain the problem in a line comment. This rewards constructive work and removes the incentive to brigade a suggestion down. Opposition exists where it matters most: the whole bill.

## Domain concepts

- **Comment thread** — anchor, root comment, replies, author display names, timestamps, moderation state.
- **Suggested edit** — anchor, replacement text, reason, author, revision history, steward status, combined-into link, stale state, support count.
- **Bill signal** — reviewer, bill, value (support/oppose), optional comment, reason tags, rider section selections, timestamps.
- **Support** — reviewer, suggested edit, timestamp, verification tier at time of support. Private.
- **Reviewer of a bill** — the denominator population. Needs a precise definition (see open questions).

## Rules and invariants

- Individual signals are private; only aggregates are shown ([09](09-integrity-moderation-and-privacy.md)).
- Signals carry no weighting by tier, role, or reputation.
- Authors do not automatically support their own suggested edits.
- No published count changes in response to one person's action within an attributable window ([11](11-ai-security.md)).
- Reading of a bill (for the denominator) is measured only in aggregate, never stored per person.
- Notifications about supported items carry no position detail beyond a link to the item.
- People are told, when writing, that comments and suggestions are public and may reveal their position.
- Stewards' and officials' own signals carry no extra weight; legislator/staff accounts cannot signal at all ([05](05-roles-and-stewardship.md)).

## Dependencies

- [01](01-bill-ingestion-and-display.md) for anchors.
- [02](02-versions-and-review-lifecycle.md) for stale handling.
- [08](08-onboarding-and-verification.md) for eligibility and tier recorded on each signal.
- [07](07-legislator-brief-and-responses.md) consumes eligible items and aggregates.
- [11](11-ai-security.md) inference threats on counts and comments.

## Open questions

- **Denominator:** what counts as "reviewed this bill" — opened it, scrolled a section, signed in on it?
- **Support-only blind spot:** should the brief also count line comments that raise concerns on the same lines as a suggestion?
- **Delivery minimum:** is 25 right, and should it scale with jurisdiction size?
- **Small-count threshold and batch interval:** what N and what interval ([11](11-ai-security.md))?
  - **Decided (operator, 2026-11-28, M2 prototype):** batches hourly; N is a configuration value that grows with the user base, starting at 5. Exact counts with batching and suppression, no noise. Final values are an M2 exit decision ([M2-00](tasks/m2/00-decisions-and-prerequisites.md)).
- **Rider threshold:** how many tags before the brief calls out a section, and should a steward first confirm the section is unrelated?
- Can a suggested edit span multiple non-contiguous ranges or sections?
  - **Decided (operator, 2026-11-28):** no. A suggested edit replaces one contiguous line range in one section; several changes are several suggestions.
- Are line comments allowed on sections tagged as riders, linked to the tags?
