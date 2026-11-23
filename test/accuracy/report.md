# Accuracy report (M1-09)

Generated 2026-11-23 by `cmd/accuracy-report`.

- Generator: `fakegen` (harness only: deterministic outputs built from the bill text; no model quality is measured)
- Grader: `pending` (must differ from the generator)
- Bills: us-119-hr-4313, us-119-hr-3062, us-119-hr-3427
- Prompt versions: bill-explanation=v1-f2a32231, bill-summary=v1-58de141b, change-explanation=v1-c8de06df, version-changes=v1-0adc248d
- Reproduce: `accuracy-report --out test/accuracy`

## Per task

| Task | Outputs | Ready | Unverified | Withheld | Refused/truncated/invalid | Supported | Contradicted | Fidelity fail | Amendment fail | Neutral-wording fail |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| bill-explanation | 16 | 16 | 0 | 0 | 0 | pending (live run not approved) | pending (live run not approved) | 0.0% (0/16) (harness) | 0.0% (0/16) (harness) | 0.0% (0/16) (harness) |
| bill-summary | 12 | 12 | 0 | 0 | 0 | pending (live run not approved) | pending (live run not approved) | 0.0% (0/12) (harness) | 0.0% (0/12) (harness) | 0.0% (0/12) (harness) |
| change-explanation | 54 | 54 | 0 | 0 | 0 | pending (live run not approved) | pending (live run not approved) | 0.0% (0/54) (harness) | 0.0% (0/54) (harness) | 0.0% (0/54) (harness) |
| version-changes | 4 | 4 | 0 | 0 | 0 | pending (live run not approved) | pending (live run not approved) | 0.0% (0/4) (harness) | 0.0% (0/4) (harness) | 0.0% (0/4) (harness) |

Validator failure rates are shares of outputs with at least one finding. Supported and contradicted shares are shares of graded statements, reported separately.

## Planted errors: catch rate per check

Known errors injected into good outputs. The deterministic checks run in CI on any outputs, so these rates are measured now; the grader column needs a live run.

| Planted error | Planted | Meant for | Caught by that check | Caught by the grader |
| --- | --- | --- | --- | --- |
| changed-number | 74 | fact-fidelity | 100.0% (74/74) | pending (live run not approved) |
| swapped-section | 16 | grader only | n/a | pending (live run not approved) |
| reversed-effect | 16 | amendment-consistency | 62.5% (10/16) | pending (live run not approved) |
| evaluative-wording | 86 | neutral-wording | 100.0% (86/86) | pending (live run not approved) |
| invented-number | 86 | fact-fidelity | 100.0% (86/86) | pending (live run not approved) |
| nonexistent-anchor | 86 | anchors | 100.0% (86/86) | pending (live run not approved) |

## Operator sample and grader agreement

Sample prepared: 30 statements (`test/accuracy/operator-sample.md` and `.json`). Grader-to-operator agreement: pending (live run not approved).

## CRS summary coverage (bill summaries)

Coverage: pending (live run not approved). CRS summaries are references only and never reach the generating model.

## Proposed M3 target

Pending: a target needs the live numbers above. Proposed shape, to be filled from them: zero published statements marked contradicted by the grader; deterministic checks catching every planted number, amendment and wording error (measured above); grader-to-operator agreement and CRS coverage targets set from the first live run.

## Live run (stage L3): blocked on operator approval

Needs: the operator's key and approval; `job-summarize --generator claude` on the three exit bills, then `accuracy-report --generator claude --grade --grade-planted --crs`, then the operator grades the sample and the report is rerun with `--agreement`.

Estimate (3.5 characters per token, about 3,000 output tokens per task including thinking at effort medium):

| Step | Calls | Input tokens | Output tokens | Cost at list | Cost with batches at half |
| --- | --- | --- | --- | --- | --- |
| Generate (`claude-opus-5-5`, $4 / $20 per million) | 86 | about 258,000 uncached | about 258,000 | about $6.2 | about $3.1 |
| Grade the 86 outputs (`claude-sonnet-5-5`, $2 / $10) | 86 | about 172,000 | about 129,000 | about $1.6 | about $0.8 |
| Grade planted errors (6 kinds, about 20 outputs each) | 120 | about 240,000 | about 180,000 | about $2.3 | about $1.2 |
| CRS coverage | 3 | about 6,000 | about 4,500 | under $0.1 | under $0.1 |
| Total | 295 | about 676,000 | about 571,500 | about $10 | about $5 |

Per exit bill generation (input tokens uncached / tasks): us-119-hr-4313 about 44,600 / 20; us-119-hr-3062 about 189,000 / 53; us-119-hr-3427 about 24,000 / 13. Prompt caching of the shared bill text lowers the input share further. Output tokens dominate and are the uncertain part: twice the assumed thinking doubles the output lines.
