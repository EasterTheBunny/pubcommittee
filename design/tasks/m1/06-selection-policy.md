# M1-06 — Selection Policy and Tracked Set

## Goal

Implement the published, versioned bill selection policy in pilot mode, evaluate it against catalog entries, and set each bill's markup status and tracked reasons. The tracked set for M1 is "committee action and a focus topic" (healthcare, deficit reduction, affordability), plus companions and administrator selections.

## Read first

- [01 § Bill selection](../../01-bill-ingestion-and-display.md#bill-selection) (candidates, rules 1–6, modes, transparency)
- [architecture § Bill selection](../../../architecture/README.md#bill-selection) (policy shape, signals used, markup status)
- [05 § Platform administration](../../05-roles-and-stewardship.md#platform-administration) (phase rule)
- [09 § Coverage neutrality](../../09-integrity-moderation-and-privacy.md#coverage-neutrality)

## Scope

**In**
- Policy document schema (`version`, `mode`, `phase`, `measure_kinds`, `exclude_kinds`, `committee_activity`, `follow_companions`, `topics`), stored immutable per version in `selection_policy`.
- Initial policy version 1: `mode: pilot`, `phase: early_pilot`, with draft topic terms from the legislature's subject vocabulary and policy areas.
- Pure evaluation function: catalog entry plus policy in, `(markup_status, exclusion reason, tracked reasons)` out.
- Rules: measure kind, committee action beyond referral, topic match by policy area, subjects, and title terms, companion following, once tracked stays tracked.
- Evaluation of catalog entries written by pulls ([M1-05](05-manual-bill-pull-and-storage.md)); newly tracked bills get `tracked: {since, policy, reasons}` and become eligible for a text pull. In the local phase, evaluation runs inside each pull and as its own command over the stored catalog (`job-ingest --evaluate`).
- Tracked-set breakdown: counts by topic, by sponsor party, and administrator-selected. The architecture publishes it with each `job-ingest` run; when it is published in M1 is an open question.
- Plain-language scope statement derived from the current policy, for the UI.

**Out**
- Markup requests, nominations, steward decisions, `markup_queue`, `markup_decisions`: M2/M3.
- Publishing the policy (admin action, M1-12) — this task supplies the operation it calls.

## Deliverables

- Policy schema and evaluator package with table-driven tests
- Policy version 1 file, reviewed and committed
- `job-ingest` integration and breakdown output
- Scope statement generator

## Requirements

- **Process signals and the legislature's own subject indexing only.** Sponsor party, position, and predicted outcome are never inputs ([01 § Transparency](../../01-bill-ingestion-and-display.md#transparency)). Party appears only in the disclosure counts, computed after selection.
- Referral alone never counts as committee action.
- Every tracked bill records the policy version and the rules that selected it.
- A tracked bill is never untracked by a policy change; only an administrator error correction can untrack (M1-12).
- Excluded kinds carry a human-readable reason (for example "privileged resolution: a yes-or-no question rather than drafting").
- Title terms let a bill match before subject indexing arrives; subjects confirm later. A bill matched by title only is recorded as such.

## Suggested breakdown

1. Policy schema and validation; immutable storage per version.
2. Measure-kind and exclusion rules with reasons.
3. Committee-activity rule from `BILLSTATUS` actions and committee records.
4. Topic matching: policy area, subjects, title terms; draft term lists with sources.
5. Companion following from related bills.
6. Evaluator integration at the point the open question settles.
7. Disclosure breakdown and scope statement.
8. Table-driven tests from real catalog entries, including edge cases (referral only, title match before indexing, war powers resolution, companion of a tracked bill).

## Acceptance criteria

- Evaluator tests cover each rule and each markup status.
- On the local store, every catalog entry has a markup status; tracked bills list their reasons and policy version.
- **Cloud phase:** the same in `dev`.
- Publishing policy version 2 with a narrower topic list leaves every previously tracked bill tracked.
- A test fails if sponsor party is read by the evaluator (for example, evaluator input type has no party field).
- Breakdown counts are published.

## Open questions

- When does policy evaluation run? **Decided for the local phase:** each step runs as its own command over stored data; nothing chains steps automatically ([README § Example data and local triggers](../README.md#example-data-and-local-triggers)). Automatic chaining is a cloud-phase question.
- Re-evaluation after a new policy version: **Decided:** stored entries are re-evaluated on their next evaluation; locally, `job-ingest --evaluate` over the stored catalog. Tracked bills are never untracked by it.
- Tracked-set breakdown: **Decided:** written by every evaluation run. (M1-06: also by every pull, to `exports/tracked-set-breakdown.json`; a `read-api` route is a follow-up.)
- Which subject terms define each focus topic, and who maintains them ([01](../../01-bill-ingestion-and-display.md#open-questions), [architecture](../../../architecture/README.md#open-decisions))? M1 drafts them; confirmation is before the pilot.
- Title-only matches: **Decided:** enter the tracked set, recorded as title-only ([architecture § Bill selection](../../../architecture/README.md#bill-selection)).
