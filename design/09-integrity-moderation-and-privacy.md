# 09 — Integrity, Moderation, and Privacy

## Purpose

The signal is only worth sending if legislators believe it. Every supporter must be a real, unique resident, every moderation action must be visible, and individual positions must stay private.

## Scope

**In scope**
- Threat model and defenses
- Campaign and spike disclosure
- Moderation rules, actions, log, and appeals
- Privacy boundaries
- Transparency: open source, public logs, independent audit

**Out of scope**
- Verification mechanics (see [08](08-onboarding-and-verification.md))
- Steward safeguards (see [05](05-roles-and-stewardship.md))

## Threat model

| Threat | Defense |
| --- | --- |
| Fake or duplicate accounts | Tiered verification; only address-confirmed support counts in district totals; ID checks keep a one-way token, not the document |
| Out-of-jurisdiction brigading | Only eligible residents can signal; district from confirmed address |
| Burying a suggestion | Impossible by design: no opposing signal on suggestions |
| Organized campaigns inflating one item | Allowed but **disclosed**: spikes and shared-link referral surges flagged in the brief, never hidden or deleted |
| Pressure to support | Who supported what is private from the public and legislators; signals withdrawable while in review |
| Bad-faith suggestions | Abuse removable by moderators; substance never removed, only labelled |
| Platform operator bias | Open-source code, published moderation log, independent audit of counts each session |
| Low participation skew | Counts shown as share of reviewers, plus demographic coverage where available |
| Partisan stewards | See [05](05-roles-and-stewardship.md) safeguards |
| Automated submissions (bots, agents driving a person's browser) | No programmatic write path; presence checks and rate limits; suspected automation disclosed in the brief ([10](10-machine-access.md)) |
| Inferring individual signals from public data | Batched counts, small-count suppression, complementary suppression; see [Inference threats](#inference-threats) |
| Profiling and re-identifying reviewers | No cross-bill activity listings, coarse public timestamps, no district or tier on public content ([11](11-ai-security.md)) |

## Campaign and spike disclosure

- Detect unusual increases in support or opposition on an item or bill over a time window, relative to its baseline.
- Attribute surges to shared-link referrals where possible ([06](06-sharing.md)), in aggregate only.
- Disclose suspected automated signals the same way, as an automation event ([10](10-machine-access.md)).
- Disclose in the brief: the item, the window, the size of the surge, and the referral share. Never remove or discount the support.
- Disclosure must not reveal individual identities or the specific accounts that shared.
- Disclosure windows are reported coarsely, so a surge is not pinned to one public post's exact time.

## Moderation

### Rules
- Remove only content that breaks published rules: threats, harassment, personal data (doxxing), spam, automated submission ([10](10-machine-access.md)).
- Never remove or down-rank a suggestion or comment for its position.
- Rules are published as a public charter before launch.

### Actions
| Action | Who | Logged |
| --- | --- | --- |
| Flag content | Any signed-in user, stewards | Aggregate only |
| Remove content | Moderators | Yes, with reason |
| Suspend account | Moderators | Yes, with reason |
| Restore after appeal | Community panel | Yes, with outcome |

### Log and appeals
- Every removal and suspension is recorded in a **public moderation log** with a reason referencing a published rule.
- The log must not republish the removed personal data or abusive content itself.
- Every removal can be appealed to an **elected community panel**. Outcomes are public.

### What happens to removed content
- Removed comments show a placeholder with the rule cited.
- If a removed suggested edit had support, its supporters are notified. Support is not transferred.
- Removed content does not render in share cards ([06](06-sharing.md)).

## Privacy

| Data | Visibility |
| --- | --- |
| Comments and suggestions | Public, under a display name |
| Individual support/oppose | Private to the person. Never shown to the public, legislators, or stewards |
| Aggregate counts | Public; to legislators by jurisdiction (later, by district) |
| Address, precise location, phone | Private |
| ID documents | Not stored; one-way token only |
| Steward, moderator, office actions | Public and attributed |

- Small-population breakdowns (e.g. a district with very few reviewers) must not allow inference of individuals' positions. A minimum cell size for published figures is needed.
- **No published figure may change in response to a single person's action within a window short enough to attribute it.**
- Privacy is judged against every public output combined over time (pages, exports, feeds, briefs, logs), not each alone.
- No steward, moderator, or office tool shows individual signals or lists of supporters.
- Users can see and withdraw their own signals.
- Machine interfaces and bulk exports follow exactly these rules ([10](10-machine-access.md)).

## Inference threats

Individual signals can leak without any private data being read. Full catalogue in [11](11-ai-security.md).

| Attack | Defense |
| --- | --- |
| Count differencing: watching a count change right after a known person acts | Counts published in batches at a fixed interval, never per event |
| Small counts identifying the author or a known supporter | Counts below a threshold shown as "fewer than N"; authors do not auto-support |
| Intersecting breakdowns: national minus other districts, tier minus tier-in-district | Minimum cell size plus complementary suppression, applied consistently across releases |
| Withdrawals, merges, resets revealing who acted | Same batching; lineage merges applied at a batch boundary |
| Linking audit records to public comments by time | See audit rules below |

## Transparency and audit

- Code is open source.
- Moderation log is published.
- Support counts are independently audited each legislative session. The design must make counts reproducible for an auditor **without** exposing who supported what.
- Any per-signal audit record uses a per-bill token that cannot be linked across bills, with coarse timestamps, under a written agreement with the auditor. A method that verifies totals without per-signal records is preferred.

## Domain concepts

- **Flag** — reporter, target, rule claimed, timestamp.
- **Moderation action** — moderator, target, rule, reason, timestamp, appeal link. Public (sanitized).
- **Appeal** — requester, action, panel decision, reasoning. Outcome and reasoning public; requester private.
- **Integrity event** — item, window, metric, baseline, referral share; attached to briefs.
- **Audit export** — a privacy-preserving representation of signals sufficient to verify totals.

## Dependencies

- [03](03-review-features-and-support.md), [05](05-roles-and-stewardship.md), [06](06-sharing.md), [07](07-legislator-brief-and-responses.md), [08](08-onboarding-and-verification.md), [10](10-machine-access.md), [11](11-ai-security.md).

## Open questions

- What auditing method can verify counts while keeping individual signals private, and who is the independent auditor?
- What thresholds define a "spike" worth disclosing?
- Add calibrated noise (differential privacy) to published counts, or rely on exact counts with batching and suppression?
- How is demographic coverage measured without collecting sensitive data most users will not give?
- How is the community panel elected, sized, and protected from capture?
- Retention: how long are withdrawn signals, suspended accounts, and verification records kept?
