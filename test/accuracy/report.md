# Accuracy report (M1-09)

Generated 2026-11-23 by `cmd/accuracy-report`.

- Generator: `claude-opus-5-5` (live run)
- Grader: `claude-sonnet-5-5` (must differ from the generator)
- Bills: us-119-hr-4313, us-119-hr-3062, us-119-hr-3427
- Prompt versions: bill-explanation=v1-f2a32231, bill-summary=v1-58de141b, change-explanation=v1-c8de06df, version-changes=v1-0adc248d
- Reproduce: `accuracy-report --generator claude --grade` after `job-summarize --revalidate` and `job-summarize --submit --retry --generator claude` (live runs, operator-approved); the earlier live run: `accuracy-report --generator claude --grade-planted --crs --out test/accuracy`

## Per task

| Task | Outputs | Ready | Unverified | Withheld | Refused/truncated/invalid | Supported | Contradicted | Fidelity fail | Amendment fail | Neutral-wording fail | Answered by fallback |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| bill-explanation | 16 | 16 | 0 | 0 | 0 | 100.0% (28/28) | 0.0% (0/28) | 0.0% (0/16) | 0.0% (0/16) | 0.0% (0/16) | 0 |
| bill-summary | 12 | 12 | 0 | 0 | 0 | 100.0% (92/92) | 0.0% (0/92) | 0.0% (0/12) | 0.0% (0/12) | 0.0% (0/12) | 0 |
| change-explanation | 54 | 45 | 9 | 0 | 0 | 88.4% (199/225) | 4.4% (10/225) | 1.9% (1/54) | 7.4% (4/54) | 0.0% (0/54) | 0 |
| version-changes | 4 | 2 | 2 | 0 | 0 | 100.0% (3/3) | 0.0% (0/3) | 50.0% (2/4) | 0.0% (0/4) | 0.0% (0/4) | 0 |

Validator failure rates are shares of outputs with at least one finding. Supported and contradicted shares are shares of graded statements, reported separately. Answered by fallback counts outputs a fallback model produced instead of the generator.

## Planted errors: catch rate per check

Known errors injected into good outputs. The deterministic checks run in CI on any outputs, so these rates are measured now; the grader column is from the earlier live run of 2026-11-22 (56 ready outputs) and was not re-run on the 75 now ready; the deterministic columns are measured on the 75 now ready outputs.

| Planted error | Planted | Meant for | Caught by that check | Caught by the grader |
| --- | --- | --- | --- | --- |
| changed-number | 63 | fact-fidelity | 96.8% (61/63) | 80.0% (36/45) (earlier run, 56 ready outputs) |
| swapped-section | 14 | grader only | n/a | 100.0% (8/8) (earlier run, 56 ready outputs) |
| reversed-effect | 3 | amendment-consistency | 66.7% (2/3) | 100.0% (3/3) (earlier run, 56 ready outputs) |
| evaluative-wording | 75 | neutral-wording | 100.0% (75/75) | 96.4% (54/56) (earlier run, 56 ready outputs) |
| invented-number | 75 | fact-fidelity | 100.0% (75/75) | 98.2% (55/56) (earlier run, 56 ready outputs) |
| nonexistent-anchor | 75 | anchors | 100.0% (75/75) | 26.8% (15/56) (earlier run, 56 ready outputs) |

## Operator sample and grader agreement

Sample unchanged from the earlier run: no statement in it changed status (`test/accuracy/operator-sample.md` and `.json`; 30 statements, labels blank). Grader-to-operator agreement (operator graded 2026-11-22; hand-written section): **30/30 (100%)**. By task: bill-explanation 9/9, bill-summary 9/9, change-explanation 9/9, version-changes 3/3. The grader also labelled all 30 `supported`, because the sample was drawn from served (ready) outputs only, so it holds none of the 10 grader-contradicted statements. It therefore shows that the grader's `supported` label held for every sampled statement (no false approvals found), but it does not measure whether the grader catches errors; the planted-error grader columns above are the evidence for that. A later sample should include grader-contradicted and unverified statements.

## CRS summary coverage (bill summaries)

