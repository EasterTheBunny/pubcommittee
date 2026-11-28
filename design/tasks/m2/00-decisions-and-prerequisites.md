# M2-00 — Decisions and Prerequisites

**Phase:** local and cloud. M2's first task. It collects the decisions that other M2 tasks cannot assume. Local decisions are needed before the local-phase tasks they block; cloud decisions and the operator steps are needed only before the cloud phase ([environments](environments.md)). Each decision is recorded where its open question lives, then here.

## Goal

Every M2 task starts with its blocking decisions answered, or with a recorded default the operator accepted for the prototype, and with the domain and providers it needs in place.

## Read first

- [MILESTONES § M2](../../MILESTONES.md#m2--interactive-prototype)
- [architecture § Open decisions](../../../architecture/README.md#open-decisions) and [§ Milestone fit](../../../architecture/README.md#milestone-fit)
- [architecture § Identity and personal data](../../../architecture/README.md#identity-and-personal-data)
- Open questions in [03](../../03-review-features-and-support.md#open-questions), [06](../../06-sharing.md#open-questions), [08](../../08-onboarding-and-verification.md#open-questions), [09](../../09-integrity-moderation-and-privacy.md#open-questions), [10](../../10-machine-access.md#open-questions), [11](../../11-ai-security.md#open-questions)

## Scope

**In: decisions** (operator; product, policy, legal, or spending). Each needs an answer, or an accepted prototype default, before the tasks named:

| Decision | Phase | Blocks | Source |
| --- | --- | --- | --- |
| Domain for `<domain>` and `api.<domain>` (same-site cookies) | cloud | cloud parts of 05; 16 | architecture § Open decisions |
| Serverless Postgres vendor, and whether its resume latency is acceptable for sign-in | cloud | cloud part of 02; 16 | architecture § Open decisions |
| Sessions in the document store or SQL | local | 05 | architecture § Open decisions |
| `k_signal` and other HMAC key rotation procedure | local | 01 (`Keys` port shape), 07; 03 in the cloud phase | architecture § Open decisions |
| Batch interval and small-count threshold N (prototype values; final values are an M2 exit decision) | local | 08 | 03, 11 |
| Noise or exact counts with batching and suppression | local | 08 | 09, 11 |
| Licence for user content in exports | local | 14 | 10 |
| AI-drafted text submitted by a person: allowed, disclosed, or prohibited | local | 06, 17 | 10, 11 |
| Display names on pages, or per-bill pseudonyms as in exports | local | 06, 15 | 10, 08 |
| Share links: human-readable only, or short-code aliases too; live or frozen preview counts | local | 12 | 06 |
| Can a suggested edit span non-contiguous ranges | local | 06 | 03 |
| Rider tags in M2 or later | local | 07 | 03 |
| Account suspension in M2, or removal only | local | 10 | 09 |
| Attribution of steward and moderator actions: name or public role id | local | 10 | 11 |
| Per-reader rate limits at the edge: load balancer with Cloud Armor now, or after M2 | cloud | 16 | architecture § Open decisions |
| Test users: seeded local accounts (local); who, how many, and how real testers are invited (cloud) | local and cloud | 05, 17 | MILESTONES § M2 |

**In: operator steps for the cloud phase** (printed by agents, run by the operator; none is needed locally):
- Register the domain; point `<domain>` and `api.<domain>` at the hosting edge ([M2-16](16-infrastructure.md)).
- Create OAuth clients for Google (`openid` only) and Facebook (`public_profile`), with redirect URIs on `api.<domain>`, per environment. Client secrets go to Secret Manager, never to either repository.
- Generate the VAPID key pair for Web Push; the private key goes to Secret Manager.

**Out**
- Tier 2 and tier 3 verification, district breakdowns (M5). Email notifications (deferred). Two-party sign-off and the steward program (M3).

## Decided

Local decisions (operator, 2026-11-28), each also recorded under its source open question:

| Decision | Answer |
| --- | --- |
| Sessions | SQL (`sessions` in private Postgres); the document store is public |
| Key rotation | Versioned HMAC results from M2-01 on; a later re-keying job rewrites signal keys from each person's `my_signals`; M2 runs on version 1 |
| Batch interval | Hourly (prototype; final at M2 exit) |
| Small-count threshold N | A configuration value that grows with the user base, starting at 5 |
| Noise or exact counts | Exact counts with batching and suppression; no noise |
| Names on pages | Per-bill pseudonyms on pages and in exports |
| AI-drafted text by a person | Allowed, with no label; automated submission stays prohibited |
| User-content licence | CC BY 4.0, attributed to the per-bill pseudonym |
| Share links | Human-readable canonical links only; cards show the latest batch (live) |
| Suggested-edit ranges | One contiguous range in one section |
| Rider tags | In M2 ([M2-07](07-signals-and-markup-requests.md)) |
| Account suspension | In M2 ([M2-10](10-moderation-and-steward-actions.md)) |
| Attribution of steward and moderator actions | Stable public role id; the operator holds the mapping |
| Local test users | Seeded by `scripts/local.sh seed`: a tester, a steward, a moderator |

Still open, for the cloud phase: domain, Postgres vendor, edge rate limits, real testers, and the operator steps above.

## Deliverables

- Answers recorded in the source docs' open questions, marked **Decided (operator, date)**, and listed in a table here.
- Operator steps listed as complete in [STATUS.md](../STATUS.md).

## Acceptance criteria

- **Local phase:** every local decision has an answer or an accepted prototype default, recorded in its source doc, before the task it blocks starts.
- **Cloud phase:** every cloud decision is answered; Google and Facebook sign-in clients exist for `dev`, their secrets only in Secret Manager; `<domain>` and `api.<domain>` resolve in `dev`.

## Open questions

The table above is the list. A task-orchestrator answers none of them; it stops and reports.
