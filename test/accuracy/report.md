# Accuracy report (M1-09)

Generated 2026-11-23 by `cmd/accuracy-report`.

- Generator: `claude-opus-5-5` (live run)
- Grader: `claude-sonnet-5-5` (must differ from the generator)
- Bills: us-119-hr-4313, us-119-hr-3062, us-119-hr-3427
- Prompt versions: bill-explanation=v1-f2a32231, bill-summary=v1-58de141b, change-explanation=v1-c8de06df, version-changes=v1-0adc248d
- Reproduce: `accuracy-report --generator claude --regrade-contradicted --report-only --agreement test/accuracy/operator-sample.json --out test/accuracy` (live, operator-approved, 2026-11-23); earlier: `accuracy-report --generator claude --grade` after `job-summarize --revalidate` and `job-summarize --submit --retry --generator claude`, and `accuracy-report --generator claude --grade-planted --crs --out test/accuracy`
- Counted outputs: those the current plan asks for. Short-title sections no longer get a bill explanation (design/tasks/m1/FEEDBACK.md item 2), so the 12 stored `s1` bill explanations drop out: bill-explanation shows 4 outputs and 16 statements (16 and 28 before), and the planted sets hold 63 outputs (75 before).

## Per task

| Task | Outputs | Ready | Unverified | Withheld | Refused/truncated/invalid | Supported | Contradicted | Fidelity fail | Amendment fail | Neutral-wording fail | Answered by fallback |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| bill-explanation | 4 | 4 | 0 | 0 | 0 | 100.0% (16/16) | 0.0% (0/16) | 0.0% (0/4) | 0.0% (0/4) | 0.0% (0/4) | 0 |
| bill-summary | 12 | 12 | 0 | 0 | 0 | 100.0% (92/92) | 0.0% (0/92) | 0.0% (0/12) | 0.0% (0/12) | 0.0% (0/12) | 0 |
| change-explanation | 54 | 45 | 9 | 0 | 0 | 90.7% (204/225) | 0.0% (0/225) | 1.9% (1/54) | 7.4% (4/54) | 0.0% (0/54) | 0 |
| version-changes | 4 | 2 | 2 | 0 | 0 | 100.0% (3/3) | 0.0% (0/3) | 50.0% (2/4) | 0.0% (0/4) | 0.0% (0/4) | 0 |

Validator failure rates are shares of outputs with at least one finding. Supported and contradicted shares are shares of graded statements, reported separately. Answered by fallback counts outputs a fallback model produced instead of the generator.

## Planted errors: catch rate per check

Known errors injected into good outputs. The deterministic columns are measured now, on the 63 ready outputs of the current plan. The grader column is from the earlier live run of 2026-11-22 (56 ready outputs, including short-title bill explanations) and was not re-run.

| Planted error | Planted | Meant for | Caught by that check | Caught by the grader |
| --- | --- | --- | --- | --- |
| changed-number | 63 | fact-fidelity | 96.8% (61/63) | 80.0% (36/45) (earlier run, 56 ready outputs) |
| swapped-section | 14 | grader only | n/a | 100.0% (8/8) (earlier run, 56 ready outputs) |
| reversed-effect | 3 | amendment-consistency | 66.7% (2/3) | 100.0% (3/3) (earlier run, 56 ready outputs) |
| evaluative-wording | 63 | neutral-wording | 100.0% (63/63) | 96.4% (54/56) (earlier run, 56 ready outputs) |
| invented-number | 63 | fact-fidelity | 100.0% (63/63) | 98.2% (55/56) (earlier run, 56 ready outputs) |
| nonexistent-anchor | 63 | anchors | 100.0% (63/63) | 26.8% (15/56) (earlier run, 56 ready outputs) |

## Operator sample and grader agreement

Sample unchanged (`test/accuracy/operator-sample.md` and `.json`; 30 statements, operator graded 2026-11-22). Grader-to-operator agreement on the 30 (hand-written): **30/30 (100%)**; by task: bill-explanation 9/9, bill-summary 9/9, change-explanation 9/9, version-changes 3/3. Computed by this run on the sampled statements still in the current plan: 100.0% (17/17); contradicted statements the grader missed: 0; false alarms: 0. The sample was drawn from served (ready) outputs only, so it holds none of the formerly contradicted statements and does not measure whether the grader catches errors; the planted-error grader column is the evidence for that.

### Second sample (hard sample, operator graded 2026-11-23; hand-written)

`test/accuracy/operator-sample-2.md` and `.json`: 30 statements, 10 per stratum: the 10 formerly grader-contradicted, 10 from validator-rejected outputs, and 10 grader-supported controls. The operator read 17 with the help of a model panel (`operator-sample-2.panel.md`: `claude-fable-5-2` and `claude-opus-5-5`, high effort, one statement per request with plain-language notes) and accepted as supported the 13 that the stored grade and both panel models all found supported, without reading them.

Operator labels: 20 supported, 10 unsupported, 0 contradicted.