| Version | CRS points | Covered | Contradicted |
| --- | --- | --- | --- |
| us/119/hr/3062/ih | 7 | 100.0% (7/7) | 0 |
| us/119/hr/3427/ih | 6 | 100.0% (6/6) | 0 |

(CRS comparison from the earlier run; not re-run.)

## Proposed M3 target

Proposed shape, from the live numbers, for operator review: zero published statements graded contradicted; contradicted share of graded statements at or below 1% for every task (live: change-explanation 5.2% (9/172), already withheld from publication); nonexistent-anchor catch rate 100% by the deterministic check (live: 100%); the grader is not an anchor check (26.8%). Not final until the operator grades the sample.

## Validator fixes and the follow-up run (hand-written, 2026-11-22)

The first live run (86 tasks: 56 ready, 28 unverified, 2 invalid) was re-checked offline with the fixed validators (`job-summarize --revalidate`, no model call), then the outputs still not ready were retried once (`--submit --retry`, a paid batch), and the new ready outputs were graded.

Outcomes of the 86 outputs (ready / unverified / invalid):

| Stage | Ready | Unverified | Invalid |
| --- | --- | --- | --- |
| First live run (old validators) | 56 | 28 | 2 |
| After revalidation with the fixed validators (no model) | 66 | 18 | 2 |
| After one retry of the 20 not ready | 75 | 11 | 0 |

Classification of the 30 outputs the old validators withheld (28 unverified, 2 invalid):

| Family | Outputs | Verdict |
| --- | --- | --- |
| Provision citation form (en dash, subsection suffix, ids the summary was never given) | 6 (bill summaries) | false positive, fixed |
| Citation number in text (`16 U.S.C. 796` read as numbers 16 and 796) | 1 | false positive, fixed |
| Amendment text paraphrased | 3 | false positive, fixed |
| Amendment text paraphrased through "this provision" (needs coreference) | 1 | false positive, not fixed on purpose |
| Amendment explanation too vague (names no removed text or subsection) | 5 | true rejection |
| Derived or reused number (a difference of two figures, a count not stated, a section number only in uncited lines) | 5 | true rejection |
| No anchor, statement text "placeholder" | 7 | true rejection |
| Empty statement (schema) | 2 | true rejection |

So 10 false positives were fixed, 1 remains, 19 were true rejections. Fixes: (1) provision ids compare in stored form (`usc/42/1395cc-7`); a printed citation resolves to a stored provision or to the stored section that contains the cited subsection, never to a broader or another provision; bill summaries list the provision ids of the citation set (ids only, no text) in the checked context, so their texts support no fact; (2) a US Code citation of a provision of the input is not read as numbers; (3) an amendment text counts as named when one sentence of the explanation keeps at least half of its meaningful words (at least 2). Numbers are not loosened: derived values stay rejected.

Deterministic planted-error catch rates before and after the fixes (same outputs, no grader): unchanged for every kind. Live set (56 ready outputs): changed-number 43/45, evaluative-wording 56/56, invented-number 56/56, nonexistent-anchor 56/56, reversed-effect 2/3, both before and after. On the 75 now ready: changed-number 61/63, evaluative-wording 75/75, invented-number 75/75, nonexistent-anchor 75/75, reversed-effect 2/3. Fakegen harness set: changed-number 74/74, evaluative-wording 86/86, invented-number 86/86, nonexistent-anchor 86/86, reversed-effect 10/16, before and after.

Grading of the new ready outputs raised the graded total to 225 statements: 88.4% supported, 4.4% (10/225) contradicted, all in change explanations that are ready and served. The share is a measured result for the operator; no threshold is applied here.

## Spend of the follow-up (upper bound, USD; list prices, batch 0.5x, cache writes 1.25x)

| Phase | USD |
| --- | --- |
| Retry generation (batch, 20 requests; the 9 stored results used 37,162 input and 6,800 output tokens, the rest estimated by ratio) | at most 0.40 |
| Grading of the 19 newly ready outputs (sync, estimated) | at most 0.50 |
| Total of the follow-up | at most 0.90 (cap 5.00) |

Earlier run total, from the first report: at most 7.5.
