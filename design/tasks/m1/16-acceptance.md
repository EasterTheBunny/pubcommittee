# M1-16 — M1 Acceptance

## Goal

Prove each M1 exit criterion with recorded evidence, set measured targets for M3, and move answered open questions into decided behavior in the docs. It has two gates:
- **Local gate** (closes the local phase): every exit criterion proved on a local machine with the `local` target, live public sources, and a local live model run.
- **Hosted gate** (closes the cloud phase and M1): the same checks repeated in `staging`, and the reader session held there.

## Read first

- [MILESTONES § M1](../../MILESTONES.md#m1--real-bill-text-read-only) (in scope and exit criteria)
- Every M1 task's "Open questions" section

## Scope

**In**
- The three exit bills are decided in [M1-04](04-bill-xml-parser.md) (H.R. 4313, H.R. 3062, H.R. 3427 of the 119th Congress) and live in `fixtures/`: real, at least one with multiple official versions, together covering amendatory text, insertions, repeals, and freestanding text. Locally they are tracked by policy or by a local `bills track`.
- Evidence per exit criterion:
  1. **Three real bills display correctly with explanations**: review of every section against the official text; explanation spot-check.
  2. **Measured citation resolution rate and explanation accuracy**: reports from M1-07 (citation resolution) and M1-09 (`test/accuracy/report.md`: statements supported and contradicted against their cited lines, validator and grader catch rates, grader-to-operator agreement, CRS coverage), each with a target set for M3.
  3. **Answers recorded for M1 open questions** in [01](../../01-bill-ingestion-and-display.md#open-questions): line unit and non-prose content (M1-04).
  4. **Automated client fetches any line range by canonical address without running scripts, and the structured form matches the page**: a scripted check that fetches page HTML and the `read-api` resource for every line range of the exit bills and compares anchors and text.
  5. **Structure of machine responses documented and versioned**: published spec and versioning policy (M1-02).
- Check of the in-scope list: catalog backfill, pilot policy, admin actions logged, installable app offline, exports and feed, rate limits published, notice and crawler directives.
- Doc updates: collect every task's open questions and their answers; move answered questions to decided behavior; update [architecture § Milestone fit](../../../architecture/README.md#milestone-fit) and [§ Open decisions](../../../architecture/README.md#open-decisions); update `README.md` and `app/README.md` to say the code is M1 and point to the archived M0 demo.
- Invite a small group of readers (M1 users are internal and invited readers) and record what they could not understand.

**Out**
- Fixing gaps found: file them back to the owning task or as M2 work.

## Deliverables

- `design/tasks/m1/ACCEPTANCE.md` (or a section here) listing each criterion, the evidence, and a pass or fail
- Page-versus-structured-form check script, kept in CI
- Updated docs

## Requirements

- Evidence is reproducible: scripts and samples are in the repository, not only screenshots.
- Results are reported as measured, including failures.

## Suggested breakdown

1. Confirm the exit bills from `fixtures/README.md`.
2. Write the page-versus-structured-form check and add it to CI.
3. Collect M1-07 and M1-09 reports; propose M3 targets.
4. Walk the in-scope list locally (local gate), then in `staging` (hosted gate).
5. Reader session and notes.
6. Doc updates and the acceptance record.

## Acceptance criteria

- Local gate: every M1 exit criterion is marked pass on a local machine with linked evidence.
- Hosted gate: the same in `staging`, or the milestone is held open with the gap named.
- Open questions answered during M1 are moved to decided behavior in their feature docs.

## Open questions

- The exit bills are decided (M1-04). Cloud phase: are any tracked by administrator selection in `dev`, needing a public log entry?
- Which open questions from other tasks must be answered to close M1, and which can carry over to M2?
