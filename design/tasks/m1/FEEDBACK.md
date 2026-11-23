# M1 Feedback (local phase)

Operator feedback on the M1 local-phase implementation, from using the local run (`scripts/local.sh up`). Each item records what was seen, what causes it, a proposed change, and its owner. Decided changes move to the owning task or design doc; nothing here changes behaviour on its own.

Context for every item: the local run uses the fake generator (`adapters/fakegen`), not a model. Live-model outputs (the M1-09 L3 run, [test/accuracy/report.md](../../../test/accuracy/report.md)) differ where noted.

| # | Seen | Status |
| --- | --- | --- |
| 1 | "What changes" repeats the bill text; "Why it matters" is empty | Done: each part shows only when it has content |
| 2 | Sections whose text needs no explanation (the short title) still get an auto-generated block | Done: short-title-only sections are not explained |
| 3 | Most bill links answer 404 | Done: a tracked bill with no stored text has a page |
| 4 | Explanations sit in a side-bar, not pinned to the sections they describe; H.R. 3062's are hard to place; live data still explains the short title | Done: inline controls above the lines, with readable citations |
| 5 | A command to switch to the live-model data | Done: `scripts/local.sh data live` / `data fake` |

## 1. "What changes" repeats the bill text; "Why it matters" is empty

**Seen (2026-11-22).** Many explanations show "What changes" filled with a copy of the bill text beside it, and an empty "Why it matters". The copy duplicates the text on the page and clutters it.

**Cause.**
- The fake generator writes the `changes` statement as the cited lines verbatim and never writes a `why` statement (`internal/adapters/fakegen/fakegen.go`, `explanation`). Locally, every explanation looks like this.
- With a live model the `changes` statement is a paraphrase, and `why` is optional by prompt: change explanations included one in 49 of 66 live outputs; section explanations without a cited law (`bill-explanation`) included none of 17, because their prompt makes `why` optional and says "only from the text" (`internal/generate/prompts/bill-explanation.md`).
- The UI always renders both headings, even when a field is empty (`app/src/lib/components/ExplanationPanel.svelte`).

**Is "Why it matters" coming later?** Partly: a live run fills it for most explanations of amendments to existing law. For other sections it stays empty unless the prompt changes.

**Proposal.**
- Cite the block by line numbers only ("Lines 4-7") in place of repeating text that is already on the page; show "What changes" as prose only when it is not a copy of the cited lines.
- Show "Why it matters" only when it has content.
- Owners: M1-13 (UI), M1-09 (whether `bill-explanation` should ask for a `why`, and how the panel tells a copy from a paraphrase).

**Decided (operator, 2026-11-22).** With a live model "What changes" is a paraphrase, so keep it, and show "What changes" and "Why it matters" each only when it has content. Done in `app/src/lib/components/ExplanationPanel.svelte`. The fake generator's verbatim "What changes" is a local-run artefact and stays.

## 2. No explanation for sections that need none

**Seen (2026-11-22).** When the bill title does not change, there is nothing to explain, yet the section (for example "This Act may be cited as ...") carries an auto-generated block.

**Cause.** Planning creates a `bill-explanation` task for every section without a resolved citation, including the short title (`internal/generate/plan.go`).

**Proposal.** Skip the explanation for short-title sections, and for any section whose text is unchanged from the previous version when the explanation would only restate it; show no auto-generated block there. Owner: M1-09 (planning), with M1-08 (unchanged-section data from the stored comparison).

**Done (2026-11-22).** Planning no longer creates an explanation for a section whose every line only names the Act ("... may be cited as ..."); a short title that sits with a table of contents or other provisions is still explained (`shortTitleOnly` in `internal/generate/plan.go`, tested in `plan_shorttitle_test.go`). Explanations already stored stay until the data is rebuilt. Skipping unchanged sections across versions is not built. Since item 4, read-api and the change-explanations export also leave out short-title explanations stored earlier (`data.ExplanationsToShow`), so the live data no longer shows one.

## 3. Most bill links answer 404

**Seen (2026-11-22).** Most bill links show 404 (possibly expected).

**Checked.**
- The catalog's bill links point to Congress.gov, using the `legislationUrl` from each bill's official status record (for example `https://www.congress.gov/bill/119th-congress/house-bill/1025`); bills whose record has none get the same form derived from the bill id (M1-16 finding 1).
- Congress.gov answers `403` to every automated request, so their status could not be checked from the local run.
- The local seed stores text only for the three exit bills. The selection policy also tracks other catalog bills (for example `us-119-s-219`), but the fixtures hold only their status records, so they have no stored text; untracked catalog bills show no text by design (01 § Public catalog).

**Found (2026-11-22): `/bills/us-119-s-219`.** The selection policy tracks this bill, but the fixtures hold only its status record, so no text version is stored (the text pull is its own step). The app looked for a catalog entry only among untracked bills, found none, and answered 404, although `read-api` serves the bill.

**Done.** The bill page now falls back to the catalog entry whether or not the bill is tracked, and says "This bill is in review, but its official text has not been added here yet", with the official links (`app/src/lib/server/bill.ts`, `UntrackedBill.svelte`). `test/acceptance/M1-16/check.mjs` checks that such a bill answers 200 with that notice.

## 4. Explanations inline, pinned to the lines they describe

**Seen (2026-11-22).** Explanations sat in a side-bar, not pinned to the sections they describe; the H.R. 3062 change explanations were hard to place; the live data still explained the short title.

**Decided (operator).** Replace the side-bar with clickable text, styled like the line's `block` control, placed above the line it describes: "What changes", "Why it matters", "Existing law cited". Clicking one opens a block that pushes the text down; clicking it again closes it; clicking another switches the content and adjusts the block's style.

**Done (2026-11-23, `task/M1-13-inline-expl`).**
- `ExplanationBlock.svelte` replaces `ExplanationPanel.svelte`. Each part with content is a native `<details>` sharing a `name`, so one is open at a time, a second click closes it, and it works with scripts off and from the keyboard. Existing law shows as official quoted text; the other parts as platform text. "Auto-generated" and provenance stay visible with every part closed.
- Each explanation names what it explains: "Explains line 26, changes 16 U.S.C. 824a(f)". The API carries `line_from`, `line_to` and `provision_citation` (additive); explanations stored without anchors, such as the live set, are pinned on read from their citation lines.
- Checked on both local data sets with `scripts/local.sh test`: every check passes, and H.R. 3062's explanations each sit on their own line.

## 5. Switch to the live-model data

**Done (2026-11-22).** `scripts/local.sh data live` switches to a copy of the M1-09 live-model outputs (no key or model call); `data fake` switches back, and `data` shows the set in use. The set not in use is kept in `.local-data.<name>/` (git-ignored).
