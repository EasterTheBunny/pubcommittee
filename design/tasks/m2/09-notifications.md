# M2-09 — Notifications: Inbox and Device Notifications

**Phase:** local, then cloud phase in `dev`.

## Goal

Each tester has an in-app inbox of updates on the items they took part in, and can opt in to content-free device notifications sent in batches, with no supporter list stored anywhere.

## Read first

- [architecture § Notifications](../../../architecture/README.md#notifications) (finding whom to notify, Web Push, cost)
- [08 § Notifications](../../08-onboarding-and-verification.md#notifications)
- M1-11's change feed (the event source)

## Scope

**In**
- `GET /v1/me/notifications` on `write-api`: joins the person's `my_signals` and own items with feed events since `last_read_cursor`, in memory.
- `cmd/job-notify`: after each batch, per account with a device subscription, the same join; one content-free push per device; advance `last_event_cursor`.
- `adapters/webpush`: RFC 8291 encryption, VAPID signing; payload `{ "kind": "updates" }` only. Locally (stage L2) a development VAPID key pair is generated into `.local-data/` and never committed; the browser on `localhost` uses its vendor's push service.
- Subscribe and unsubscribe through the service worker and `write-api`; endpoint and keys stored encrypted; deleted on revoke, sign-out, or a gone report.
- New feed events M2 needs: revisions, stale flags, new versions, removals.

**Out**
- Email (deferred).

## Requirements

- **Local run:** adds what it builds to `compose.yaml` and `scripts/local.sh` under resource limits, so `scripts/local.sh up` shows it ([environments § Local run](environments.md#local-run)).
- Nothing joined is stored: no item-to-person index at rest (architecture § Notifications).
- Notifications carry no position detail (MILESTONES § M2).
- Only `job-notify` holds the VAPID private key; only `job-notify` and `write-api` decrypt `my_signals` and subscriptions.

## Acceptance criteria

- On `local`, a tester's inbox lists an event on an item they supported, and not on items they did not.
- A test push payload contains only `{ "kind": "updates" }`; a gone subscription is deleted.
- A database dump shows which accounts have devices, not which items they took part in.
- **Local (L2):** a browser on `localhost`, subscribed through the app, shows the notification after `scripts/local.sh job job-notify`.
- **Cloud phase:** a tester on an installed app in `dev` receives a device notification after a batch.

## Open questions

- None beyond [M2-00](00-decisions-and-prerequisites.md) (VAPID keys).
