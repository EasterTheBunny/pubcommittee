# 06 — Sharing

## Purpose

Let anyone share a whole bill or any part of it, down to a single line, so social media posts can quote a bill accurately and link straight into review. Every link opens on exactly that text with the full bill one tap away.

## Scope

**In scope**
- Shareable targets and their links
- Version pinning of links
- Preview cards and downloadable quote images
- Landing behavior and sign-in prompts

**Out of scope**
- Referral spike disclosure in briefs (see [07](07-legislator-brief-and-responses.md), [09](09-integrity-moderation-and-privacy.md))

## Shareable targets

| Target | Entry point | Preview card shows |
| --- | --- | --- |
| Whole bill | Share button at top of bill | Title, number, sponsor, status, one-line change explanation, current support and opposition |
| Section | Share icon by section heading | Heading and first lines, bill title, share of opposers tagging it a rider (if notable) |
| Line, block, or range | Select a line, then Share. A line with nested lines selects its block (the line and everything under it); a custom end narrows or widens the range; a line with nothing nested shares alone | Quoted words, section citation, bill title |
| Suggested edit | Share button on suggestion | Side-by-side comparison, reason, support count |
| Comment thread | Share button on thread | Quoted line and opening comment |

Anyone, signed in or not, can share.

## Link behavior

1. **Pinned to a version.** Every link records the official version it was shared from (via the anchor, [01](01-bill-ingestion-and-display.md)).
2. **Changed since sharing.** If later versions changed that text, the page opens on the shared words with a notice and a one-tap view of the current version. An old post never misquotes the bill.
3. **Landing.** The page scrolls to and highlights the shared text. The full bill, comments, and suggestions are reachable from there.
4. **Signed-out readers** can read everything and are invited to join only when they try to comment or signal.
5. **Closed bills** still resolve; the page shows the bill's final state.
6. **Combined or linked items** (duplicates combined, bills linked by lineage) resolve to the surviving item with a note.
7. **Server-resolvable.** Every shared target has a canonical address the server can resolve without client-side state, so crawlers and automated clients reach the same text people do ([10](10-machine-access.md)).

## Distribution

- Share targets: common social networks and messaging apps, plus copy link.
- Downloadable quote-card image for platforms that strip link previews.
- Preview metadata must be available to link-unfurling crawlers without sign-in.
- Broader automated reading (structured forms, exports, feeds) is covered in [10](10-machine-access.md).

## Privacy rules

- A share card **never** shows the sharer's own support or opposition. Sharers may add it in their own post text.
- Share counts are **not** part of the signal to legislators.
- Referral spikes from shared links **are** disclosed in the brief ([09](09-integrity-moderation-and-privacy.md)).
- Links must not encode the sharer's identity in a way that reveals who shared, beyond optional aggregate referral attribution.

## Domain concepts

- **Share target** — `(kind, anchor or item id, version)`. Anchor form: [10 § Canonical addresses](10-machine-access.md#canonical-addresses).
- **Preview card** — rendered text and image derived from the target at request time, with counts as of that moment.
- **Referral** — an aggregate record that a visit or signal arrived via a shared link (used only for spike disclosure).

## Dependencies

- [01](01-bill-ingestion-and-display.md) stable anchors.
- [02](02-versions-and-review-lifecycle.md) version history and change detection.
- [03](03-review-features-and-support.md) items and counts shown on cards.
- [09](09-integrity-moderation-and-privacy.md) referral disclosure; moderated content must not render in cards.
- [10](10-machine-access.md) canonical addresses and machine-readable forms of shared targets.

## Open questions

- Should links be human-readable (bill number and section path) or opaque short codes, or both? [10](10-machine-access.md) requires a human-readable canonical address; short codes would be optional aliases.
- Do preview counts update live, or freeze at share time for cached cards?
- What happens when a shared comment thread is later moderated away?
- Should a share of a stale suggestion show its stale state prominently on the card?
