# 05 — Roles and Stewardship

## Purpose

Define who can do what. Citizens review and support; every other role exists to keep suggestions legible and the signal trustworthy. No role can hide a suggestion because of its position.

## Scope

**In scope**
- Role model and permission matrix
- Steward activities, queues, and sign-off
- Safeguards on a partisan steward workforce
- Appeals of steward actions

**Out of scope**
- Identity verification tiers for reviewers (see [08](08-onboarding-and-verification.md))
- Moderation rules and the moderation log (see [09](09-integrity-moderation-and-privacy.md))
- Office responses to briefs (see [07](07-legislator-brief-and-responses.md))

## Roles

| Role | Who | Can do | Cannot do |
| --- | --- | --- | --- |
| Reader | Anyone, no account | Read bills, comments, suggestions, counts; share | Comment, suggest, signal |
| Reviewer | Eligible signed-in resident ([08](08-onboarding-and-verification.md)) | Comment on lines; write and support suggested edits; support or oppose a bill | Signal on bills outside their jurisdiction |
| Steward | Verified staff of sitting members of Congress at launch; vetted volunteers later | Mark well formed, offer wording fixes, combine duplicates, tag topics, check explanations, link lineage, group competing bills | Hide a suggestion for its politics; add weight to support |
| Moderator | Platform staff plus elected community moderators | Remove abuse, spam, doxxing; suspend accounts | Change the substance of a suggestion |
| Legislator / office staff | Verified official accounts | Read briefs; reply on items; mark items adopted / under consideration / declined | Signal; hide citizen content |

A person may hold more than one role (e.g. a steward is also a reviewer). Role-specific limits apply to the action, not the person.

## Steward activities

| Activity | Description | Sign-off |
| --- | --- | --- |
| Check change explanations | Fix errors in automatic explanations; resolve citations the parser could not match | Single |
| Mark well formed | Confirm a suggestion reads as legislative text, uses defined terms correctly, and does not break cross-references | Single |
| Offer wording fix | Propose cleaner drafting; author accepts or rejects | Single (author decides) |
| Combine duplicates | Link suggestions making the same change; authors notified; supporters keep support | **Two-party** |
| Re-anchor stale suggestions | After a new version, move suggestions whose meaning is unchanged; flag ones whose meaning changed | Single |
| Rider-tag context note | Short factual note on what a heavily tagged section does and how it relates to the bill's subject; tags never removed | **Two-party** |
| Answer factual questions | Sourced replies in line comments, marked steward context | Single |
| Tag topics | Label sections by subject for discovery | Single |
| Flag abuse | Send to moderators; cannot remove content | — |
| Link bill lineage | Confirm reintroductions and companions ([04](04-bill-lineage-and-competing-bills.md)) | Single |
| Group competing bills | Confirm shared issue; write neutral comparison ([04](04-bill-lineage-and-competing-bills.md)) | **Two-party** |

### Work queues
Stewards need queues for: unresolved/ambiguous citations, explanations pending check, unreviewed suggestions that have reached a support threshold, stale suggestions, duplicate candidates, rider-heavy sections, lineage and issue candidates, pending sign-offs, and appeals.

### Two-party sign-off
An action marked two-party takes effect only after a second steward, whose member belongs to a different party than the first, approves it. Pending actions are visible as pending.

## Safeguards on a partisan workforce

1. Every steward action is public and signed with the steward's name and their member's office.
2. Stewards cannot hide, rank, or remove suggestions. Their own signals carry no extra weight.
3. **Conflict rule:** stewards do not mark, combine, or annotate rider tags on bills their own member sponsors or co-sponsors. They may still answer factual questions there.
4. Rider-tag notes and combined duplicates require two-party sign-off.
5. **Appeals:** any author may ask for a second review by a steward from another party's office. Every appeal outcome is public.

## Domain concepts

- **Account** — a person; holds one or more role grants.
- **Role grant** — role, scope (jurisdiction; for officials, their office), verifying authority, start/end dates.
- **Office** — a sitting member's office; has a party affiliation and a list of sponsored/co-sponsored bills (from [01](01-bill-ingestion-and-display.md) metadata).
- **Steward action** — actor, office, action type, target, before/after content, timestamp, sign-off state, appeal link. Public.
- **Appeal** — requester, target action, assigned reviewing steward (different party), outcome, reasoning. Public.

## Rules and invariants

- Permission checks consider role, jurisdiction, and the conflict rule for every steward action.
- No role has a "hide for content" capability. Removal exists only for moderators and only for rule-breaking content ([09](09-integrity-moderation-and-privacy.md)).
- Steward and official status must be revoked promptly when staff leave an office or a member leaves office.

## Dependencies

- [08](08-onboarding-and-verification.md) — stewards must be ID-verified (tier 3).
- [09](09-integrity-moderation-and-privacy.md) — moderators, logs, community panel.
- [01](01-bill-ingestion-and-display.md), [02](02-versions-and-review-lifecycle.md), [03](03-review-features-and-support.md), [04](04-bill-lineage-and-competing-bills.md) — targets of steward actions.

## Open questions

- How is staff identity and employment confirmed with each office, and how is departure detected?
- Will enough offices from both parties lend staff time to make two-party sign-off workable? What is the fallback if one side is under-staffed?
- How are independents or third-party members treated in the two-party rule?
- Selection and vetting criteria for volunteer stewards (law clinics, law librarians).
- How are community moderators elected, and by whom?
