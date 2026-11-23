# job-ingest: manual bill pull

Pulls one or more bills on demand. M1 has no schedule and no feed: a person runs it.

```sh
# Offline, from the example data in fixtures/ (default source)
DATA_DIR=.local-data go run -tags local ./cmd/job-ingest --bill us-119-hr-4313 --track

# Live from govinfo (stage L2: one request at a time, 2 s apart; 1 status + 1 request per text version)
DATA_DIR=.local-data go run -tags local ./cmd/job-ingest --source govinfo --bill us-119-hr-4313 --track
```

| Flag | Meaning |
| --- | --- |
| `--bill <id>` | Bill id with jurisdiction and session, `us-119-hr-4313`. Repeat or comma-separate for several. |
| `--source fixtures\|govinfo` | Where status and text come from (env `INGEST_SOURCE`, default `fixtures`; `--fixtures-dir` / `FIXTURES_DIR`, default `fixtures`). |
| `--track` | Track the bill (reason `manual-pull`): fetch, store and parse every text version. A bill already tracked stays tracked. Without it, selection policy decides: a tracked bill's text is fetched, any other bill gets a catalog entry only. A bill of an excluded measure kind is never tracked, `--track` or not. |
| `--publish-policy v1\|<file>` | Store a selection policy version in the local data directory (`v1` is built in; a file is a JSON policy document) with its public log entry (`--reason`). Local target only; republishing identical content is a no-op, changed content for a stored version is refused. |
| `--evaluate` | Evaluate every stored catalog entry against the latest stored policy. Writes the entries that change (markup status, tracked, reasons, policy version) and the breakdown, and lists `newly_tracked` bills: pull their text with `--bill <id>`. Tracked bills are never untracked. With `--dry-run` it writes nothing. |
| `--law` | Existing-law ingest (M1-07, architecture "Ingest existing law"): store the newest US Code release point with its date; a release already stored is left alone and nothing is fetched. `--source fixtures` reads `fixtures/uscode/`, `--source govinfo` the Law Revision Counsel (same politeness rules). |
| `--titles <list>\|all` | With `--law`: the titles to store. Default: the titles the fixtures hold; for the live source a new release needs `--titles`. |
| `--dry-run` | Report what would change; write nothing, fetch no text. |
| `--verify` | Re-fetch stored originals and compare SHA-256; a mismatch is a failure and nothing is overwritten. |

Environment: `DATA_DIR` (local target). Output: one JSON report per bill on stdout (`stored`, `present`, `would_store`, `failures`); logs on stderr. Exit non-zero when any pull failed.

Stored, as in [architecture § Data recorded per ingest](../../architecture/README.md#data-recorded-per-ingest):
`bills/us/119/hr/4313/v/<code>/source.xml` (write-once, with `source.xml.meta.json` holding source URL, retrieval time, SHA-256), `.../text.json` (write-once, parser version), the bill document (tracked bills), the catalog entry, and failed items in `ingest_state` for the next pull to retry (`source` key `govinfo/119`).

A repeat pull fetches only versions not stored yet. Pulls of the same bill may overlap: stored versions are write-once and the other documents are rewritten with identical content.

Selection policy (M1-06): with a policy stored (`--publish-policy v1`), every pull evaluates it and sets the entry's `markup_status`, `reasons`, `policy_version` and `tracked_since`. With none stored, a pull warns and only `--track` tracks. Every pull run and every `--evaluate` run writes the tracked-set breakdown (counts by topic, by sponsor party, companions, administrator-selected) to `exports/tracked-set-breakdown.json`.

```sh
DATA_DIR=.local-data go run -tags local ./cmd/job-ingest --publish-policy v1
DATA_DIR=.local-data go run -tags local ./cmd/job-ingest --bill us-119-hr-84 --bill us-119-s-6
DATA_DIR=.local-data go run -tags local ./cmd/job-ingest --evaluate
```

In the fixtures source a bill the policy tracks from the catalog sample has no text to fetch; that pull reports a failure for each version and keeps the catalog entry.

Analysis steps (M1-07, M1-08): after the versions are stored, a pull of a tracked bill runs `citations` (every stored version against the current release point, freshness flags from stored enrolled texts of later public laws) and `comparisons` (every consecutive pair and every older version against the latest). Both cover all stored versions, so a repeat pull catches up a bill stored earlier; both are write-once. With no release point stored, citations are skipped with a note in the report (`notes`); run `--law` and pull again. Reports carry `citation_sets` and `comparisons`.

Extending the pull: `internal/ingest.DefaultSteps()` is an ordered list; insert steps with `Pipeline.After(name, step)` or swap one with `Replace`. M1-06 replaced `policy` (in `cmd/job-ingest/policy.go`); M1-07 and M1-08 insert after `after-store` and read `Run.Stored`.