| Stratum | Supported | Unsupported | Contradicted |
| --- | --- | --- | --- |
| Formerly grader-contradicted | 6 | 4 | 0 |
| Validator-rejected outputs | 6 | 4 | 0 |
| Grader-supported controls | 8 | 2 | 0 |

Agreement with the operator:

| Model | All labelled | Read by the operator |
| --- | --- | --- |
| `claude-sonnet-5-5` (stored grade, after the regrade) | 80% (16/20) | 64% (7/11) |
| `claude-fable-5-2` (panel) | 83% (25/30) | 71% (12/17) |
| `claude-opus-5-5` (panel; also the generator) | 87% (26/30) | 76% (13/17) |

The 10 statements of withheld outputs have no stored grade. The accepted 13 agree by construction, so the "read" column is the honest measure.

Findings:
- None of the 10 formerly contradicted statements is contradicted: the first grader's contradicted label was wrong in all 10 (6 supported, 4 unsupported). The fixed grader's labels for them agree with the operator in 8 of 10.
- The stored grader is lenient on omissions and scope: 3 statements it calls supported are unsupported for the operator (`09ad#5`, `2aaf#0`, `7af8#3`), and 1 it calls unsupported is supported (`37f2#3`). So a `supported` grade does not prove a statement is fully supported.
- The validators withheld outputs in which 6 of the 10 sampled statements are supported: the rejection is per output, so supported statements are withheld together with the failing ones.
- Against the whole bill text rather than the cited lines, 3 of the 10 unsupported statements are fully correct and 3 more are correct apart from a loose word ("new" for the exceptions) or a broad effective date: 77–87% accurate, against 67% by cited lines. The accuracy note shown with generated text says 75–85% (operator decision, 2026-11-23).
- `d8d6#2` (H.R. 4313, 42 U.S.C. 1395iii(b)(1)) explains an amendment that strikes "$1,403,000,000", but the stored provision says $2,041,000,000: the struck text is not in the current law, and the explanation presents the change as lowering the amount anyway. No check flags an amendment whose struck text is missing from the law.

## CRS summary coverage (bill summaries)

| Version | CRS points | Covered | Contradicted |
| --- | --- | --- | --- |
| us/119/hr/3062/ih | 7 | 100.0% (7/7) | 0 |
| us/119/hr/3427/ih | 6 | 100.0% (6/6) | 0 |

(CRS comparison from the earlier run; not re-run.)

## Proposed M3 target

Proposed shape, from the live numbers, for operator review: zero published statements graded contradicted; contradicted share of graded statements at or below 1% for every task (live after the regrade: 0.0% for every task, 0/336); nonexistent-anchor catch rate 100% by the deterministic check (live: 100%); the grader is not an anchor check (26.8%); grader-to-operator agreement on a hard sample at or above 90% (second sample: 80%, 16/20, all disagreements on omissions or scope). For operator decision.

## Grader fix and regrade of the contradicted pool (hand-written, 2026-11-23)

The 10 change-explanation statements graded contradicted (4.4%, 10/225) were reviewed by hand: 6 had reasons that found the statement consistent with its lines, 3 were omissions or qualifiers (unsupported, not contradicted), and 1 was a real error (`2675#4`, a scope the lines do not give).

Fixes: the grade prompt states the label rules (a missing claim or an added qualifier is `unsupported`; `contradicted` needs a conflicting fact); a contradicted verdict must quote, in `conflict`, words that appear in the statement's cited text, and its reason must not concede a match, or the grade is refused (`ParseGrade`); the request asks for one verdict per statement, by index; each cited provision is sent once; and when a reply still has the wrong verdict count, each statement is graded in a request of its own.

The 8 outputs holding the 10 statements were regraded (`--regrade-contradicted`, four paid runs). One output (`usc/16/796`, 4 statements) returned one verdict for its 4 statements three times and was graded statement by statement in the last run.

| Statement | Before | After |
| --- | --- | --- |
| `09ad#5`, `2675#2`, `c315#4`, `0b49#2`, `2496#4` | contradicted | supported |
| `2675#4` | contradicted | supported (likely a grader miss: should be unsupported for scope) |
| `6e01#3`, `37f2#2`, `37f2#3`, `8a70#2` | contradicted | unsupported |
| `c315#5` (not in the pool) | unsupported | supported |
| `37f2#1` (not in the pool) | supported | unsupported |
| `8a70#3` (not in the pool) | supported | unsupported (the cited text of 16 U.S.C. 796 is truncated and lacks the definitions it names) |

All stored grades: 348 statements, 327 supported, 21 unsupported, 0 contradicted (336 counted above for the current plan). Grade prompt versions are mixed: the 8 regraded outputs used three successive versions of the fixed prompt; the rest keep the original version.

Open: in the per-statement grade of `8a70`, two reasons hold stray markup (`</br>` and a trailing `"}]}`), a sign the grader wrote JSON inside a reason string; that may be why it answered one verdict for four statements.

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
