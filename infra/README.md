# infra

Public, reusable OpenTofu modules and a placeholder example root. Environment roots, values, state, and the operator roster live in the private deployment repository, which pins modules by tag (`?ref=modules/vX.Y.Z`; `v0.1.0` predates the prefix). Module tags never start a deploy; `vX.Y.Z` tags are application releases. See architecture, "Public modules, private deployment".

- `modules/gcp/`: one module per concern. Every project, region, name, domain, and principal is an input; no real-value defaults; no sensitive outputs; scale to zero.
- `examples/`: wires every module with fake values in `fake.tfvars`. Validate and plan without credentials:

```sh
tofu init -backend=false && tofu validate
tofu plan -refresh=false -var-file=fake.tfvars
```

Never run `tofu apply` from this repository. The shared Artifact Registry and the per-environment deploy identities are created by the private bootstrap, not by these modules; modules take image references as input.

## Cloud runbook

Jobs in a deployed environment are run by hand: M1 has no schedules. The operator starts each one with `gcloud run jobs execute`. Each execution takes its own flags, and the `--args` override applies to that one execution only. Placeholders: `<project>` and `<region>` belong to the environment, and `<src-bucket>` and `<dst-bucket>` are the data buckets of two environments.

```sh
gcloud run jobs execute <job> --project <project> --region <region> --wait --args=<flag>,<value>,...
```

- `--task-timeout=4h` raises the task timeout for one execution. The deployed default (1 hour) is too short for a full US Code download.
- A job prints its report to stdout as one JSON line, so Cloud Logging stores it as one structured entry.
- Check a running job: `gcloud run jobs executions list --job <job> --project <project> --region <region> --limit 3`.
- A law download logs `law ingest started` (its `mode` and `source`), then one `law title stored` line per title (`method`, `done` of `total`, `elapsed_s`), so its logs show how far it is.
- Paid model runs need `--update-env-vars=PUBCOMMITTEE_LIVE_MODEL_RUNS=1` on that execution, after a dry run, a cost estimate, and the operator's approval. Without it, every model call is refused.

Flags for each job: [`cmd/job-ingest`](../cmd/job-ingest/README.md), [`cmd/job-summarize`](../cmd/job-summarize/README.md), [`cmd/job-exports`](../cmd/job-exports/README.md).

### First data in a new environment

Run these steps in order. Each one starts after the previous execution succeeded.

1. **Existing law.** In the first environment, download the newest US Code release:
   `job-ingest --law --titles all`, with `--task-timeout=4h`.
   In any other environment, copy that release instead of downloading it again:
   `scripts/law-copy.sh <src-bucket> <dst-bucket>` (operator login).
2. **Bills.** Pull and track each bill: `job-ingest --bill <id> --track`. A pull runs the citation and comparison steps when a release is stored.
3. **Catch up.** `job-ingest --analyze` runs citations and comparisons over every tracked bill. It is needed only when bills were pulled before step 1 finished.
4. **Plan.** `job-summarize --submit --dry-run --bill <id>` prints the planned tasks and input characters, and writes nothing. Estimate the cost from them.
5. **Generate**, after approval: `job-summarize --submit --generator claude --bill <id>`, with the live-run switch. Then `job-summarize --collect --generator claude` until no batch is waiting.
6. **Publish.** `job-exports` writes the exports and the change feed.

`job-summarize --submit` refuses a bill with no citation set, so steps 1 and 3 come first.

### Each new bill

Once a release is stored, a new bill needs steps 2, 4, 5 and 6 only. `--track` stores every text version and runs citations and comparisons. No separate `--analyze` is needed.

### New US Code release

1. **Check**, which downloads nothing: `job-ingest --law --dry-run`. A report with `"new": false` means the stored release is current; stop.
2. **Download**, in one environment: `job-ingest --law --titles all`, with `--task-timeout=4h`. Every release point is stored as a full copy under `law/usc/<release>/`, so every title is fetched again ([M1-07b](../design/tasks/m1/07b-incremental-law-release.md) makes this incremental).
3. **Copy** to every other environment: `scripts/law-copy.sh <src-bucket> <dst-bucket>`. The destination's `current.json` moves only when its own release is older.
4. **Re-analyze**, in each environment: `job-ingest --analyze` stores new citation sets for every tracked bill against the new release.
5. **Recheck**: `job-summarize --revalidate --bill <id>` for each summarized bill. It makes no model call. Generation keys do not include the release, so a dry run does not plan stored explanations again. Until [M1-09b](../design/tasks/m1/09b-law-aware-replanning.md), an explanation written against a changed provision stays as it is.
6. **Publish**: `job-exports`.

Never pass a subset of titles (`--titles 42`) for a new release: the run stores only those titles, then writes the release record and moves `current.json` to it, and every citation into another title stops resolving.

If an execution stops at the timeout, run it again. Stored sections are checked byte for byte and not written again. The release record and `current.json` are written only after the last title, so a stopped run leaves the previous release current.
