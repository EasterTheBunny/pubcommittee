# AGENTS.md — internal/selection/

Purpose: evaluate the published bill selection policy against a catalog entry, and publish policy versions.
Skills: `tool-go`, `tool-golangci-lint`, `tool-docker`.
Owns: policy validation, the pure evaluator, scope statement, tracked-set breakdown, `policies/`. Does not own: the pull pipeline (`internal/ingest`), storage, or the admin action that publishes (M1-12).

## Rules
- The evaluator reads process signals and the legislature's subject indexing only. `Input` has no sponsor, party, or position field; `TestInputHasNoSponsorFields` fails if one is added. Party is read only in `breakdown.go`, after selection ([01 § Transparency](../../design/01-bill-ingestion-and-display.md#transparency)).
- A tracked bill is never untracked: `Evaluate` with a `Prior` keeps its reasons and policy version.
- Referral alone is never committee action.
- A policy version is immutable. A new term list is a new version, published with `Publish` and a public reason.
- Topic term lists are drafts until the operator confirms them; `policies/README.md` names each term's source.

## Commands
- `docker exec pc-<id>-dev go test -tags memory ./internal/selection/... ./internal/ingest/...`
- `docker exec pc-<id>-dev go tool golangci-lint run ./internal/selection/...`

## Gotchas
- Reason strings are public data: `committee-action:<kind>`, `topic:<name>:<subject|policy-area|title-only>`, `companion:<bill id>`, `manual-pull`.
- Floor and calendar activity come from the status, the latest action, and the official text-version labels, because the catalog keeps only the latest action.
