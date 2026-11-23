# AGENTS.md — internal/generate/

Purpose: task building, prompts, output validators and the two-phase submit/collect logic of `job-summarize`.
Skills: `tool-go`, `tool-golangci-lint`, `tool-docker`.
Owns: prompts and schemas (`prompts/`, `schemas/`, embedded), validators, planted-error injection, grader harness. Does not own: model adapters (`internal/adapters/claude`, `fakegen`), ports (`internal/data`).

## Rules
- Prompts are built from official bill text, cited provisions and platform metadata only; never user content, never CRS summaries ([architecture § AI-generated content](../../architecture/README.md#ai-generated-content), rules 1 and 2).
- A prompt or schema edit changes `PromptVersion` (a content hash), so it creates new idempotency keys; never edit an output in place.
- Validators are deterministic and never call a model. A failing output is stored `unverified`, never dropped and never published.
- No network, no SDK import here: everything runs on `fakegen` in tests.

## Commands
- `docker exec pc-<id>-dev go test -tags memory ./internal/generate/...`
- `docker exec pc-<id>-dev go tool golangci-lint run --build-tags memory ./internal/generate/...`

## Gotchas
- Anchors are `<section path joined by .>-l<n>` or `-l<from>-<to>`; a change summary may also cite `prev:`-prefixed anchors of the earlier version.
- A change to a validator needs `job-summarize --revalidate` on the stored outputs and the planted-error rates compared before and after (`accuracy-report`); a catch rate must not drop. Provision ids are compared in stored form (`usc/42/1395cc-7`); a printed citation (`42 U.S.C. 1395cc–7(a)(1)`) resolves to it.
