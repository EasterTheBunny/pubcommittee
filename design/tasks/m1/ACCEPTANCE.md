# M1 Acceptance

Record of the M1 gates ([16-acceptance.md](16-acceptance.md)).

## Local gate

Passed on 2026-11-22 (run `r20261122-1412`, `m1-integration`). The evidence table maps every exit criterion in [MILESTONES § M1](../../MILESTONES.md#m1--real-bill-text-read-only) to its check: [test/acceptance/M1-16/EVIDENCE.md](../../../test/acceptance/M1-16/EVIDENCE.md). Run it with `RUN_ID=<id> sh test/acceptance/M1-16/stack.sh`.

Generation accuracy: [test/accuracy/report.md](../../../test/accuracy/report.md). Citation resolution: [test/acceptance/M1-07/report.md](../../../test/acceptance/M1-07/report.md).

Pending operator steps and open questions, with owners, are listed in the evidence table.

Operator feedback from using the local run: [FEEDBACK.md](FEEDBACK.md).

## Hosted gate

Passed on 2026-11-27 in `staging`. The same check script as the local gate passed all 64 checks against the hosted API edge and web UI, with selection policy v1 published; the operator checked by hand the explanations of the three exit bills against the official text, installation on a phone and a desktop with offline reading, and held the reader session. Evidence: [test/acceptance/M1-16/EVIDENCE.md § Hosted gate](../../../test/acceptance/M1-16/EVIDENCE.md#hosted-gate-staging-2026-11-26).

Open questions answered in M1 moved to decided behavior in [01](../../01-bill-ingestion-and-display.md) (Lines and addressing, Auto-generated text, Selection rules) and [10](../../10-machine-access.md) (Read access). Follow-up tasks recorded after the gate: M1-05b, M1-07b, M1-09b, M1-09c, M1-10b.
