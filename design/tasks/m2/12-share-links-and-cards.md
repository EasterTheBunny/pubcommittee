# M2-12 — Share Links and Preview Cards

**Phase:** local, then cloud phase in `dev`.

## Goal

Every shareable target has a real link and a preview card that social platforms render, with no sharer identity, no sharer position, and no user text in summary or preview fields.

## Read first

- [06](../../06-sharing.md) (shareable targets, link behavior, distribution, privacy rules)
- [10 § Canonical addresses](../../10-machine-access.md#canonical-addresses) and [§ Instruction injection](../../10-machine-access.md#instruction-injection)

## Scope

**In**
- Links to bills, versions, lines and ranges, comment threads, and suggested edits, on the human-readable canonical address only; cards show the latest batch when fetched (both decided in [M2-00](00-decisions-and-prerequisites.md)).
- Open Graph and similar metadata rendered server-side by the app; generated card images for line ranges and suggestions.
- Optional aggregate referral attribution that does not identify the sharer.

**Out**
- Referral-spike disclosure in the brief (M3, with the detector).

## Requirements

- **Local run:** adds what it builds to `compose.yaml` and `scripts/local.sh` under resource limits, so `scripts/local.sh up` shows it ([environments § Local run](environments.md#local-run)).
- A card never shows the sharer's support or opposition, and share counts are not part of any signal ([06 § Privacy rules](../../06-sharing.md#privacy-rules)).
- Card title and description use official or platform text only; user text appears only inside the page, labelled `user` ([10](../../10-machine-access.md#instruction-injection)).
- Card counts follow the batch and "fewer than N" rules.

## Acceptance criteria

- A shared suggested-edit link opens the suggestion at its anchor; its card shows the bill, the lines, and the platform-written label, and no user text in `og:title` or `og:description`.
- A seeded injection comment never appears in any card field (MILESTONES § M2 exit).
- A link carries no sharer identifier.
- Locally, the card fields are checked in the app's rendered HTML. **Cloud phase:** a social platform's card validator renders a card from `dev`.

## Open questions

- Cards for moderated or stale items ([06 § Open questions](../../06-sharing.md#open-questions)).
