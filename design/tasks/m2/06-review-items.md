# M2-06 — Line Comments and Suggested Edits

**Phase:** local, then cloud phase in `dev`.

## Goal

Signed-in testers write line comments and suggested edits on real bill versions; both persist, stay anchored to their lines, and are served by `read-api` labelled as `user` content.

## Read first

- [03 § Line comments](../../03-review-features-and-support.md#line-comments), [§ Suggested edits](../../03-review-features-and-support.md#suggested-edits), [§ Rules and invariants](../../03-review-features-and-support.md#rules-and-invariants)
- [02](../../02-versions-and-review-lifecycle.md) (versions, stale items) and M1-08's stored comparisons
- [architecture § Document store](../../../architecture/README.md#document-store-public-mutable)
- [11 § Profiling and re-identification](../../11-ai-security.md#profiling-and-re-identification)

## Scope

**In**
- Create, edit while unsupported, and withdraw line comments and suggested edits on an anchor or line range of a version. A suggested edit covers one contiguous range in one section; AI-assisted drafting is allowed with no label (both decided in [M2-00](00-decisions-and-prerequisites.md)).
- Suggested edit stores replacement text for its range; diff view data for the UI.
- Public document per item: anchor, version, text (`user`), author's per-bill pseudonym (decided in [M2-00](00-decisions-and-prerequisites.md)), day, state (open, stale, removed).
- Stale state on a new version from M1-08's comparison; the item keeps its original anchor.
- `read-api` routes from [M2-04](04-api-contract-writes.md).
- Public-content warning shown before the first write (UI in [M2-15](15-web-ui-interactive.md); copy here).

**Out**
- Re-anchoring (steward action, [M2-10](10-moderation-and-steward-actions.md)). Support counts ([M2-07](07-signals-and-markup-requests.md), [M2-08](08-batcher-and-published-counts.md)).

## Requirements

- **Local run:** adds what it builds to `compose.yaml` and `scripts/local.sh` under resource limits, so `scripts/local.sh up` shows it ([environments § Local run](environments.md#local-run)).
- No author listing or search; the public document carries no account id.
- No automatic support by the author ([MILESTONES § M2](../../MILESTONES.md#m2--interactive-prototype)).
- Official text is never modified (global principles).
- Writes only through `write-api` ([M2-05](05-write-api-sign-in-sessions.md)).

## Acceptance criteria

- A tester's comment and suggestion survive a restart and appear on the version page at their anchor, labelled `user`.
- After a new version is ingested, an item on a changed line is marked stale and still opens at its original version.
- A test shows no route returns an author's items as a list.

## Open questions

- None; ranges, names, and AI-drafted text are decided ([M2-00](00-decisions-and-prerequisites.md)).
