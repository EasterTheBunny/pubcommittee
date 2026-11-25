# M1-09b — Law-Aware Re-Planning of Explanations

After M1 exit. Depends on [M1-07b](07b-incremental-law-release.md) for the changed-section list; works without it, only slower.

## Goal

When a new US Code release changes a provision that an explanation was written against, `job-summarize` plans a new explanation for that section, and only for it. The new explanation is served as current; the old one stays readable from old links, marked with the release it was written against and the explanation that replaced it.

## Read first

- [M1-09](09-ai-generation.md) (tasks, keys, write-once outputs, revalidation)
- [M1-07](07-us-code-and-citations.md) (citation sets per version, parser, and release)
- [architecture § AI-generated content](../../../architecture/README.md) (output layout, `current-<target hash>.json` pointer, revised records)
- [infra/README.md § Cloud runbook](../../../infra/README.md#cloud-runbook) (new US Code release steps)

## Decisions (operator, 2026-11-25)

- **Method:** the generation key carries a hash of the law inputs, the provision texts the request holds. An unchanged provision gives the same key, so nothing is planned; a changed one gives a new key. Not the release id: that would re-plan every explanation on every release.
- **Old explanations stay visible on old links**, with their release date and a pointer to the explanation that replaced them. They are never deleted.
- **Two tasks**, M1-07b (ingest) and M1-09b (generation), both after M1 exit.

## Scope

**In**
- `GenKey` gains a law-inputs hash (empty for tasks with no provision text, such as the bill summary, so their keys do not change). Existing outputs keep their keys and stay valid.
- Planning: an explanation whose law-inputs hash has no stored output is planned; one with a stored output is not. A dry run lists the re-planned tasks with the provisions that changed.
- Serving: the `current-<target hash>.json` pointer moves to the new output once it is ready. The old output gets a superseded record (release written against, the replacing key id); `read-api` serves it when asked by its id and shows that note.
- Feed: one `explanation-corrected` entry per replaced explanation (existing event).
- With M1-07b's changed-section list: `--analyze` and planning skip bills that cite no changed section.
- Runbook update: a new release ends with a dry run, a cost estimate, and an approved submit of the re-planned tasks.

**Out**
- Re-planning on a prompt or model change (already a new key).
- Freshness flags from later laws within one release (M1-07 open question).

## Requirements

- No model call without the live-run switch and an approved estimate (M1-09).
- Outputs stay write-once; superseding adds a record, never rewrites one.
- Unknown is better than wrong: if a cited provision is missing from the new release (repealed, renumbered), the explanation is marked for review, not served as current.

## Suggested breakdown

1. Law-inputs hash in `GenKey` and `key.ID()`, with key-stability tests for tasks without provisions.
2. Planner change and dry-run report fields (`replanned`, changed provisions).
3. Superseded record, pointer move, and `read-api` route for an explanation by id with the note.
4. Feed entry.
5. Skip by changed-section list (after M1-07b).
6. Runbook and docs.

## Acceptance criteria

- A release that changes one cited provision re-plans exactly the explanations that carry it; a release that changes none plans nothing.
- After collect, the bill page shows the new explanation; the old one opens from its old link with its release date and the replacing link.
- Keys of bill summaries and version change summaries are unchanged by this task.

## Open questions

- Link form for an old explanation: by key id, or by version plus release. Answer here.
- What a reader sees on the bill page while the replacement is pending: the old explanation with a "law changed, update pending" note, or nothing. Operator.
