# PubCommittee — M0 proof of concept (archived)

This is the archived M0 demo. It lives in `demo/` and is served from its existing Netlify site, pointed at this directory (build command `npm run build`, publish directory `build`). The M1 app is in [`../app/`](../app/README.md).

A clickable demo of the full review cycle on mocked data. See [`design/MILESTONES.md`](../design/MILESTONES.md) for M0 scope and [`design/`](../design/) for the feature docs.

Stack: SvelteKit 3 (Svelte 5 runes), Tailwind CSS 4, `adapter-static`. No backend. Every page is prerendered, so the build is plain static files.

## Run

```sh
npm install
npm run dev          # http://localhost:5173
npm run check        # type check
npm run build        # static site in build/
npm run preview      # serve the build
```

Deploy `build/` to any static host. All routes are prerendered, so no SPA fallback is needed.

## What works in M0

- Bill page with numbered lines, the "bill against law" explanation panel, version switcher, rider tags and steward notes.
- Line selection (click a line number, shift-click to extend). The selection is written to the URL hash, so any line range is a shareable link, e.g. `/bills/hr-4127/v/2#s4-l12-13`.
- Old links: opening a v1 link whose words changed in v2 shows a notice and a link to the current version.
- Line comments, replies, and flags.
- Suggested edits with a word-level side-by-side diff, support, the brief minimum, a stale state after a new version, and a composer with live preview.
- Bill-level support and oppose with reason tags and rider section picks.
- Steward tools: mark well formed, plus the conflict rule (Steward A's member sponsors H.R. 4127).
- Issue page comparing competing bills, and a joined cross-chamber companion (S. 2210 v2 is H.R. 5530's text).
- Legislator brief built from live demo state; the office persona can respond Adopted / Under consideration / Declined.
- Persona switcher (header) and "Reset demo" (header icon or home page).

Anything not built in M0 is a dashed button that opens a pop-up naming its milestone and design doc. Social share targets and social sign-in work this way.

## Where things live

| Path                           | Contents                                                                                                                                 |
| ------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------- |
| `src/lib/types.ts`             | Domain types, named after the design-doc vocabulary                                                                                      |
| `src/lib/mock/`                | Mock bills, issues, personas and seeded activity. Edit here to change the demo                                                           |
| `src/lib/state/demo.svelte.ts` | Demo state (signals, supports, comments, responses), saved to `localStorage`                                                             |
| `src/lib/state/ui.svelte.ts`   | Global dialog state                                                                                                                      |
| `src/lib/placeholders.ts`      | Registry of future features shown in pop-ups                                                                                             |
| `src/lib/anchors.ts`           | Line anchors, URL hashes, stale detection                                                                                                |
| `src/lib/diff.ts`              | Word-level diff for suggested edits                                                                                                      |
| `src/lib/components/`          | UI components                                                                                                                            |
| `src/routes/`                  | Pages: `/`, `/bills/[bill]`, `/bills/[bill]/v/[version]`, `.../sec/[section]`, `/issues`, `/issues/[issue]`, `/briefs`, `/briefs/[bill]` |
| `src/routes/layout.css`        | Tailwind theme tokens and shared utilities (`btn-*`, `card`, `chip`, `field`)                                                            |

Notes:

- `#lib/...` imports need explicit extensions (`#lib/mock/index.ts`, `#lib/state/demo.svelte.ts`); SvelteKit 3 maps `#lib/*` straight to `src/lib/*`.
- Demo state is per browser. It is a convenience for demos, not storage.
- All names, offices, citations and numbers are mock data.
