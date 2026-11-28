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

### Staging (operator, 2026-11-27, H.R. 3062, rfs, ready explanations)

31 of 38 explanations across four versions start at line 2 or 3 of s2 and end at 27 to 45. Nearly every explanation, in its `changes` statements too, cites the same shared lines (s2-l3, l5-l8, l22, l24, l28, l33, often l35) plus a few of its own (717b l20, l22, l35; 824a/f l23-l26; 796 l9-l11, l45; 60502 l21, l40; 3/301 l12-17, l40; 717a l42). Model anchors, with or without a role filter, therefore do not separate explanations, and a list of runs built from them would still stack every explanation over the shared lines. Line 3 is not the first line of the section there, so the lead-in rule does not fire.

### Fixtures (agent, local, 2026-11-27)

`fixtures/bills/` holds source XML only, so the citation lines come from running the extractor (`citations.Extract`, usc citations) over every fixture version; the provision id is `usc/<title>/<section>[/<path>]`. Per (section, provision) pair, the lines of the section that cite it:

| Measure | Value |
| --- | --- |
| (section, provision) pairs in the fixture versions | 125 |
| Pairs whose citation lines form one run | 121 |
| Pairs with more than one run | 4 (H.R. 3115 s2 18 U.S.C. 921(a): lines 2 and 193; s3 18 U.S.C. 922: 213 and 279; s3 20 U.S.C. 1070: 239 and 262; H.R. 3944 s2 10 U.S.C. 2883: 24 and 26) |
| Widest gap between runs | 190 lines (H.R. 3115) |
| Lines covered, citation lines alone | 133 |
| Lines covered, each extended to its block | 2757 (15 pairs widen; a cited lead-in takes its whole subsection, for example 2 to 212 and 279 to 2516) |

H.R. 3062 rfs `s2`: every provision cites a single, distinct line (717a 42; 717b 33; 717b(c) 22; 796 45; 824a(e) 24; 824a(f) 26; 824a-4(b) 27; 824o 44; 3/301 33; 42/4321 5; 42/7151 33; 60502 21). The only shared line is 33, cited by 717b, 3/301 and 42/7151, which is the line they all really cite.

### Decisions

- **Rule** (`Section.ExplanationSpans`, `internal/data/explain_span.go`): the lines of the section whose citation links name the provision, one run per cited line, touching runs merged. Fall back to the stored anchors (lead-in left out when other lines are named), then the whole section. `ExplanationSpan` is first-to-last of the runs.
- **Block extension: dropped.** It widens the fixture spans twentyfold (133 to 2757 lines) and swallows siblings and, for a cited lead-in, the whole subsection. It is not needed: a cited line is the line that says what the provision does.
- **One range or a list?** One range suffices for 121 of 125 fixture pairs and for every H.R. 3062 pair. Four pairs have more than one run (gaps 1, 22, 65, 190 lines). Their served range runs first-to-last, which is wider than the cited lines but never wider than today. A `line_spans` field and the UI to place several runs is a minor refinement, not needed for the acceptance criteria; it is the operator's call whether to schedule it as part 2.
- **Call sites.** `read-api` passes the citation links (`toExplanation`, `citationLinks`). The change-explanations export serves the stored explanations (`data.ExplanationsToShow`) with their stored `anchors` and no derived span, so it has no span to disagree with.

### Not measured, operator step

Re-list H.R. 3062 explanations from `dev` or `staging` `read-api` after deploying the rule, and record per-section overlap here. This needs the citation set of each version to be present (a version with no stored citation set falls back to the anchors, as before).

**Measured (`staging`, 2026-11-28, after the release with this rule and `job-ingest --analyze`):** `/v1/bills/us-119-hr-3062/v/eh/explanations` serves 10 explanations, all in section `s2`, each pinned to one line (before: all started at line 3 and ended between lines 33 and 45).

| Provision | Lines |
| --- | --- |
| 49 U.S.C. 60502 | 21 |
| 15 U.S.C. 717b(c) | 22 |
| 16 U.S.C. 824a(f) | 26 |
| 16 U.S.C. 824a-4(b) | 27 |
| 15 U.S.C. 717b, 3 U.S.C. 301, 42 U.S.C. 7151 | 33 |
| 15 U.S.C. 717a | 42 |
| 16 U.S.C. 824o | 44 |
| 16 U.S.C. 796 | 45 |

Overlap: only line 33, shared by three explanations whose provisions are cited on that same line. No two explanations overlap otherwise. `rfs` is now a collapsed referral of `eh` (`/v/rfs` answers 308 to `/v/eh`), so `eh` is the version served.

**Decided (operator, 2026-11-27):** part 2 (an additive `line_spans` field and UI placement by runs) is skipped. The four multi-run pairs in the fixtures are served first-to-last, never wider than before.
