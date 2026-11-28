# M2-18 — M2 Acceptance

**Phase:** local gate (closes the local phase), then hosted gate in `staging` (closes the cloud phase).

## Goal

Every M2 exit criterion is proved, first on a local machine with the stand-in identity provider, then in `staging` with real sign-in for invited testers.

## Read first

- [MILESTONES § M2](../../MILESTONES.md#m2--interactive-prototype)
- M1-16 (`test/acceptance/M1-16/`, local and hosted gates) as the pattern

## Scope

**In**
- `test/acceptance/M2-18/`: checks against the `scripts/local.sh` stack (app, `read-api`, `write-api`, `admin-api`, Postgres, jobs) for the review cycle, write barriers, injection labelling, batch-window privacy, notifications, moderation log, exports and feed.
- `scripts/local.sh test` runs them against a running local stack.
- Hosted gate script against `staging` through the edge, honouring `Retry-After`.
- Evidence file mapping each exit criterion to its check, and the recorded M2 decisions.

## Acceptance criteria

- **Local gate:** every automated check passes on the local stack, and the operator has used every M2 feature on `scripts/local.sh up` with the seeded accounts. The cloud phase starts only after this.
- **Hosted gate:** the same checks pass in `staging`; the tester findings and charter from [M2-17](17-testers-and-charter.md) are linked.
- Batch interval and N recorded as decided.

## Open questions

- None of its own; it reports any criterion that cannot be proved.
