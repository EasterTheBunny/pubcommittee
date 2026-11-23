# AGENTS.md — internal/accuracy/

Purpose: measures the accuracy checks of M1-09 and renders `test/accuracy/report.md` (used by `cmd/accuracy-report`, a development tool, never deployed).
Skills: `tool-go`, `tool-golangci-lint`, `tool-docker`.
Owns: statistics, planted-error runs, operator sample, CRS coverage, report rendering. Does not own: validators and prompts (`internal/generate`), adapters.

## Rules
- A number that needs a live model run (grader shares, grader catch rate, grader-to-operator agreement, CRS coverage, live validator failure rates) is rendered `pending` unless the run was live; fakegen results are labelled harness-only, never quality.
- Reports carry the inputs needed to reproduce them: models, prompt versions, bill and version ids, counts.
- No network and no model call here: callers pass a `data.Generator`.

## Commands
- `docker exec pc-<id>-dev go test -tags memory ./internal/accuracy/...`
- `docker exec pc-<id>-dev go tool golangci-lint run --build-tags memory ./internal/accuracy/...`
