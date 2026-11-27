# M1-10b — Explanation Line Pinning

M1 follow-up (operator, 2026-11-25), from the cloud-phase check of H.R. 3062 in `dev` and `staging`. Related: [FEEDBACK.md item 6](FEEDBACK.md#6-explanations-of-one-section-overlap).

## Goal

Each auto-generated explanation is pinned to the lines it is about, so the explanations of one section no longer cover nearly the same long range. A reader can tell which lines each explanation describes, and two explanations overlap only where they describe the same lines.

## Seen

H.R. 3062 shows several explanations on the same version that all start at line 3 and end between lines 33 and 45. They cover nearly the same text.

## Cause

- Planning creates one `change-explanation` per (section, resolved provision) pair (`linked` in `internal/generate/plan.go`). A section that cites several provisions gets several explanations. This is intended.
- On publish, an explanation keeps the anchors cited by its `changes` statements (`changeAnchors` in `internal/generate/run.go`).
- `Section.ExplanationSpan` (`internal/data/explain.go`) pins the explanation from the lowest of those anchors to the highest. A model that cites the section's lead-in line (line 3) and a few lines further down produces a range such as 3–45, even when it rests on only a few of those lines.
- `read-api` serves the span as one `line_from`/`line_to` pair (`toExplanation` in `internal/readapi/derived.go`), and the UI places each explanation over that range (`app/src/lib/components/LineList.svelte`).

Item 4 of [FEEDBACK.md](FEEDBACK.md#4-explanations-inline-pinned-to-the-lines-they-describe) checked H.R. 3062 on the M1-09 live set, stored before anchors were kept, so each explanation was pinned to its citing line. Explanations generated since keep their anchors, and the lowest-to-highest range applies to them.

`ExplanationSpan` runs on the read side. A change to it applies to every stored explanation with no regeneration and no model call.

## Read first

- [FEEDBACK.md item 4](FEEDBACK.md#4-explanations-inline-pinned-to-the-lines-they-describe) (inline explanations, H.R. 3062)
- [M1-09](09-ai-generation.md) (tasks, anchors kept with an explanation)
- [M1-10](10-read-api.md) and [M1-13](13-web-ui-real-data.md) (explanations route, inline placement)
- [01 § Domain concepts](../../01-bill-ingestion-and-display.md#domain-concepts) (line, block, anchor)
- [architecture § API](../../../architecture/README.md) (contract conventions, additive changes)

## Scope

**In**
- Measure the spans first: for every stored explanation of the exit bills (fixtures under `fixtures/bills/`, and `read-api` answers in `dev`), list the anchors kept, the citing lines from the citation set, and the served span. Record how many explanations of one section overlap, and by how much, before and after.
- A pinning rule in `Section.ExplanationSpan` that uses the cited lines themselves, not the range between the lowest and the highest. Candidates, chosen from the measurement:
  - Leave out a lead-in line that every explanation of the section cites, when the explanation cites other lines.
  - Group the cited anchors into contiguous runs, extended to the block of each cited line ([01](../../01-bill-ingestion-and-display.md): a line and every line nested under it).
  - Fall back to the lines that cite the provision (citation links), then to the whole section, as today.
- If one range per explanation cannot describe scattered lines, an additive field in `api/openapi.yaml` that lists the ranges (for example `line_spans`), with `line_from`/`line_to` kept as the first-to-last range for existing clients.
- The UI places an explanation by its ranges, and the explanations of one section that still share lines read as one group, not as stacked repeats.
- The change-explanations export serves the same spans as `read-api` (`data.ExplanationsToShow` is already shared).

**Out**
- Planning and prompts: one explanation per (section, provision) stays. A prompt change creates new keys and needs a paid run.
- Carrying explanations from one version to the next ([MILESTONES § M4 known issues](../../MILESTONES.md#known-issues)).

## Deliverables

- Span measurement for the exit bills, recorded in this file under **Measured**
- Pinning rule in `internal/data/explain.go`, with table tests built from H.R. 3062
- If needed: an additive contract field, regenerated Go and TypeScript types, `read-api` mapping
- UI placement in `LineList.svelte` and `ExplanationBlock.svelte`, with a Playwright check on H.R. 3062
- `architecture/README.md` updated where it describes explanation pinning

## Requirements

- No regeneration and no model call. Stored outputs are write-once and stay unchanged.
- Additive contract changes only; existing fields keep their meaning ([10](../../10-machine-access.md)).
- Pages stay complete without scripts: the placement is server-rendered.
- Unknown is better than wrong: when the anchors do not name lines of the section, fall back as today.

## Suggested breakdown

1. Measurement script over the fixtures and the `dev` `read-api`; record the results.
2. Pinning rule and table tests.
3. Contract field, if step 1 shows one range is not enough.
4. UI placement and Playwright check.
5. Docs.

## Acceptance criteria

- On H.R. 3062 no two explanations of one section share more lines than the lines both actually cite; the served ranges match the recorded measurement.
- Every explanation of the exit bills still has a span inside its section.
- `read-api` and the change-explanations export serve the same spans.
- The bill page of H.R. 3062 renders without scripts, with every explanation placed over its own lines.

## Open questions

- One range or a list of ranges per explanation? Answered by the measurement in step 1; a list needs the contract field.

## Measured

Not yet measured.
