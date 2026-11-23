# M1-13 — Web UI on Real Data

## Goal

Move the SvelteKit app from a static site on mocked data to server-rendered pages built from `read-api`. Readers browse and search the real catalog, read tracked bills with real text, explanations, citations, and version history, and share any line by a canonical address. Pages are complete without scripts. Features that need accounts stay placeholders naming M2. The M0 demo is archived to `demo/` and stays available on its existing Netlify site.

## Read first

- [architecture § Web tier](../../../architecture/README.md#web-tier)
- [01 § Two-layer view](../../01-bill-ingestion-and-display.md#two-layer-view), [§ Public catalog](../../01-bill-ingestion-and-display.md#public-catalog)
- [06 § Link behavior](../../06-sharing.md#link-behavior)
- [10 § Machine-readable forms](../../10-machine-access.md#machine-readable-forms) and [§ Instruction injection](../../10-machine-access.md#instruction-injection) (markup classes)
- [MILESTONES § Prior work, M1](../../MILESTONES.md#m1--real-bill-text-read-only-1) swap points: `types.ts`, `mock/bills.ts`, `anchors.ts`, `links.ts`, `diff.ts`
- `app/README.md`

## Scope

**In**
- **Archive the M0 demo first.** Move the M0 code out of `app/` to `demo/` before converting `app/`. Keep it buildable and available, and update every reference to it: `README.md`, `app/README.md`, [MILESTONES § M0](../../MILESTONES.md#m0--proof-of-concept), and [architecture](../../../architecture/README.md). The operator repoints the existing Netlify site at `demo/`, so the live demo URL keeps serving the M0 demo. (Done in M1-13: Netlify base `demo/`, build `npm run build`, publish `build`; the repoint itself is an operator step.)
- Switch from `adapter-static` to a Node-host adapter; keep prerendering for immutable version pages where useful.
- API client in `app/` using the generated TypeScript types (M1-02); `read-api` base URL from environment; server-side `GET` only.
- Replace domain types in `app/src/lib/types.ts` with generated types or thin wrappers named in design vocabulary.
- Pages:
  - home with the scope statement from the current policy;
  - catalog: filters, search, markup status, official links, selection reasons, coverage breakdown;
  - bill page: real text with sections, numbered lines, depth, blocks; bill summary; explanation panel with provenance labels (*auto-generated*, *bill-only*), cited provision text with release date, unresolved citations listed; version switcher; version change summary;
  - canonical path addresses for section and line range pages, beside in-page fragments; "changed since" notice on old-version addresses using M1-08 data;
  - selection policy page and public log page.
- `rel="alternate"` link on every page to its `read-api` resource, and content negotiation: a request for a page with `Accept: application/json` gets `303 See Other` to that resource ([10 § Machine-readable forms](../../10-machine-access.md#machine-readable-forms)); semantic markup with anchors on sections and lines; content class attributes on text.
- Mock data (bills, comments, suggestions, signals, briefs, personas) removed from the M1 app; it lives on in the archived demo. Entry points for account features become placeholders naming M2 or later through `placeholders.ts`.
- Line selection and copy-link sharing on real text (no social cards in M1).

**Out**
- Sign-in, comments, suggestions, signals, markup requests, briefs, inbox with real data: later milestones.
- Social preview cards and quote images: later.

## Deliverables

- Archived M0 demo at its new path, buildable and available
- Updated `app/` with server rendering from `read-api`
- Updated `app/README.md`
- UI tests covering catalog, bill page, line address, old-version notice, and no-script rendering

## Requirements

- **Pages render fully without scripts**; no content is reachable only by running scripts ([10](../../10-machine-access.md#machine-readable-forms)).
- The two layers stay visually distinct: explanation vs. literal text ([01 § Two-layer view](../../01-bill-ingestion-and-display.md#two-layer-view)).
- Every explanation shows its provenance; as-amended previews, if any, say *unofficial*.
- Untracked catalog entries link to the legislature's pages and never show bill text.
- The web host holds no backend credentials and receives no cookies.
- Code names follow design-doc vocabulary.
- Phone-width layout works; bills are long, so long-page performance is checked on a large real bill.

## Suggested breakdown

1. Archive the M0 demo to its new path; confirm it builds and is available; update references.
2. Adapter switch, environment config, API client and generated types.
3. Catalog page with filters, search, cursors, and official links.
4. Bill page rendering of real parsed text, depth, and blocks; performance on a long bill.
5. Explanation panel, citations, provisions, provenance labels; summaries.
6. Version switcher, version change summary, "changed since" notice.
7. Canonical path routes for sections and ranges; `anchors.ts` and `links.ts` updates; copy-link sharing.
8. Policy and log pages; scope statement; coverage breakdown.
9. Remove mock data from the M1 app; placeholder sweep for account features.
10. `rel="alternate"`, semantic markup, content-class attributes; no-script test.

## Acceptance criteria

- With JavaScript disabled, a reader can browse the catalog, open an exit bill, read every line, and follow a line-range address.
- A page requested with `Accept: application/json` answers `303` to its `read-api` resource.
- An old-version link to changed words shows the notice and a one-tap link to the current version.
- `npm run check`, lint, and Playwright UI tests ([architecture § Tooling](../../../architecture/README.md#tooling)) pass in CI against a `read-api` built with `memory` and fixtures.
- No mock data appears in the M1 app.
- The archived M0 demo builds from its new path and is available.
- All of the above run locally against a `read-api` on the `local` target.
- **Cloud phase:** the app is served from the web host for `dev` and `staging`.

## Open questions

- Archived demo in CI: **Decided:** yes; CI type-checks and builds `demo/` with its own dependencies.
- Which web host serves `dev` and `staging`? The host is interchangeable; record the choice in [architecture § Hosting](../../../architecture/README.md#hosting).
