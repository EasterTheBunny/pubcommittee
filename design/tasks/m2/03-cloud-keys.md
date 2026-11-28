# M2-03 — Cloud Key Adapter

**Phase:** cloud.

## Goal

`adapters/kmsgcp` implements the `Keys` port with Cloud KMS, so HMAC and encryption keys never leave the key service in deployed environments.

## Read first

- [architecture § Components](../../../architecture/README.md#components) (key management) and [§ SQL](../../../architecture/README.md#sql-private-small) (`k_signal`, `my_signals` key, subscription key)
- [M2-01](01-write-ports-and-domain.md) (`Keys` port)

## Scope

**In**
- HMAC (`k_identity`, `k_signal`, `k_rate`) and encrypt/decrypt (`my_signals`, push subscriptions) through KMS calls.
- Key rotation procedure from [M2-00](00-decisions-and-prerequisites.md), with a re-keying job if `k_signal` rotation needs one.
- The same conformance suite as `localkeys`, run against a real `dev` key (C1).

**Out**
- Creating keys and IAM bindings ([M2-16](16-infrastructure.md)).

## Requirements

- No key material in images, environment variables, or logs.
- Each unit holds only the key permissions it needs: `write-api` (identity, signal, my-signals, subscription), `job-notify` (my-signals decrypt, subscription decrypt), `job-batcher` (none beyond SQL reads).
- `read-api` never links this adapter.

## Acceptance criteria

- `kmsgcp` passes the `Keys` conformance suite against a `dev` key.
- The documented rotation procedure runs once in `dev` on test data and leaves every signal reachable.

## Open questions

- Rotation procedure ([M2-00](00-decisions-and-prerequisites.md)).
