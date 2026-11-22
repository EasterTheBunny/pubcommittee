# AGENTS.md — app/

Purpose: the SvelteKit web UI, deployed to a Node web host; an ordinary client of `read-api` and `write-api`.
Skills: `tool-sveltekit`, `tool-playwright`, `tool-docker`.
Owns: pages, components, the service worker, the manifest, UI tests. Does not own: the API contract (`api/openapi.yaml`; generated types here are regenerated, never edited), backend code, the archived M0 demo (`demo/`, from M1-13).

## Rules
- Data only through server-side `GET` to `read-api` with the generated types ([architecture § Web tier](../architecture/README.md#web-tier)). No backend credentials or cookies on the web host.
- Pages are complete without scripts ([10 § Machine-readable forms](../design/10-machine-access.md#machine-readable-forms)).
- The service worker never caches credentialed responses and never queues writes ([architecture § Progressive web app](../architecture/README.md#progressive-web-app)).
- Until M1-13 runs, `src/lib/mock/` holds the M0 demo data; M1 code must not import it.
- Features that need accounts stay placeholders through `src/lib/placeholders.ts`.

## Commands
- `docker exec -w /src/app <container> npm ci`
- `docker exec -w /src/app <container> npm run check`
- `docker exec -w /src/app <container> npm run lint`
- `docker exec -w /src/app <container> npm run build`

## Gotchas
- `npm run check` also type-checks `src/service-worker/` with its own `tsconfig.json`.
- Prettier runs the Svelte and Tailwind plugins; format only files you own.
