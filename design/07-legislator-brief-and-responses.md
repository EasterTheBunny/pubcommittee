# 07 — Legislator Brief and Responses

## Purpose

This is the product's core output. Each legislator's office receives a short per-bill brief, written like an amendment memo, that a staffer can act on in under five minutes per item. Offices reply publicly, and participants see when their input lands.

## Scope

**In scope**
- Brief composition and ordering
- Delivery cadence and triggers
- Export for drafting offices
- Office responses and the per-member responsiveness record
- Detecting official amendments that match citizen suggestions ("closing the loop")

**Out of scope**
- How signals are gathered (see [03](03-review-features-and-support.md))
- Integrity flags' detection logic (see [09](09-integrity-moderation-and-privacy.md)); this doc only displays them

## Brief structure

1. **Header** — bill, official version the brief is based on, reporting period, total reviewers.
2. **Bill-level signal**
   - Support and opposition counts, raw and as share of reviewers.
   - Share of opposition citing *too complex*, *unrelated riders*, *disagree with policy*.
   - Sections most often tagged as riders, with any steward context note.
3. **Competing bills** — if the bill belongs to an issue, the alternatives with their counts and the neutral comparison ([04](04-bill-lineage-and-competing-bills.md)).
4. **Suggested edits** — brief-eligible items, grouped by section, ranked by support.
5. **Integrity disclosures** — spikes, referral surges, coverage and turnout notes ([09](09-integrity-moderation-and-privacy.md)).

### One suggested-edit item

| Field | Content |
| --- | --- |
| Suggestion | Citation and short form, e.g. "Sec. 4(b): replace 'shall' with 'may'" |
| Comparison | Side-by-side bill text and replacement, against the named official version |
| Why | The author's reason, in their words |
| Support, your district | *Later phase.* Count and share of district reviewers |
| Support, whole jurisdiction | Count and share |
| Competing suggestions | Other suggestions on the same lines, with counts |
| Top concern | Most active line-comment thread on the section, quoted |
| Steward review | Well formed / not reviewed; cross-reference notes |
| Status | Open, or the office's current response |

At launch, every member receives the same national figures. District and state breakdowns arrive once address confirmation exists ([08](08-onboarding-and-verification.md)). Support counts are split by verification tier once higher tiers exist.

## Delivery

| Trigger | Timing |
| --- | --- |
| Regular cadence | Weekly while a bill is in review and has new activity |
| Key events | Immediately before committee markup and floor votes |
| On demand | Office requests a current brief |

- Channels: email, and a brief page in the office's account.
- Export to formats drafting offices already use.
- Which offices receive a brief for a bill (all members, sponsors, committee members) must be defined — see open questions.

## Responses

Offices may mark each item:

| Status | Meaning |
| --- | --- |
| Adopted | Offered as an amendment |
| Under consideration | Being evaluated |
| Declined | Not pursued, with optional reason |
| *(no response)* | Shown publicly beside the member's name when the office has not answered |

- Responses are public and attributed to the office.
- Offices may also reply in a comment on the item.
- A response is optional, but silence is visible.

### Responsiveness record
- Per-member public record of **whether and how** an office answered, not whether it agreed.
- Rewards engagement, not obedience.

## Closing the loop

- When a new official version arrives ([02](02-versions-and-review-lifecycle.md)), compare its changes to open suggested edits on the same lines.
- On a match, notify every supporter of the suggestion that it appears in the official text.
- Matches are shown on the suggestion as "reflected in version X". A steward confirms ambiguous matches.

## Domain concepts

- **Brief** — office, bill, based-on version, period, generated content snapshot, delivery records.
- **Brief item** — reference to a suggested edit or bill-level aggregate, with the figures as of generation.
- **Office response** — office, item, status, reason, responder, timestamp. Public.
- **Responsiveness record** — derived per office: items received, answered, by status, median time to answer.
- **Amendment match** — suggestion, official version, match confidence, confirmation state.

## Rules and invariants

- A brief never names individual supporters.
- Briefs are snapshots; later changes do not alter a delivered brief.
- Responses never affect counts.
- Office accounts cannot signal or hide citizen content ([05](05-roles-and-stewardship.md)).

## Dependencies

- [03](03-review-features-and-support.md) eligible items and aggregates.
- [04](04-bill-lineage-and-competing-bills.md) competing bills.
- [05](05-roles-and-stewardship.md) official accounts and steward status.
- [08](08-onboarding-and-verification.md) tier and district splits.
- [09](09-integrity-moderation-and-privacy.md) disclosures.
- External: a source of committee markup and floor vote schedules.

## Open questions

- Which offices receive a given bill's brief: every member, sponsors and committee members, or opt-in?
- What makes a busy office answer? Options: public responsiveness records, press partnerships, local media digests.
- Should the brief include line comments that raise concerns on the same lines as a suggestion (support-only blind spot)?
- Which export formats do drafting offices actually use? To be learned from pilot offices.
- How much lead time is needed before markup and floor votes, and how reliable are schedule sources?
