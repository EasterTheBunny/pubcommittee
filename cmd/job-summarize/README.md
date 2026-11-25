# job-summarize

Generates bill summaries, change explanations and version change summaries
([M1-09](../../design/tasks/m1/09-ai-generation.md)). Two phases, never chained
by this job or by `job-ingest`:

```sh
# plan and submit one batch per bill (a re-run submits nothing new)
DATA_DIR=.local-data go run -tags local ./cmd/job-summarize --submit --bill us-119-hr-4313
# a later run validates, stores and publishes finished batches
DATA_DIR=.local-data go run -tags local ./cmd/job-summarize --collect
```

- `--revalidate --bill <id>` repeats the deterministic checks on the stored outputs with the current validators.
  It calls no model and regenerates nothing. An output whose status or findings change is stored as a revised
  record beside the write-once one (readers see the revised record); a newly ready output is published.
- `--submit --retry` also plans again the stored outputs that are not ready (rejected, refused, truncated or
  invalid). A later `--collect` stores a ready answer as the revised record of the same key, so the write-once
  key does not block a retry and no prompt edit is needed. It costs money with `--generator claude`.
- Default generator is `fakegen`: deterministic, no model, no network. This is what CI and the
  acceptance script use.
- `--generator claude` is a live model run (stage L3). It refuses to start unless
  `PUBCOMMITTEE_LIVE_MODEL_RUNS=1` is set and the operator's key is in `ANTHROPIC_API_KEY`. Do not
  use it without the operator's approval: it costs money.
- On the `gcp` build (Cloud Run job), `--generator claude` uses Claude Models on Google Cloud with the
  job's service account (no API key); the workspace is `ANTHROPIC_GOOGLE_CLOUD_WORKSPACE_ID`, the project
  `GCP_PROJECT`. The `gcp` build has no `fakegen`: without `--generator claude` and the live-run switch,
  every model call is refused, so only `--submit --dry-run` works. `--collect` of a live batch also needs
  both.
- `--dry-run` with `--submit` plans and prints counts and input sizes; it writes and submits nothing.
- Outputs are stored write-once at `generated/{task}/{bill}/v/{version}/{key}.json`; ready summaries
  are served by read-api (`/summary`, `/changes`), ready explanations by `/explanations`. Every
  output is `review_state: unreviewed`, provenance `auto`.
