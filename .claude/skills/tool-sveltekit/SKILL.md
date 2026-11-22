---
name: tool-sveltekit
description: Working in app/, the PubCommittee SvelteKit UI: server rendering from read-api only, pages complete without scripts, generated API types, design-doc vocabulary, placeholders, and the npm commands to check, lint, and build inside the task container. Load for any change under app/.
---

# SvelteKit app (`app/`)

Read `app/AGENTS.md` first.

## Rules

- **Data only from `read-api`**, through server-side `GET` in `+page.server.ts` / `+layout.server.ts`, using the generated types in `src/lib/api/`. No direct store access, no backend credentials, no cookies.
- **Pages render fully without scripts.** Text, anchors, and links are in the server HTML. Scripts may enhance; they never gate content.
- **Semantic markup with anchors:** sections, headings, and numbered lines carry their anchor ids; text carries its content class (`data-content-class="official|platform|user"`).
- **Two layers stay distinct:** explanation versus literal bill text; every explanation shows its provenance label.
- **Untracked bills** link to the legislature's pages and never show bill text.
- **Account features** (sign-in, comments, signals) are placeholders through `src/lib/placeholders.ts`, naming their milestone.
- Names follow the design-doc vocabulary (`Bill`, `OfficialVersion`, `Anchor`, `Provision`).
- Svelte 5 runes; Tailwind CSS; phone-width layout works.

## Commands (inside the container, working directory `/src/app`)

```sh
docker exec -w /src/app <container> npm ci
docker exec -w /src/app <container> npm run check     # svelte-check + service worker types
docker exec -w /src/app <container> npm run lint      # prettier --check
docker exec -w /src/app <container> npm run build
```

Do not run `npm run format` on files you do not own.
