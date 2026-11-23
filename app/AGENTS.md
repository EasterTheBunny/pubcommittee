# AGENTS.md — app/

Purpose: the SvelteKit web UI, deployed to a Node web host; an ordinary client of `read-api` and `write-api`.
Skills: `tool-sveltekit`, `tool-playwright`, `tool-docker`.
Owns: pages, components, the service worker, the manifest, UI tests (`tests/`, including `tests/stub-api.mjs`, an invented-data read-api stand-in for comparisons and a very long bill). Does not own: the API contract (`api/openapi.yaml`; generated types here are regenerated, never edited), backend code, the archived M0 demo (`demo/`, from M1-13).

## Rules

- Data only through server-side `GET` to `read-api` with the generated types ([architecture § Web tier](../architecture/README.md#web-tier)). No backend credentials or cookies on the web host.
- Pages are complete without scripts ([10 § Machine-readable forms](../design/10-machine-access.md#machine-readable-forms)).
- The service worker never caches credentialed responses and never queues writes ([architecture § Progressive web app](../architecture/README.md#progressive-web-app)).
- The M0 demo and its mock data live in `demo/`; nothing in `app/` imports from there. No mock data in `app/`.
- `READ_API_URL` (where the server reads) and `PUBLIC_API_URL` (what `rel="alternate"` and the 303 point at; defaults to the first) are read at runtime from `process.env` in `src/lib/server/api.ts`. The client there caches successful GETs for the API's `max-age` (capped at 5 minutes), because every reader shares the web host's one rate-limit allowance.
- Page paths mirror API paths; `src/lib/addresses.ts` maps one to the other for `rel="alternate"` and `hooks.server.ts` (303 on `Accept: application/json`). Add a route there when you add a page that has an API twin.
- Parts the API does not serve yet (summary, changes, compare, citations) render their designed empty or unavailable state; never invent data.
- Features that need accounts stay placeholders through `src/lib/placeholders.ts`.

## Commands

- `docker exec -w /src/app <container> npm ci`
- `docker exec -w /src/app <container> npm run check`
- `docker exec -w /src/app <container> npm run lint`
- `docker exec -w /src/app <container> npm run build` (Node server in `build/`; start with `READ_API_URL=<url> node build`)
- `BASE_URL=<app> STUB_URL=<app on stub> docker exec -e ... <playwright container> npx playwright test` from `/src/app` (see `tests/`, `tool-playwright`)

## Gotchas

- `npm run check` also type-checks `src/service-worker/` with its own `tsconfig.json`.
- Prettier runs the Svelte and Tailwind plugins; format only files you own.
- Playwright runs with `workers: 2`; more workers exceed a 1.5 GiB container. The fixture `read-api` needs `RATE_LIMITS` raised for a full test run (operator-set values, `api/MACHINE-API.md`).
- Svelte 5 `{#each}` over lines is keyed by anchor; keep per-line markup small (plain classes in `layout.css`), because bills are long.
- SvelteKit 3: `Handle` is imported from `@sveltejs/kit/hooks`; there is no `$env/dynamic/private`.
