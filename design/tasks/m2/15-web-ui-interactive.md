# M2-15 — Interactive Web UI

**Phase:** local, then cloud phase in `dev` and `staging`.

## Goal

The M0 review experience on M1's real text: sign in, the side-by-side view, line comments, suggested edits, support and the bill-level signal, markup requests, the inbox, steward and moderator actions, and share links, all working without scripts where the design requires it.

## Read first

- `demo/` (the M0 prototype) and `app/` (M1)
- [03](../../03-review-features-and-support.md), [06](../../06-sharing.md), [08 § Installable app](../../08-onboarding-and-verification.md#installable-app), [§ Notifications](../../08-onboarding-and-verification.md#notifications)
- [architecture § Web tier](../../../architecture/README.md#web-tier) and [§ Progressive web app](../../../architecture/README.md#progressive-web-app)

## Scope

**In**
- Sign-in and sign-out through `write-api` redirects; per-bill pseudonyms shown on pages (decided in [M2-00](00-decisions-and-prerequisites.md)); public-content warning before the first write.
- Side-by-side view of a version and its review items; line comment and suggestion forms; diff view for suggestions; support and withdraw; bill-level signal; markup request with optional reason.
- My signals page (view and withdraw), inbox, device-notification opt-in.
- Steward and moderator controls for test roles.
- Share buttons and preview metadata from [M2-12](12-share-links-and-cards.md).
- Placeholders in `app/src/lib/placeholders.ts` replaced for M2 features; M3 placeholders kept.

**Out**
- Brief delivery, office responses, appeals (M3).

## Requirements

- **Local run:** adds what it builds to `compose.yaml` and `scripts/local.sh` under resource limits, so `scripts/local.sh up` shows it ([environments § Local run](environments.md#local-run)).
- Every write is a form that works without scripts (POST through `write-api` with the anti-forgery token); scripts only enhance.
- Credentialed pages never cache at the CDN; public review-item pages follow the TTL decided for review-item freshness (architecture § Open decisions).
- Counts shown with batch and "fewer than N" rules; never a live per-event count.

## Acceptance criteria

- Playwright on the local stack (`scripts/local.sh up`): a tester signs in (stand-in provider), comments, suggests, supports, withdraws, and sees the inbox, with scripts on and off.
- The page shows no count that changes between batches.
- **Cloud phase:** the same flow in `dev` with Google sign-in.

## Open questions

- Review-item freshness TTL ([architecture § Open decisions](../../../architecture/README.md#open-decisions)).
