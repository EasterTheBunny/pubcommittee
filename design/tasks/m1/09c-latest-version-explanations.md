# M1-09c — Explanations on the Latest Version Only

M1 follow-up (operator, 2026-11-25), to lower generation cost. Related: [M1-05b](05b-version-ordering-and-referral.md), [M1-09](09-ai-generation.md), [M1-09b](09b-law-aware-replanning.md), [MILESTONES § M4 known issues](../../MILESTONES.md#known-issues).

## Goal

Inline explanations are generated only for a bill's latest version. A version that is already superseded when it is planned is handled as today, with its bill summary and its version change summary; only its inline explanations are missing. A bill whose four versions are pulled together pays for one set of inline explanations, not four. An operator can still ask for the inline explanations of one older version.

## Seen

- `PlanAll` (`internal/generate/run.go`) walks every stored version, and `PlanVersion` (`internal/generate/plan.go`) plans every task kind for each one: `bill-summary`, `version-changes` (from the second version on), and the inline explanations `change-explanation` and `bill-explanation`. Each key names its version (`GenKey.Version`), so each version's explanations are paid for separately.
- Inline explanations are the larger share of a bill's tasks. Each section that cites provisions gets one `change-explanation` per resolved provision, and each one carries the cited provision texts.
- A reader of a superseded version mostly needs to know what that version said and how the next one changed it. The bill summary and the version change summary cover that.

## Read first

- [M1-09](09-ai-generation.md) (task kinds, planning, write-once outputs, `job-summarize`)
- [M1-05b](05b-version-ordering-and-referral.md) (which version is the latest, collapsed referrals, code addresses)
- [M1-09b](09b-law-aware-replanning.md) (re-planning when cited law changes)
- [M1-10](10-read-api.md), [M1-11](11-exports-feed-machine-notice.md), and [M1-13](13-web-ui-real-data.md) (explanations route, change-explanations export, inline placement)
- [01 § Rules and invariants](../../01-bill-ingestion-and-display.md#rules-and-invariants) (accuracy note, unknown better than wrong)

## Scope

**In**
- **Planning.** `PlanAll` plans `change-explanation` and `bill-explanation` tasks only for the latest version in the version list ([M1-05b](05b-version-ordering-and-referral.md): stage order, collapsed referrals left out). `bill-summary` and `version-changes` are planned for every version, as today.
- **Reports.** `job-summarize --dry-run` and `--submit` report, per version, the explanation tasks left out because the version is superseded, so the cost estimate shows the saving.
- **Stored outputs stay.** Explanations already stored for a version that is now superseded stay stored and keep being served for that version (write-once). Only tasks not yet stored are left out.
- **New version arrives.** When a new latest version is pulled, the next `job-summarize` plans its explanations. The version it supersedes keeps what it already has and gets nothing new.
- **Read side and UI unchanged.** A superseded version is served and shown as today: bill summary, version change summary, accuracy note. Its explanations answer and its change-explanations export are empty, as for any version with no explanations. No new field and no new note (operator, 2026-11-25).
- **Older version on request.** `job-summarize --submit --bill <bill> --explain-version <code>` plans the inline explanations of that one version, superseded or not, on top of the normal plan. It goes through the usual live-run rules: `--dry-run` first, a cost estimate approved by the operator, and `PUBCOMMITTEE_LIVE_MODEL_RUNS=1` with `--generator claude` (operator, 2026-11-25). A code that is not in the bill's version list is refused. In the cloud phase, running it through `pubops` needs a new tool mode, which is a change to the `pubops` rule in [AGENTS.md](../../../AGENTS.md) and is proposed to the operator, not built here.
- **Re-planning.** [M1-09b](09b-law-aware-replanning.md) re-plans explanations for the latest version only. That task file gets a line saying so.

**Out**
- Carrying explanations to a version with unchanged text ([MILESTONES § M4 known issues](../../MILESTONES.md#known-issues)).
- Prompt changes. Keys of the tasks still planned do not change.
- Deleting or hiding stored outputs.

## Deliverables

- Planning rule in `internal/generate`, with tests: a bill pulled with several versions at once, a bill whose new version arrives after the old one was generated, and a single-version bill
- Skip counts in the `job-summarize` reports and dry run
- `--explain-version` flag on `job-summarize`, with tests: a superseded version, the latest version (no extra tasks), an unknown code
- One line in [M1-09b](09b-law-aware-replanning.md); `architecture/README.md` updated where it describes what is generated per version

## Requirements

- No model call in tests or CI; local runs use `adapters/fakegen`.
- Stored outputs are write-once and stay unchanged.
- Depends on [M1-05b](05b-version-ordering-and-referral.md): a wrong latest version would leave the real latest unexplained.

## Suggested breakdown

1. Planning rule and tests.
2. Report and dry-run counts.
3. `--explain-version` flag and tests.
4. Docs and the M1-09b line.

## Acceptance criteria

- For H.R. 4313 (after M1-05b: `ih`, `rh`, `eh`) with no stored outputs, `job-summarize --submit --dry-run` plans `bill-summary` for all three versions, `version-changes` for `rh` and `eh`, and inline explanations only for `eh`, and reports the explanation tasks left out for `ih` and `rh`.
- A version with explanations already stored keeps serving them after a newer version is pulled; the next run plans no new explanations for it.
- With `fakegen`, the bill page of `ih` shows its bill summary and accuracy note as today, with no inline explanations; `/v1/bills/us-119-hr-4313/v/ih/explanations` answers an empty list.
- `job-summarize --submit --dry-run --bill us-119-hr-4313 --explain-version ih` plans the inline explanations of `ih` and nothing else that is not already planned; `--explain-version zz` is refused.
- Cloud phase: the dry run in `dev` for H.R. 4313 shows the reduced task count before any paid run.

## Open questions

None. Decided (operator, 2026-11-25): the M1 exit criterion means the latest version and is reworded in [MILESTONES § M1](../../MILESTONES.md#m1--real-bill-text-read-only); an older version can be explained on request; an older version is shown as today, with no new note.
