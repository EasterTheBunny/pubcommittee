# M2-11 — Brief Page from Live Data

**Phase:** local, then cloud phase in `dev`.

## Goal

A bill's legislator brief is generated from live data and viewable as a page, so testers and the operator can judge what an office would receive. Nothing is delivered.

## Read first

- [07 § Brief structure](../../07-legislator-brief-and-responses.md#brief-structure) and [§ One suggested-edit item](../../07-legislator-brief-and-responses.md#one-suggested-edit-item)
- [03 § Count display](../../03-review-features-and-support.md#count-display), [M2-08](08-batcher-and-published-counts.md)

## Scope

**In**
- Brief data from published counts, brief-eligible suggestions with their diff and anchors, bill-level signal totals, and the tracked-set disclosure; built from the latest batch only.
- A `read-api` route for brief data and an app page that renders it without scripts.
- Counts follow the batch and "fewer than N" rules.

**Out**
- `job-briefs`, delivery, office responses, responsiveness record (M3).

## Requirements

- **Local run:** adds what it builds to `compose.yaml` and `scripts/local.sh` under resource limits, so `scripts/local.sh up` shows it ([environments § Local run](environments.md#local-run)).
- No individual signal, supporter, or author name beyond what review items already show publicly.
- No user text in any summary field of the brief; user text appears only as quoted, labelled `user` content.

## Acceptance criteria

- For a test bill with seeded signals, the brief page lists the brief-eligible suggestions and totals from the latest batch, and matches the snapshot rows.
- The page renders without scripts.

## Open questions

- Support-only blind spot and delivery minimum ([03 § Open questions](../../03-review-features-and-support.md#open-questions)); the prototype uses the documented example values.
