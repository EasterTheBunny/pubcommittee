# M2-05 — `write-api`: Sign-in, Sessions, and Write Barriers

**Phase:** local first (a local identity provider stand-in), cloud phase for Google and Facebook.

## Goal

A new service, `write-api`, signs testers in with Google or Facebook (tier 1), keeps server-side sessions, and refuses every write that does not come from the product's own pages, within per-account rate limits.

## Read first

- [architecture § Identity and personal data](../../../architecture/README.md#identity-and-personal-data) (sign-in, sessions, inventory, platform logs)
- [08 § Social sign-in](../../08-onboarding-and-verification.md#social-sign-in), [§ Tier 1 signals](../../08-onboarding-and-verification.md#tier-1-signals), [§ Eligibility](../../08-onboarding-and-verification.md#eligibility)
- [10 § Write barriers](../../10-machine-access.md#write-barriers)
- [architecture § HTTP services](../../../architecture/README.md#http-services-read-api-write-api-worker)

## Scope

**In**
- `cmd/write-api`: OAuth authorization code with PKCE, `state`, and `nonce` for Google (`openid` only) and Facebook (`public_profile`, `id` only); `HMAC(k_identity, provider ‖ subject)` to account; pseudonymous display name chosen at first sign-in; `us_at_signup` and day only.
- A local stand-in identity provider for `local` and CI (fake principals), so the whole flow runs without a provider account.
- Sessions in SQL (decided in [M2-00](00-decisions-and-prerequisites.md)): opaque id, `HttpOnly`, `Secure`, `SameSite=Lax`, host-only on `api.<domain>`; server-side expiry; sign-out deletes the record.
- Barriers: `Origin` must be `https://<domain>`; per-session anti-forgery token on every write; no bearer tokens, API keys, or CORS for writes; per-account rate limits keyed through `Keys`.
- Test-user gate: only invited test accounts can sign in during M2. Locally, `scripts/local.sh seed` creates them on the stand-in provider (a tester, a steward, a moderator).
- The routes for writes delegate to [M2-06](06-review-items.md), [M2-07](07-signals-and-markup-requests.md), [M2-09](09-notifications.md), [M2-10](10-moderation-and-steward-actions.md); this task owns the service, auth, and barriers.

**Out**
- Phone, address, or ID verification (M5). Passkey presence checks (later).

## Requirements

- **Local run:** adds what it builds to `compose.yaml` and `scripts/local.sh` under resource limits, so `scripts/local.sh up` shows it ([environments § Local run](environments.md#local-run)).
- No raw provider subject, email, name, or IP stored or logged (architecture § Personal data inventory, § Platform logs).
- `write-api` is the only service the browser sends credentials to.
- The allowed `Origin`, cookie host, and provider endpoints are configuration: on the local stack the app and `write-api` run on `localhost` ports (one site, so `SameSite=Lax` cookies work) with the stand-in provider; in the cloud phase they are `https://<domain>`, `api.<domain>`, Google, and Facebook.
- Every write and its refusal is countable in metrics without identity (architecture § Privacy rules for telemetry).

## Acceptance criteria

- Sign-in, session, and sign-out pass end to end on `local` with the stand-in provider; a test shows no provider subject or IP in SQL or logs.
- A write with a wrong `Origin`, a missing or stale anti-forgery token, or a bearer token is refused with a problem response.
- A burst over the per-account limit gets `429` with `Retry-After`.
- **Cloud phase:** Google and Facebook sign-in work in `dev` on `api.<domain>`; an uninvited account is refused.

## Open questions

- Real testers and their invitation method, for the cloud phase ([M2-00](00-decisions-and-prerequisites.md)).
