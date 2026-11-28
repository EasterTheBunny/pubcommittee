# M1-17b — Release Approval Gate

M1 follow-up (operator, 2026-11-27). A requirement of the [M3 closed pilot](../../MILESTONES.md#m3--closed-pilot) release, not an M1 exit criterion (operator, 2026-11-27). Related: [M1-17](17-continuous-delivery.md).

## Goal

A release tag never reaches `staging` or `prod` without the operator's approval for that environment. Today a pushed release tag appears to run `promote.yml` through to `prod` without waiting.

## Read first

- [M1-17](17-continuous-delivery.md) (promotion, approvals, rollback; criterion "A `prod` promotion waits for an approval in the `prod` environment", recorded as met on 2026-11-24)
- GitHub documentation for deployment environments: required reviewers, and which plans enforce them on private repositories

## Observed

- 2026-11-27 (operator): a release promotion appears not to wait for approval.
- The workflow wiring is as M1-17 describes: every `staging` and `prod` deploy runs in a job that names its GitHub environment, so the environment's protection rules apply if they exist.
- Not yet checked: the protection rules of the private repository's `staging` and `prod` environments. One possible cause: GitHub enforces required reviewers on private repositories only on some plans.

## Scope

**In**
- Find out why the gate does not hold: read each environment's protection rules and the run log of the last release.
- Restore a gate that holds before each of `staging` and `prod`. Either:
  - **Environment reviewers:** required reviewers on `staging` and `prod`, enforced on the private repository's plan; or
  - **Manual `prod`:** a release tag promotes to `staging` only, and `prod` runs only from a manual `promote.yml` dispatch, which becomes the approval. This works on any plan. `staging` still needs its own gate, or the operator accepts an automatic `staging`.
- Change to the private repository, proposed as a diff, if the second option is chosen.
- Update M1-17's runbook text and the deployment repository's README to match.

**Out**
- A second operator and "Prevent self-review" (M1-17 open question, decided for one operator).

## Acceptance criteria

- A signed release tag on a `main` commit with a `dev` record stops before `staging` until the operator approves, or starts no `staging` deploy, as chosen.
- `prod` changes only after an explicit operator action for that release.
- A rollback by hand still restores an earlier record without a rebuild (M1-17).

## Open questions

- Which gate: environment reviewers (needs a plan that enforces them on the private repository), or a manual `prod` promotion? Operator decision (spending).
- Does `staging` keep an approval, or deploy automatically on a release tag?
