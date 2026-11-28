# M2-01 — Write Ports and Domain Types

**Phase:** local.

## Goal

The domain types and ports M2 writes through exist in `internal/data`, with a memory adapter that passes conformance suites every later adapter must pass.

## Read first

- [architecture § Data access layer](../../../architecture/README.md#data-access-layer) (ports, interface rules, build-time wiring, conformance tests)
- [architecture § SQL (private, small)](../../../architecture/README.md#sql-private-small) and [§ Document store](../../../architecture/README.md#document-store-public-mutable)
- [03 § Domain concepts](../../03-review-features-and-support.md#domain-concepts), [05 § Domain concepts](../../05-roles-and-stewardship.md#domain-concepts), [09 § Domain concepts](../../09-integrity-moderation-and-privacy.md#domain-concepts)

## Scope

**In**
- Types: account, identity, session, review item (line comment, suggested edit), signal (support, bill-level signal, markup request, rider tag), request reason, flag, moderation action, role grant, push subscription, notify state, count snapshot.
- Ports, split by who may hold them: `Accounts`, `Sessions`, `ReviewItems` (public documents), `Signals` and `MySignals` (private SQL), `Moderation` writes, `Roles`, `PushSubscriptions`, `NotifyState`, `Snapshots` (read side joins the public read group, per architecture § Ports).
- A `Keys` port for HMAC and encryption calls, so no adapter or service holds raw keys; `localkeys` implements it locally. Every HMAC result carries its key version (decided in [M2-00](00-decisions-and-prerequisites.md)); M2 uses version 1.
- Memory adapter for every new port; conformance suites for each.
- Dependency rules: `read-api` links no private write port, no SQL, no key adapter (extend `test/ci/deps.sh`).

**Out**
- Postgres ([M2-02](02-postgres-adapter.md)), cloud keys ([M2-03](03-cloud-keys.md)), any HTTP route.

## Requirements

- `signal_key = HMAC(k_signal, account_id ‖ item_id)`; the signals store holds no account id (architecture § SQL).
- `my_signals` is encrypted with its own key (architecture § SQL).
- Typed errors as in M1 (`ErrNotFound`, `ErrConflict`, `ErrInvalid`); a second signal by the same person on the same item is `ErrConflict`.
- Names follow [design/README § Shared vocabulary](../../README.md#shared-vocabulary).

## Suggested breakdown

1. Types and errors. 2. Port interfaces with doc comments. 3. `Keys` port and `localkeys` methods. 4. Memory adapter per port. 5. Conformance suites. 6. Dependency rule updates.

## Acceptance criteria

- Memory adapter passes every new conformance suite; `golangci-lint` clean.
- A test shows the signals store cannot answer "which items did account X signal" without `MySignals` and the key.
- `go list -deps -tags gcp ./cmd/read-api` contains no write port, SQL, or key package.

## Open questions

- None. Key versioning and sessions in SQL are decided ([M2-00](00-decisions-and-prerequisites.md)).
