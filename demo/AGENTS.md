# AGENTS.md — demo/

Purpose: the archived M0 proof of concept, a static SvelteKit site on mock data, kept buildable and served from its existing Netlify site.
Skills: `tool-sveltekit`, `tool-docker`.
Owns: the M0 code, mock data, and its own `package.json`. Does not own: the M1 app (`app/`), the API contract, backend code.

## Rules
- Frozen. Change it only to keep it building (dependency or tooling fixes) or to fix a broken link. No new features.
- It has no `read-api` client, no generated API types, and no shared code with `app/`. Do not import across the two.
- CI type-checks and builds it with its own dependencies (`.github/workflows/web.yml`).
- The operator points the Netlify site at `demo/` (build command `npm run build`, publish directory `build`).

## Commands
- `docker exec -w /src/demo <container> npm ci`
- `docker exec -w /src/demo <container> npm run check`
- `docker exec -w /src/demo <container> npm run build`

## Gotchas
- Mock data, personas, and demo state live in `src/lib/mock/` and `src/lib/state/`; they stay here and never move back into `app/`.
