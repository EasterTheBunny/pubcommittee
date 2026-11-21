# Design Docs: PubCommittee

General design docs derived from [`PRODUCT_DOC.md`](../PRODUCT_DOC.md). Each doc covers one top-level feature: its purpose, scope, behavior, domain concepts, rules, and open questions.

These docs are deliberately **stack-agnostic**. They name no languages, frameworks, databases, hosting, or vendors. Where the product doc names an external source (Congress bill XML, the US Code XML, Google and Facebook sign-in), it is treated as an input requirement, not an implementation choice.

## Feature docs

| # | Doc | Covers |
| --- | --- | --- |
| 01 | [Bill Ingestion and Display](01-bill-ingestion-and-display.md) | Selecting which bills enter review, the public bill catalog and markup requests, pulling official versions, addressable sections and lines, linking to existing law, change explanations |
| 02 | [Versions, Amendments, and Review Lifecycle](02-versions-and-review-lifecycle.md) | Bill review states, new official versions, stale and re-anchored items |
| 03 | [Review Features and Support](03-review-features-and-support.md) | Line comments, suggested edits, bill-level signal, rider tags, support rules |
| 04 | [Bill Lineage and Competing Bills](04-bill-lineage-and-competing-bills.md) | Reintroduced copies, cross-chamber renames, issue pages, candidate detection |
| 05 | [Roles and Stewardship](05-roles-and-stewardship.md) | Role model, permissions, steward activities, partisan safeguards, appeals |
| 06 | [Sharing](06-sharing.md) | Deep links to any text, version pinning, preview cards, quote images |
| 07 | [Legislator Brief and Responses](07-legislator-brief-and-responses.md) | Brief composition, delivery cadence, export, office responses, responsiveness record, closing the loop |
| 08 | [Onboarding and Verification](08-onboarding-and-verification.md) | Sign-in, verification tiers, location signals, jurisdiction assignment |
| 09 | [Integrity, Moderation, and Privacy](09-integrity-moderation-and-privacy.md) | Threat model, campaign disclosure, moderation rules and log, privacy boundaries, audit |
| 10 | [Machine Access](10-machine-access.md) | Read access for automated clients, exports and feeds, provenance, barriers to automated writes |
| 11 | [AI Security and Inference Threats](11-ai-security.md) | Signal inference, profiling and re-identification, generated campaigns, risks to reading agents |

## Delivery

See [MILESTONES.md](MILESTONES.md): M0 proof of concept on mocked data through M6 additional jurisdictions.

Stack, hosting, and storage choices live outside these docs, in [architecture/](../architecture/README.md).

## Dependency sketch

```
01 Ingestion ──► 02 Lifecycle ──► 03 Review ──► 07 Brief
     │                │              │            ▲
     └──► 04 Lineage ◄┘              ▼            │
                              06 Sharing          │
08 Verification ──► 03 Review, 07 Brief (tiers)   │
05 Roles ──► gates actions in 01–07               │
09 Integrity ──► constrains 03, 05, 06, 07 ───────┘
10 Machine access ──► read side of 01–07; write barriers on 03, 07
```

## Shared vocabulary

| Term | Meaning |
| --- | --- |
| Bill | A legislative measure with one or more official versions |
| Official version | One published text of a bill (introduced, committee substitute, engrossed, enrolled, etc.) |
| Anchor | A stable reference to a section, line, or line range within a specific official version |
| Provision | An addressable unit of existing law (e.g. a US Code section or subsection) |
| Change explanation | Plain-language account of what a bill does to a provision of existing law |
| Reviewer | A signed-in participant eligible to comment, suggest, and signal |
| Signal | Support on a suggested edit, or support/oppose on a bill |
| Brief | The per-bill summary delivered to a legislator's office |
| Steward | Verified legislative staff (later, vetted volunteers) who keep review legible |
| Jurisdiction | The legislature a bill belongs to; at launch, the US Congress |

## Global principles

These apply across every feature and should be treated as invariants:

1. **Citizens never change bill text.** Only an official version from the legislature changes what is reviewed.
2. **Not a vote.** The system is a communication channel. No output is framed as a decision.
3. **No suppression by position.** No role can hide, rank down, or remove content because of its politics.
4. **Every privileged action is public and attributed.** Steward, moderator, and office actions are logged and visible.
5. **Signals are private, totals are public.** Who supported what never leaves the system.
6. **Higher verification never adds weight.** Tiers change *where* support counts, not *how much*.
7. **Jurisdiction-general model.** Launch targets Congress, but no concept should assume Congress specifically.
8. **Machines read, people write.** Everything public is open to automated reading. Every comment, suggestion, signal, flag, and office response requires a person in a live interactive session; no interface lets software write on someone's behalf (see [10](10-machine-access.md)).
