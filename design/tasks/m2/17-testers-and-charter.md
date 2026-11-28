# M2-17 — Tester Sessions and Draft Charter

**Phase:** local (materials, charter, operator walkthrough), then cloud (sessions with invited testers in `staging`).

## Goal

The M2 exit evidence that only people can give: testers complete the review cycle on real bills without guidance, their suggested edits are judged for usability, and a draft public charter exists for support and moderation.

## Read first

- [MILESTONES § M2](../../MILESTONES.md#m2--interactive-prototype) (exit criteria)
- [03](../../03-review-features-and-support.md), [09 § Moderation](../../09-integrity-moderation-and-privacy.md#moderation), [10 § Write barriers](../../10-machine-access.md#write-barriers), [11](../../11-ai-security.md)

## Scope

**In (agents prepare; the operator runs)**
- Session plan: tasks per tester on the three M1 exit bills, observation notes, and a short questionnaire, with no personal data beyond what testers volunteer.
- Rubric for "usable suggested edit" (reads as legislative text, right defined terms, no broken cross-references), used by the operator or a volunteer drafter.
- Draft public charter: support rules, moderation rules with numbered rules that removals cite, the rule against automated submission, injection text treated as spam, and AI-assisted drafting allowed with no label (decided in [M2-00](00-decisions-and-prerequisites.md)).
- A findings report: completion without guidance, suggested-edit usability rate, issues found, and the proposed batch interval and N.

**Out**
- Recruiting offices or stewards (M3).

## Acceptance criteria

- **Local phase:** the operator walks through the session plan on `scripts/local.sh up` with the seeded accounts; issues found are fixed or recorded.
- **Cloud phase:** sessions held in `staging` with the testers from [M2-00](00-decisions-and-prerequisites.md); findings report recorded.
- Draft charter published in the repository and linked from the app.
- Batch interval and N decided by the operator from the findings (MILESTONES § M2 exit).

## Open questions

- The manipulation test that does not depend on position ([10 § Open questions](../../10-machine-access.md#open-questions)).
