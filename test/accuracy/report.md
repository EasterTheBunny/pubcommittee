# Accuracy report (M1-09)

Generated 2026-11-23 by `cmd/accuracy-report`.

- Generator: `claude-opus-5-5` (live run)
- Grader: `claude-sonnet-5-5` (must differ from the generator)
- Bills: us-119-hr-4313, us-119-hr-3062, us-119-hr-3427
- Prompt versions: bill-explanation=v1-f2a32231, bill-summary=v1-58de141b, change-explanation=v1-c8de06df, version-changes=v1-0adc248d
- Reproduce: `accuracy-report --generator claude --grade` then `accuracy-report --generator claude --grade-planted --crs --out test/accuracy` (live runs, operator-approved)

## Per task

| Task | Outputs | Ready | Unverified | Withheld | Refused/truncated/invalid | Supported | Contradicted | Fidelity fail | Amendment fail | Neutral-wording fail | Answered by fallback |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| bill-explanation | 16 | 14 | 1 | 0 | 1 | 100.0% (23/23) | 0.0% (0/23) | 0.0% (0/16) | 0.0% (0/16) | 0.0% (0/16) | 0 |
| bill-summary | 12 | 6 | 6 | 0 | 0 | 100.0% (44/44) | 0.0% (0/44) | 0.0% (0/12) | 0.0% (0/12) | 0.0% (0/12) | 0 |
| change-explanation | 54 | 34 | 19 | 0 | 1 | 88.4% (152/172) | 5.2% (9/172) | 7.4% (4/54) | 16.7% (9/54) | 0.0% (0/54) | 0 |
| version-changes | 4 | 2 | 2 | 0 | 0 | 100.0% (3/3) | 0.0% (0/3) | 50.0% (2/4) | 0.0% (0/4) | 0.0% (0/4) | 0 |

Validator failure rates are shares of outputs with at least one finding. Supported and contradicted shares are shares of graded statements, reported separately. Answered by fallback counts outputs a fallback model produced instead of the generator.

## Planted errors: catch rate per check

Known errors injected into good outputs. Measured on the live outputs. Planted counts are smaller than in the harness report because only statements of the ready outputs were used.

| Planted error | Planted | Meant for | Caught by that check | Caught by the grader |
| --- | --- | --- | --- | --- |
| changed-number | 45 | fact-fidelity | 95.6% (43/45) | 80.0% (36/45) |
| swapped-section | 8 | grader only | n/a | 100.0% (8/8) |
| reversed-effect | 3 | amendment-consistency | 66.7% (2/3) | 100.0% (3/3) |
| evaluative-wording | 56 | neutral-wording | 100.0% (56/56) | 96.4% (54/56) |
| invented-number | 56 | fact-fidelity | 100.0% (56/56) | 98.2% (55/56) |
| nonexistent-anchor | 56 | anchors | 100.0% (56/56) | 26.8% (15/56) |

## Operator sample and grader agreement

Sample prepared: 30 statements (`test/accuracy/operator-sample.md` and `.json`). Grader-to-operator agreement: pending the operator's labels (operator step; labels are blank).

## CRS summary coverage (bill summaries)

| Version | CRS points | Covered | Contradicted |
| --- | --- | --- | --- |
| us/119/hr/3062/ih | 7 | 100.0% (7/7) | 0 |
| us/119/hr/3427/ih | 6 | 100.0% (6/6) | 0 |

## Proposed M3 target

Proposed shape, from the live numbers, for operator review: zero published statements graded contradicted; contradicted share of graded statements at or below 1% for every task (live: change-explanation 5.2% (9/172), already withheld from publication); nonexistent-anchor catch rate 100% by the deterministic check (live: 100%); the grader is not an anchor check (26.8%). Not final until the operator grades the sample.

## Live run facts (stage L3)

- Run: 86 tasks, one Message Batch per bill, generator `claude-opus-5-5` at effort medium, grader `claude-sonnet-5-5`. Batches carry no fallbacks, so 0 outputs were answered by a fallback model; models that answered: `claude-opus-5-5` (generation) and `claude-sonnet-5-5` (grading).
- Outcomes: 56 ready, 28 unverified, 2 invalid (schema: an empty statement), 0 refusals, 0 truncations.
- Unverified and invalid outputs are stored but not served. The grader graded the 56 ready outputs (the stored grades).
- Served outputs are labelled `auto`, `review_state: unreviewed`; none labelled steward-checked (read-api check on the local store).

## Spend (upper-bound ledger, USD, from stored token counts at list prices; cache writes priced 1.25x, batch 0.5x)

| Phase | USD |
| --- | --- |
| Generation (batch) | 1.49 |
| Grader pass (sync, estimated: grade token counts are not stored) | at most 1.5 |
| Planted-error grading and CRS coverage (sync, estimated) | at most 4.5 |
| Total | at most 7.5 (cap 20.00) |

## Validator false positives on real model output

The deterministic validators were first tested on synthetic text. On 86 real outputs, 28 were marked unverified. Reading the findings, many are validator weaknesses rather than model errors:

- Amendment consistency (9/54 change explanations): "removed/inserted text ... not named" fires when the model paraphrases the quoted text instead of repeating it. Likely false positives for most of them; the grader found 0 contradicted among the cases checked is not established, so the operator should sample these.
- Bill summary (6/12 unverified): provisions such as `42 U.S.C. 1395cc-7(a)(1)` are rejected as "not in the input" although the model cited real provisions. This looks like a citation-form mismatch (en dash in the cited form, sub-paragraph suffix) rather than an invented provision.
- Fact fidelity (4/54 change explanations, 2/4 version summaries): numbers such as 2500000 and 1866 not literally in the cited lines. Some are formatting (a figure written with commas) and some derived values; unclear which are real.
- "statement N: no anchor" and "no statement with role changes" (several change explanations): the model gave no anchor or no changed-role statement; these are model format failures and correct rejections.
- Grader: contradicted 9/172 statements (5.2%) on change-explanation only; supported 88.4%. Unverified outputs were not graded.

## Reversed-effect catch rate

The deterministic amendment-consistency check caught 2 of 3 live planted reversed-effect errors (66.7%); the grader caught 3 of 3 (100%). Sample is small (3).

## Proposals

- Fix the citation-form match for provisions with en dashes and sub-paragraphs, and compare amendment text after normalisation, then re-run to recount the false positives.
- Do not rely on the grader for nonexistent anchors (26.8%); keep the deterministic check.
