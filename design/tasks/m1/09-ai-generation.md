# M1-09 — AI Generation

## Goal

Generate bill summaries, change explanations, and version change summaries for tracked bills with Claude, as batched single calls from a two-phase job. Outputs are platform content labelled *auto-generated*, every claim is anchored to bill lines and provisions, and no user content ever reaches a model.

## Read first

- [architecture § AI-generated content](../../../architecture/README.md#ai-generated-content) (tasks, rules, flow, model and platform, evaluation)
- [01 § Domain concepts](../../01-bill-ingestion-and-display.md#domain-concepts) (change explanation) and [§ Kinds of bill change](../../01-bill-ingestion-and-display.md#kinds-of-bill-change)
- [10 § Instruction injection](../../10-machine-access.md#instruction-injection)
- [11](../../11-ai-security.md)
- Claude API documentation for Message Batches, structured outputs, prompt caching, adaptive thinking and effort, refusals and `stop_reason`, and the Claude on Google Cloud client. Confirm model ids, prices, and feature availability at task start; do not rely on values in the architecture doc without checking.

## Scope

**In**
- `Generator` port (defined in M1-01) operations: submit batch, collect results, one synchronous call.
- `adapters/claude`: Anthropic Go SDK, Google Cloud client in `gcp` builds, first-party client elsewhere; structured outputs with a JSON schema per task; prompt caching of instructions plus the bill's full text; explicit `effort` per task; `stop_reason` checks; refusal fallback as in [architecture § Model and platform](../../../architecture/README.md#model-and-platform).
- `adapters/fakegen`: recorded fixtures, deterministic, no network.
- `cmd/job-summarize`: submit phase builds tasks from official text, cited provisions (M1-07), and version comparisons (M1-08); collect phase validates schema, anchors, and stop reason, stores outputs write-once at `generated/{task}/{bill}/v/{n}/{key}.json`, and updates `generations`.
- Tasks: bill summary; change explanation per (bill section, provision); bill-only explanation for freestanding text; version change summary.
- Idempotency key `(task, bill, version, target, prompt_version, model)`.
- Prompt files versioned in the repository.
- Telemetry: `gen_ai.client.token.usage`, `pubcommittee.generation.results`.
- **Accuracy checks** ([architecture § Accuracy checks](../../../architecture/README.md#accuracy-checks-before-stewards)):
  - fact fidelity and amendment consistency as deterministic validators in the collect phase;
  - the grounding grader, run on a different model from the generator, in local live runs only;
  - a planted-error suite for every check;
  - the CRS summary coverage check for bill summaries, using the `BILLSTATUS` summaries already in `fixtures/`;
  - `cmd/accuracy-report`, writing `test/accuracy/report.md`.

**Out**
- Competing-bill comparisons (M3).
- Steward review, the review queue, and the steward-written evaluation set: none exist until stewards do ([architecture § Evaluation](../../../architecture/README.md#evaluation)). M1 records `review_state: unreviewed` so the queue can be built later.
- Live model calls in CI ([architecture § Live model calls](../../../architecture/README.md#live-model-calls)).

## Deliverables

- `internal/adapters/claude`, `internal/adapters/fakegen`, `cmd/job-summarize`, prompt files, output schemas
- A CI switch for live model runs, off by default
- Validators, grader prompt and schema, planted-error suite, `cmd/accuracy-report`, and the first `test/accuracy/report.md` with a proposed M3 target

## Requirements

- **Official and platform content in, never user content** ([architecture § Rules](../../../architecture/README.md#rules)). Bill text is passed as delimited source material, not instructions.
- **Anchored claims:** each statement lists the line anchors and provision citations it rests on; outputs citing anchors not in the input are rejected.
- **Neutral language:** no ranking, recommending, or taking a position.
- Stored with `content_class: platform`, provenance `auto`, model id, prompt version.
- Refused, truncated, or invalid outputs are recorded with their outcome and never published.
- No personal data in any request; no secrets in the repository; the adapter authenticates with workload identity in `gcp` builds.
- **Live calls are local only.** Stage L3 uses an operator-supplied Anthropic API key with the first-party client, never committed. CI never calls a model: it uses `fakegen`, with live runs behind a switch that is off by default.
- Every output is stored with `review_state: unreviewed` and shown labelled *auto-generated*; nothing is labelled *steward-checked*.
- **Unknown is better than wrong.** An output failing fact fidelity or amendment consistency is recorded `unverified` and not published; one the grader marks *contradicted* is withheld.
- **CRS summaries never reach the generating model**; they are references only.
- The grader uses a different model from the generator, recorded with every grade.

## Suggested breakdown

1. Confirm platform, model id, batch and structured-output support on Claude on Google Cloud; record findings.
2. Output schemas per task, including anchor lists per statement.
3. Prompt design per task with cached prefix layout; prompt versioning.
4. `adapters/claude` and `adapters/fakegen`, conformance and unit tests.
5. `job-summarize` submit and collect phases, idempotency, `generations` records.
6. Validators: schema, anchor existence, stop reason, refusal handling.
7. CI wiring on `fakegen`, with the live-run switch off by default.
8. Deterministic validators (fact fidelity, amendment consistency) and their planted-error tests, in CI on `fakegen`.
9. Grader prompt and output schema; CRS coverage check; `cmd/accuracy-report`.
10. Local live run on the exit bills at stage L3, with the operator's key and approval: generate, grade, run planted errors through the grader.
11. Operator sample: prepare about 30 statements with their cited lines for the operator to grade (an operator step), then report agreement.

## Acceptance criteria

- With `fakegen`, `job-summarize` submit and collect runs end to end in CI; a re-run submits nothing new.
- After a local live run (stage L3), the three M1 exit bills have summaries and explanations for every resolved citation in the local store, each with anchors that exist.
- **Cloud phase:** the same in `dev`, through Claude on Google Cloud with the job's service account.
- An output citing a non-existent anchor is rejected in a test.
- CI makes no model calls with the live-run switch in its default state.
- In CI, the deterministic validators catch the planted errors meant for them, and an output with an invented number is recorded `unverified`.
- After the local live run, `test/accuracy/report.md` gives, per task, the supported and contradicted shares, validator failure rates, each check's catch rate, grader-to-operator agreement, CRS coverage for bill summaries, and a proposed M3 target.

## Open questions

- What starts `job-summarize` after a manual pull? **Decided for the local phase:** each step runs as its own command over stored data; nothing chains steps automatically ([README § Example data and local triggers](../README.md#example-data-and-local-triggers)). Automatic chaining is a cloud-phase question. Remaining for the cloud phase: The architecture starts it from `job-ingest` and runs it on a short interval until batches finish.
- Unreviewed output: **Decided:** in M1, every output that passes the accuracy checks is shown, labelled *auto-generated* ([architecture § Rules](../../../architecture/README.md#rules), rule 6). The bar for later milestones comes from the accuracy report.
- What effort applies per task in M1? Tune it from the accuracy report; any move to a cheaper model needs the same report to hold.
- Which model grades? It must differ from the generating model; confirm ids at M1 start.
- Are the model id, prices, Message Batches, and structured outputs on Claude on Google Cloud as the architecture states? Confirm at M1 start and record in [architecture § Model and platform](../../../architecture/README.md#model-and-platform).
