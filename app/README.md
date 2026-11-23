# PubCommittee web app (M1)

SvelteKit 3 (Svelte 5 runes), Tailwind CSS 4, `adapter-node`. Pages are rendered on the server from `read-api` and are complete without scripts. The archived M0 proof of concept is in [`../demo/`](../demo/README.md).

## Run

```sh
npm install
READ_API_URL=http://localhost:8080 npm run dev     # http://localhost:5173
npm run check                                      # types (also the service worker)
npm run lint                                       # prettier --check
npm run build && READ_API_URL=http://localhost:8080 node build   # Node server on :3000
```

Start `read-api` first. For invented data: the memory target with `SEED_FIXTURES=1`, or `docker compose --profile seed run --rm seed` then `docker compose up read-api` for the `local` target.

| Variable         | Meaning                                                                                                   |
| ---------------- | --------------------------------------------------------------------------------------------------------- |
| `READ_API_URL`   | Where the server reads from. Default `http://localhost:8080`.                                             |
| `PUBLIC_API_URL` | Where `rel="alternate"` links and the `303` point, for browsers and machines. Defaults to `READ_API_URL`. |
| `PORT`, `HOST`   | The Node adapter's listener.                                                                              |

The web host holds no credentials, receives no cookies, and only sends `GET`.

## Pages

| Path                                                  | Shows                                                              | read-api resource                            |
| ----------------------------------------------------- | ------------------------------------------------------------------ | -------------------------------------------- |
| `/`                                                   | Scope statement from the selection policy, bills in review         | `/v1/bills`, `/v1/selection-policy`          |
| `/catalog`                                            | Filters, search, coverage, markup status, official links           | `/v1/bills?scope=catalog`                    |
| `/bills/{bill}`                                       | Latest official version, summary, explanations, citations          | `/v1/bills/{bill}` and `/v/latest`           |
| `/bills/{bill}/v/{n}`                                 | One version, version switcher, change summary                      | `/v1/bills/{bill}/v/{n}`                     |
| `/bills/{bill}/v/{n}/s/{section}` and `.../l/{range}` | Canonical section and line range addresses, "changed since" notice | `.../s/{section}`, `.../l/{range}`           |
| `/bills/{bill}/s/{section}` and `.../l/{range}`       | Version-less addresses (latest, with the pinned address)           | same, without `/v/{n}`                       |
| `/policy`, `/log`                                     | Selection policy, public log                                       | `/v1/selection-policy`, `/v1/moderation-log` |

Every page carries `<link rel="alternate" type="application/json">` to its resource. A request with `Accept: application/json` gets `303 See Other` to it (`src/hooks.server.ts`). Pages vary on `Accept`.

Untracked catalog entries link to the legislature's pages and never show text. Summary, change summary, comparison, and citations that `read-api` does not serve yet render their empty or unavailable state.

## Where things live

| Path                             | Contents                                                                                            |
| -------------------------------- | --------------------------------------------------------------------------------------------------- |
| `src/lib/api/schema.d.ts`        | Generated from `api/openapi.yaml` (`sh api/generate.sh`); never edited                              |
| `src/lib/types.ts`               | Thin design-vocabulary names over the generated types                                               |
| `src/lib/server/api.ts`          | The `read-api` client: server-side GET, in-memory cache                                             |
| `src/lib/server/bill.ts`         | Loaders for bill and address pages; the "changed since" decision                                    |
| `src/lib/addresses.ts`           | Page path to API path; Accept negotiation                                                           |
| `src/lib/anchors.ts`, `links.ts` | Fragment and path addresses, URL builders                                                           |
| `src/lib/selection.svelte.ts`    | Click and shift-click line selection (enhancement only)                                             |
| `src/lib/placeholders.ts`        | Features that need accounts, naming their milestone                                                 |
| `src/lib/components/`            | UI components                                                                                       |
| `tests/`                         | Playwright tests; `tests/stub-api.mjs` serves comparisons and a 10,000-line bill with invented data |

## UI tests

```sh
# app on the fixture read-api at BASE_URL; app on tests/stub-api.mjs at STUB_URL
BASE_URL=http://app:3000 STUB_URL=http://app:3001 npx playwright test
```

They cover the catalog, bill page, line addresses, the old-version notice, content negotiation, no-script rendering, phone width, and a long bill.
