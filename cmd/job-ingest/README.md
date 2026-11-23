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
| `--track` | Track the bill (reason `manual-pull`): fetch, store and parse every text version. A bill already tracked stays tracked. Without it the pull writes the catalog entry only and never fetches text. Selection policy (M1-06) replaces this decision. |
| `--dry-run` | Report what would change; write nothing, fetch no text. |
| `--verify` | Re-fetch stored originals and compare SHA-256; a mismatch is a failure and nothing is overwritten. |

Environment: `DATA_DIR` (local target). Output: one JSON report per bill on stdout (`stored`, `present`, `would_store`, `failures`); logs on stderr. Exit non-zero when any pull failed.

Stored, as in [architecture § Data recorded per ingest](../../architecture/README.md#data-recorded-per-ingest):
`bills/us/119/hr/4313/v/<code>/source.xml` (write-once, with `source.xml.meta.json` holding source URL, retrieval time, SHA-256), `.../text.json` (write-once, parser version), the bill document (tracked bills), the catalog entry, and failed items in `ingest_state` for the next pull to retry (`source` key `govinfo/119`).

A repeat pull fetches only versions not stored yet. Pulls of the same bill may overlap: stored versions are write-once and the other documents are rewritten with identical content.

Extending the pull: `internal/ingest.DefaultSteps()` is an ordered list; insert steps with `Pipeline.After(name, step)` or swap one with `Replace`. M1-06 replaces `policy`; M1-07 and M1-08 insert after `after-store` and read `Run.Stored`.
