---
name: tool-golangci-lint
description: Running PubCommittee's strict golangci-lint v2 setup inside the task container, reading findings, fixing them properly, and the nolint policy. Load before committing or reporting any Go change.
---

# Strict golangci-lint

The repository's `.golangci.yml` (golangci-lint v2) enables every linter (`linters.default: all`), justifies each disabled one in the file, and sets no issue limits. Findings fail CI.

## Run

```sh
docker exec <container> go tool golangci-lint config verify
docker exec <container> go tool golangci-lint run ./internal/data/...      # the packages you touched
docker exec <container> go tool golangci-lint run --build-tags memory ./...
docker exec <container> go tool golangci-lint fmt --diff                   # formatters
```

- Lint with each build tag your change compiles under (`memory`, `local`, `gcp`).
- Before `.golangci.yml` exists, use `run --default all` and report the result.

## Fix, do not silence

| Finding | Fix |
| --- | --- |
| `errcheck`, `errorlint`, `wrapcheck` | Handle or return the error; wrap with `%w`; compare with `errors.Is`/`errors.As` |
| `err113` | Use the typed errors in `internal/data`, or a package-level sentinel |
| `contextcheck`, `noctx` | Pass the caller's `ctx`; use `http.NewRequestWithContext` |
| `gosec` | Fix the cause (file modes, path joins, bounded reads); never disable |
| `depguard` | The import is forbidden in this package (for example SQL or key adapters in `read-api`); move the code, do not change the rule |
| `revive`, `godot`, `godox` | Doc comments on exported names, full sentences; no `TODO` without an issue or task id |
| `funlen`, `gocognit`, `cyclop` | Split the function |
| `paralleltest`, `tparallel` | Add `t.Parallel()`, or say why not in a comment |
| `exhaustive` | Handle every enum case, or add a `default` that returns an error |

## `//nolint`

- Workers never add `//nolint` and never edit `.golangci.yml`; report the finding instead.
- Task-orchestrators may add one only in the form `//nolint:<linter> // <reason>`, and list it in the task report. `nolintlint` rejects any other form.
- Changing `.golangci.yml` is a proposal in the report, never part of a task commit.
