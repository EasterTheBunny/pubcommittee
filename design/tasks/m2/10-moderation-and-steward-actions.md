# M2-10 — Basic Moderation and Steward Actions

**Phase:** local, then cloud phase in `dev`.

## Goal

Testers flag content; a moderator removes content with a reason that lands in the public moderation log; a steward marks a suggestion well formed, offers a wording fix the author accepts or rejects, and re-anchors stale suggestions.

## Read first

- [09 § Moderation](../../09-integrity-moderation-and-privacy.md#moderation) (rules, actions, log, removed content)
- [05 § Steward activities](../../05-roles-and-stewardship.md#steward-activities) and [§ Platform administration](../../05-roles-and-stewardship.md#platform-administration)
- [architecture § Administration](../../../architecture/README.md#administration) (role grants and the public log from M1-12)

## Scope

**In**
- Flag: any signed-in tester; counted in aggregate only.
- Remove with a reason that names a published rule; the log entry does not republish the content or personal data; the item shows as removed.
- Suspend account: moderator, logged with reason; a suspended account cannot sign in or write (in M2, decided in [M2-00](00-decisions-and-prerequisites.md)).
- Steward actions, single-person: mark well formed; offer a wording fix (author decides); re-anchor a stale suggestion whose meaning is unchanged, or flag it changed.
- Role grants for test stewards and moderators through `admin-api` (`roles` table), each grant in the public log.
- Removal events in the change feed ([M2-14](14-exports-and-feed.md)).

**Out**
- Appeals and the community panel, combining duplicates, two-party sign-off (M3).

## Requirements

- **Local run:** adds what it builds to `compose.yaml` and `scripts/local.sh` under resource limits, so `scripts/local.sh up` shows it ([environments § Local run](environments.md#local-run)).
- Every privileged action public and attributed (global principles); attributed to a stable public role id such as `moderator-3`; the operator holds the mapping (decided in [M2-00](00-decisions-and-prerequisites.md)).
- Stewards and moderators act through the product's pages and `write-api`, not `admin-api`; only grants use `admin-api`.

## Acceptance criteria

- A removal appears in the public log with its rule and without the removed text.
- A wording fix changes the suggestion only after the author accepts.
- A re-anchored suggestion opens at its new anchor and keeps its support.
- A tester without a role cannot call a steward or moderator route.

## Open questions

- None; attribution and suspension are decided ([M2-00](00-decisions-and-prerequisites.md)).
