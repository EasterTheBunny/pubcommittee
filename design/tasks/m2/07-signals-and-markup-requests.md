# M2-07 — Support, Bill-level Signals, and Markup Requests

**Phase:** local, then cloud phase in `dev`.

## Goal

Testers support suggested edits, give a bill-level support-or-oppose signal, and request markup of eligible catalog bills; each signal is private, one per person per item, withdrawable, and countable only in batches.

## Read first

- [03 § Bill-level signal](../../03-review-features-and-support.md#bill-level-signal), [§ Support rules](../../03-review-features-and-support.md#support-rules)
- [01 § Markup requests](../../01-bill-ingestion-and-display.md#markup-requests) and [architecture § Markup requests](../../../architecture/README.md#markup-requests)
- [architecture § SQL](../../../architecture/README.md#sql-private-small) (`signals`, `my_signals`, `request_reasons`)
- [11 § Inferring private signals](../../11-ai-security.md#inferring-private-signals)

## Scope

**In**
- Support on a suggested edit; support or oppose on a bill; markup request on a catalog bill whose markup status is *eligible*, with an optional private reason.
- Withdraw any of them while the bill is in review; signals frozen when the bill closes.
- A person's own list from `my_signals` (decrypted on request) to view and withdraw.
- Rider tags ([03 § Rider tags](../../03-review-features-and-support.md#rider-tags)): a reviewer tags a section as unrelated to the bill's purpose; one tag per person per section, withdrawable, counted in batches like other signals (in M2, decided in [M2-00](00-decisions-and-prerequisites.md)).

**Out**
- Publishing counts ([M2-08](08-batcher-and-published-counts.md)). Steward review of requests and decisions (M3).

## Requirements

- **Local run:** adds what it builds to `compose.yaml` and `scripts/local.sh` under resource limits, so `scripts/local.sh up` shows it ([environments § Local run](environments.md#local-run)).
- `signal_key = HMAC(k_signal, account_id ‖ item_id)`; no account id in `signals` or `request_reasons`.
- Suggested edits have no opposition, majority, or pass/fail (03 rule 5).
- No change to any public count at signal time; nothing in a write's response reveals a current total.
- Markup request reasons are never published; shown to stewards without names (M3).

## Acceptance criteria

- A second support by the same tester on one item is refused; a withdrawal removes it from the next batch.
- A markup request on a bill that is not *eligible* is refused.
- A copy of the SQL tables without the keys cannot link any signal to an account (test).
- A tester sees and withdraws their own signals; no other tester can.

## Open questions

- Rider threshold before the brief calls out a section ([03 § Open questions](../../03-review-features-and-support.md#open-questions)); M2 shows the count only.
