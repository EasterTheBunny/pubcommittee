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
- Checked 2026-11-28 (operator, `gh api`): `staging` and `prod` carry only a deployment branch policy, no required-reviewers rule, and administrators may bypass; the `v0.5.4` promote run recorded no approvals. The private repository's organization is on GitHub Team, and GitHub's documentation limits required reviewers on Free, Pro, and Team plans to public repositories. So no reviewer rule can be enforced there, and `v0.5.4` reached `prod` five minutes after `staging` with no stop.

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

- A signed `vX.Y.Z-rcN` on a `main` commit with a `dev` record deploys `staging` only, never `prod`.
- A signed `vX.Y.Z` deploys `staging`, then `prod`, only for a commit that `staging` already ran as a `vX.Y.Z-rcN`; on any other commit it is refused before any deploy.
- A tag that is lightweight, unsigned, or signed by a key outside the deployment repository's approved signers is refused before any deploy.
- Only the operator can create, move, or delete `v*` tags in this repository (tag ruleset).
- A rollback by hand still restores an earlier record without a rebuild (M1-17).

## Open questions

- Which gate: environment reviewers (needs a plan that enforces them on the private repository), or a manual `prod` promotion? Operator decision (spending).
  - **Decided (operator, 2026-11-28):** a manual `prod` promotion; no plan change.
  - **Changed (operator, 2026-11-28):** the operator's signed tag is the approval. Release candidates `vX.Y.Z-rcN` go to `staging`; a final `vX.Y.Z` goes to `staging` then `prod` only for a commit `staging` ran as its candidate. Tags are verified against an approved signer list in the deployment repository (GPG since 2026-11-28: the operator signs with GPG), and a tag ruleset here limits `v*` tags to the operator.
- Does `staging` keep an approval, or deploy automatically on a release tag?
  - **Decided (operator, 2026-11-28):** `staging` deploys automatically on a release tag.

## Proposed change (private repository)

A deployment-repository diff, applied by the operator (it replaces the earlier manual-`prod` draft): `promote.yml` accepts `vX.Y.Z` and `vX.Y.Z-rcN`, sends a candidate to `staging` only, refuses a candidate for `prod`, verifies every tag with `git verify-tag` in an empty keyring holding only `operators/signers.asc` (GPG; first drafted for SSH, changed 2026-11-28 because the operator's git signs with GPG), and refuses a final tag unless `staging`'s record for its commit names one of its candidates (or the release itself, on a re-run). The README and an `operators/signers.asc.example` say the same. The GPG check was tested in a container: a tag signed by the approved key passes; one signed by another GPG key, an SSH-signed tag, an unsigned annotated tag, and a lightweight tag are refused. The first candidate, `v0.6.0-rc1`, was refused by the SSH check because it carried a GPG signature.

In this repository: `dispatch-deploy.yml` also dispatches `vX.Y.Z-rcN` tags; a tag ruleset restricts creating, updating, and deleting `refs/tags/v*` to the repository admin role (created by the operator, 2026-11-28). Applied 2026-11-28. **Verified (2026-11-28):** the lightweight `v0.6.0-rc1` was refused; the signed `v0.6.0-rc4` deployed `staging` only; the signed `v0.6.0` on the same commit deployed `staging`, then `prod`.
