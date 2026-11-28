# M2-14 — Review Items in Exports and the Feed

**Phase:** local, then cloud phase in `dev`.

## Goal

Bulk exports carry review items keyed by item with per-bill pseudonyms, and the change feed carries review-item and removal events, so machines read the same public record people see.

## Read first

- [10 § Bulk exports](../../10-machine-access.md#bulk-exports), [§ Change feed](../../10-machine-access.md#change-feed), [§ Privacy under bulk access](../../10-machine-access.md#privacy-under-bulk-access)
- M1-11's `job-exports` and feed

## Scope

**In**
- Per-bill export of review items: item id, anchor, version, kind, text (`user`), per-bill pseudonym, day, state, published counts from the latest batch.
- Feed events: item created, revised, stale, re-anchored, removed; removed items leave a tombstone without the text.
- The user-content licence, CC BY 4.0 attributed to the per-bill pseudonym, in every export header (decided in [M2-00](00-decisions-and-prerequisites.md)).

**Out**
- Audit export (M4).

## Requirements

- **Local run:** adds what it builds to `compose.yaml` and `scripts/local.sh` under resource limits, so `scripts/local.sh up` shows it ([environments § Local run](environments.md#local-run)).
- Per-bill pseudonyms in exports: one person's items cannot be joined across bills from exports alone.
- Counts follow the batch and "fewer than N" rules.
- No export or feed entry carries an account id, display name history, or time finer than a day.

## Acceptance criteria

- Exports include every non-removed review item of a test bill, with pseudonyms that differ between two bills for the same tester.
- A removal produces a feed event and drops the item's text from the next export.

## Open questions

- None.
