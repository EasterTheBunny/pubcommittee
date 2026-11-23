# job-exports: bulk exports and change feed

Rebuilds every export from the stored data and appends new events to the change feed. M1 runs it on demand, after a pull and its steps; nothing chains it ([M1-11](../../design/tasks/m1/11-exports-feed-machine-notice.md)).

```sh
DATA_DIR=.local-data go run -tags local ./cmd/job-exports            # build and write
DATA_DIR=.local-data go run -tags local ./cmd/job-exports --dry-run  # build, report, write nothing
```

Output: one JSON report (`exports` built, `feed_new`, `skipped` with reasons); logs on stderr. Exit non-zero when the run failed.

What a run writes under `exports/`:

| Export id | Kind | Licence | Written |
| --- | --- | --- | --- |
| `bill-source.<bill>.<version>.xml` | `bill-text` | public-domain | write-once; the original document |
| `bill-text.<bill>.<version>.json` | `bill-text` | public-domain | write-once; parsed sections and lines |
| `existing-law.<bill>.<version>.json` | `existing-law` | public-domain | replaced whole; only when a citation set is stored under the current release |
| `change-explanations.<bill>.<version>.json` | `change-explanations` | CC0-1.0 | replaced whole (explanations are corrected); empty list when none |
| `catalog.json` | `other` | CC0-1.0 | replaced whole |
| `tracked-set.json` | `other` | CC0-1.0 | replaced whole; wraps `exports/tracked-set-breakdown.json` from M1-06 with the export header |
| `_index.json` | (the index; served at `/v1/exports`) | | replaced whole, last |

Every JSON file opens with `export`, `kind`, `as_of`, `licence`, `licences` (official public-domain, platform CC0-1.0), `content_class` and `notice`; each section, line, explanation and provision carries its own `content_class`. As-of times come from the stored records, never from the clock, so a rebuild from unchanged data is byte-identical and write-once files are no-ops. Old files are replaced, not kept.

Feed (`feed/entries.json`, served at `/v1/feed`): appended, never rewritten. An event is added once by id: `catalog-bill`, `bill-tracked`, `version`, `explanation` and `explanation-corrected` (one id per revision), `policy`, `log-entry`. `updated` is the UTC day the entry joined the feed. Titles are written from templates and never repeat source or administrator text. A first run over an existing data directory stamps every entry with that day.

Overlapping runs write the same bytes for exports; for the feed the later writer wins, and the next run adds whatever it missed.
