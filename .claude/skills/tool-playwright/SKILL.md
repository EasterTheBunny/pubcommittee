---
name: tool-playwright
description: Writing and running Playwright tests for the PubCommittee UI inside a resource-limited container: no-script rendering, canonical line addresses, structured-form parity, offline and installability checks for the service worker. Load for UI tests (M1-13, M1-14, M1-16).
---

# Playwright tests

## Where and how

- Tests live in `app/tests/`; config in `app/playwright.config.ts`.
- Run in the official Playwright image matching the `@playwright/test` version in `app/package-lock.json` (`mcr.microsoft.com/playwright:v<version>-noble`), started by the task-orchestrator with limits like any task container (see `tool-docker`).
- Point tests at a server through `BASE_URL`; never hard-code hosts.

```sh
docker exec -w /src/app <container> npx playwright test
docker exec -w /src/app <container> npx playwright test tests/no-script.spec.ts --reporter=line
```

## Patterns

- **No scripts:** `test.use({ javaScriptEnabled: false })`; assert every line of a bill section is in the HTML and a line-range address resolves.
- **Structured-form parity:** fetch the page and its `rel="alternate"` resource; compare anchors and text.
- **Changed-since notice:** open an old-version line address whose words changed; assert the notice and the link to the current version.
- **Offline:** load a version page online, wait for the service worker (`await page.evaluate(() => navigator.serviceWorker.ready)`), `await context.setOffline(true)`, reload, assert full text; assert the out-of-date label on catalog pages.
- **Installability:** assert the manifest link, fetch and validate the manifest fields (name, icons, start URL, `display: standalone`), and that a service worker controls the page. Lighthouse has no PWA audit since version 12; do not add it.
- Use role and text locators; avoid CSS selectors tied to styling.
- Phone width: run key specs with a mobile viewport project.
