# M1-14 — Installable App on Real Data

## Goal

Keep the app installable on phones and desktops after the move to server rendering, with the app shell available offline and previously opened official text readable offline. Adapt the M0 service worker to real `read-api` resources and add CI checks for installability and offline reading.

## Read first

- [08 § Installable app](../../08-onboarding-and-verification.md#installable-app)
- [architecture § Progressive web app](../../../architecture/README.md#progressive-web-app)
- `app/src/service-worker/` and `app/static/` (M0 implementation)

## Scope

**In**
- Manifest and icons carried over and checked against current browser install requirements.
- Service worker strategies:
  - app shell cached for offline start;
  - official version pages and `read-api` immutable version resources cache-first;
  - catalog, explanations, summaries, policy, and log network-first with a short fallback copy, labelled as possibly out of date when offline;
  - never cache credentialed responses (none exist in M1; rule enforced now for M2).
- Cache versioning per deploy so a new release replaces old caches; size bound and eviction for cached bill text.
- Offline label component on pages served from fallback copies.
- The M0 local-notification demo stays only in the archived M0 demo ([M1-13](13-web-ui-real-data.md)); the M1 app drops it. Web Push is M2.
- CI checks: manifest and service worker valid (installability audit), and an offline load of a previously opened version page.

**Out**
- Web Push, inbox, subscriptions: M2.

## Deliverables

- Updated service worker and manifest in `app/`
- CI job running installability and offline tests

## Requirements

- Official text served from cache is always correct because versions are immutable; any non-immutable data shown offline is labelled.
- Never queue writes for later (none exist in M1; keep the rule in code).
- Cross-origin caching of `read-api` responses only for immutable resources; respect `Cache-Control`.

## Suggested breakdown

1. Audit M0 service worker against new routes and the API origin.
2. Strategy per resource class; cache names and versioning.
3. Offline label and fallback pages. (Done in M1-14: `OfflineLabel.svelte` and an offline page for never-opened URLs.)
4. Eviction policy for bill text.
5. CI installability audit and offline test.

## Acceptance criteria

- On `localhost` (a secure context), the app installs on a desktop browser, and the offline checks below pass.
- **Cloud phase:** the app installs on a current mobile browser and a desktop browser in `staging`.
- After opening an exit bill online, the same version page opens offline with full text.
- An offline catalog view shows the out-of-date label.
- CI fails if the manifest or service worker breaks installability (Playwright checks, [architecture § Tooling](../../../architecture/README.md#tooling)).

## Open questions

- Offline cache bound: **Decided:** the 50 most recently opened version pages and their resources; the least recently opened is evicted first. Recorded in [architecture § Progressive web app](../../../architecture/README.md#progressive-web-app).
