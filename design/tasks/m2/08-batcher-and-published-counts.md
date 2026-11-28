# M2-08 — `job-batcher` and Published Counts

**Phase:** local, then cloud phase in `dev`.

## Goal

Counts reach the public only through `job-batcher`: at a fixed interval it totals signals, applies the small-count threshold, writes a snapshot, and publishes counts that one person's action cannot be traced through within a batch window.

## Read first

- [03 § Count display](../../03-review-features-and-support.md#count-display)
- [11 § Inferring private signals](../../11-ai-security.md#inferring-private-signals) and [09 § Inference threats](../../09-integrity-moderation-and-privacy.md#inference-threats)
- [architecture § SQL](../../../architecture/README.md#sql-private-small) (`snapshots`) and [§ Scheduled jobs](../../../architecture/README.md#scheduled-jobs)

## Scope

**In**
- `cmd/job-batcher`, hourly, with N as configuration starting at 5 and exact counts (decided in [M2-00](00-decisions-and-prerequisites.md)): totals per item from `signals`, raw count and share of reviewers of the bill, "fewer than N" below the threshold, consistent suppression across a bill's items, one `snapshots` row per item per batch.
- Published counts as public documents with batch id and as-of time; `Snapshots` read port in `read-api` (architecture § Ports).
- Reading measured only in aggregate: the reviewer denominator counted without per-person records.
- Brief-eligibility flag when an item reaches the minimum support count (03 rule 6), for [M2-11](11-brief-page.md).

**Out**
- Spike disclosure and the detector (M3). Noise (not chosen, [M2-00](00-decisions-and-prerequisites.md)).

## Requirements

- **Local run:** adds what it builds to `compose.yaml` and `scripts/local.sh` under resource limits, so `scripts/local.sh up` shows it ([environments § Local run](environments.md#local-run)).
- No count changes between batches; no write response exposes a total.
- A published count never moves by exactly one person's action inside one batch where it would identify them; suppression stays consistent between related counts.
- Snapshots are the audit record of what was published.

## Acceptance criteria

- A test with a single test account adding and withdrawing a signal inside one batch window shows no observable change in published counts (MILESTONES § M2 exit).
- Counts below N show as "fewer than N" everywhere they appear, including exports.
- Snapshot rows match every published count.
- **Cloud phase:** the job runs on its schedule in `dev`.

## Open questions

- The reviewer denominator ([03 § Open questions](../../03-review-features-and-support.md#open-questions)). Final batch interval and N are an M2 exit decision.
