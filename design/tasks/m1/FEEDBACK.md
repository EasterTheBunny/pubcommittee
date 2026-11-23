# M1 Feedback (local phase)

Operator feedback on the M1 local-phase implementation, from using the local run (`scripts/local.sh up`). Each item records what was seen, what causes it, a proposed change, and its owner. Decided changes move to the owning task or design doc; nothing here changes behaviour on its own.

Context for every item: the local run uses the fake generator (`adapters/fakegen`), not a model. Live-model outputs (the M1-09 L3 run, [test/accuracy/report.md](../../../test/accuracy/report.md)) differ where noted.

| # | Seen | Status |
| --- | --- | --- |
| 1 | "What changes" repeats the bill text; "Why it matters" is empty | Open: proposal below |
| 2 | Sections whose text needs no explanation (the short title) still get an auto-generated block | Open: proposal below |
| 3 | Most bill links answer 404 | Open: needs the links clicked |

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

## 2. No explanation for sections that need none

**Seen (2026-11-22).** When the bill title does not change, there is nothing to explain, yet the section (for example "This Act may be cited as ...") carries an auto-generated block.

**Cause.** Planning creates a `bill-explanation` task for every section without a resolved citation, including the short title (`internal/generate/plan.go`).

**Proposal.** Skip the explanation for short-title sections, and for any section whose text is unchanged from the previous version when the explanation would only restate it; show no auto-generated block there. Owner: M1-09 (planning), with M1-08 (unchanged-section data from the stored comparison).

## 3. Most bill links answer 404

**Seen (2026-11-22).** Most bill links show 404 (possibly expected).

**Checked.**
- The catalog's bill links point to Congress.gov, using the `legislationUrl` from each bill's official status record (for example `https://www.congress.gov/bill/119th-congress/house-bill/1025`); bills whose record has none get the same form derived from the bill id (M1-16 finding 1).
- Congress.gov answers `403` to every automated request, so their status could not be checked from the local run.
- The local seed tracks only the three exit bills; every other catalog bill has a catalog entry and no text, so an in-app page for it shows no text by design (01 § Public catalog).

**Needed.** Which links were clicked (page and link text, or the URL), to tell a wrong link from an untracked bill or from Congress.gov itself. Owner: M1-13 (links), M1-05 (link source).
