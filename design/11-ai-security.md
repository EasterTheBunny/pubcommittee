# 11 — AI Security and Inference Threats

## Purpose

Collect in one place the threats that automated and AI-assisted adversaries pose to the product, and the defenses each feature doc must carry. AI changes the cost, not the kind, of most attacks: correlating public data across every account and every bill, linking pseudonyms to real people, writing varied fake comments, and watching counts minute by minute all become cheap. The design must assume an adversary that reads everything public, continuously, and reasons over it.

This doc is a summary and index. The binding rules live in the feature docs it links to.

## Scope

**In scope**
- Inferring private signals from public data
- Profiling and re-identifying reviewers
- Automated and AI-generated writes
- Risks to agents that read the product (injected instructions, misquotation)
- The platform's own automatically generated content

**Out of scope**
- Conventional application security (authentication, injection, transport); covered at build time
- Verification mechanics ([08](08-onboarding-and-verification.md))

## Core rule

> **No published figure may change in response to a single person's action within a window short enough to attribute it.**

Everything a machine can read is assumed to be read, stored, and compared over time. Privacy is judged against the combination of all public outputs (pages, structured forms, exports, feeds, briefs, logs), not against each one alone.

## Adversaries

| Adversary | Goal | Typical capability |
| --- | --- | --- |
| Employer, neighbor, family member | Learn one known person's position | Knows the person's display name or habits; watches counts |
| Political operation or data broker | Build voter profiles at scale | Full scrape, cross-platform data, language models |
| Legislative office or partisan staff | Identify or retaliate against constituents | Briefs, district figures, steward tools |
| Campaign or bot operator | Inflate or fake support | Many accounts, generated text, agents driving browsers |
| Content poster | Manipulate agents that read the product | Writes comments containing instructions to AI readers |

## Threats and defenses

### Inferring private signals

| Threat | Attack | Defense | Owner |
| --- | --- | --- | --- |
| Signal comment exposure | Comment on a bill signal shown with the author reveals the signal | Signal comments are never shown with the author | [03](03-review-features-and-support.md) |
| Small counts | A count of 1 or 2 identifies the author or a known friend | Authors do not auto-support; small counts shown as "fewer than N" | [03](03-review-features-and-support.md) |
| Count differencing over time | 41 → 42 right after a known person acts | Published counts update in batches, never per event; optional calibrated noise | [03](03-review-features-and-support.md), [09](09-integrity-moderation-and-privacy.md), [10](10-machine-access.md) |
| Intersection of breakdowns | National minus other districts; tier minus tier-in-district | Minimum cell size plus complementary suppression; consistent noise across releases | [09](09-integrity-moderation-and-privacy.md) |
| Withdrawals, merges, resets | Drop after a public falling-out; overlap revealed by a lineage merge | Same batching; merges applied at a batch boundary | [09](09-integrity-moderation-and-privacy.md) |
| Text-to-signal classification | Model predicts each commenter's signal from their public comments | Cannot be prevented; users warned that comments are public and may reveal positions | [03](03-review-features-and-support.md) |
| Audit export linkage | Pseudonymous per-signal records matched to public comment times | Per-bill unlinkable tokens, coarse timestamps, auditor agreement; prefer verification without per-signal records | [09](09-integrity-moderation-and-privacy.md) |

### Profiling and re-identification

| Threat | Attack | Defense | Owner |
| --- | --- | --- | --- |
| Cross-bill profiling | All comments by one display name form a political profile | No per-author activity listing or search; exports keyed by item, with per-bill pseudonyms | [10](10-machine-access.md) |
| Re-identification | Stylometry, self-disclosed details, activity hours, cross-posting link a display name to a person | Pseudonymous display names encouraged; coarse public timestamps; optional per-bill pseudonym | [08](08-onboarding-and-verification.md), [10](10-machine-access.md) |
| Location badges | District or tier shown on a comment narrows the candidate set | District and tier never shown on public content | [08](08-onboarding-and-verification.md) |
| Reading history | Per-person "opened this bill" records reveal interests | Reading measured only in aggregate | [03](03-review-features-and-support.md) |
| Public appeal requester | Naming who appealed reveals they were moderated | Appeal outcomes public; requester not | [09](09-integrity-moderation-and-privacy.md) |
| Privileged tools | Stewards or offices see who supported what | No steward or office tool shows individual signals or supporter lists | [05](05-roles-and-stewardship.md), [09](09-integrity-moderation-and-privacy.md) |
| Notifications | Forwarded "your supported suggestion…" messages reveal support | Notifications carry no position detail; link to the item | [03](03-review-features-and-support.md) |
| Spike timing | Precise window ties a surge to one public figure's post | Disclosure windows reported coarsely | [09](09-integrity-moderation-and-privacy.md) |
| Permanent copies | Removed or withdrawn content persists in mirrors and training sets | Feed emits removal events; export licence requires honoring them; old dumps rebuilt, not retained | [10](10-machine-access.md) |

### Automated and generated writes

| Threat | Attack | Defense | Owner |
| --- | --- | --- | --- |
| Software writing for a person | API tokens, delegated app access | No programmatic write path exists | [10](10-machine-access.md) |
| Agent in a person's browser | Agent operates a signed-in session | Rate limits, confirmation step, proof of presence on signals and suggestions | [10](10-machine-access.md) |
| Generated sockpuppet campaigns | Many accounts post varied, model-written comments that evade identical-text checks | Verification tiers; behavioral and cross-account signals; disclosure as an automation event | [08](08-onboarding-and-verification.md), [09](09-integrity-moderation-and-privacy.md) |
| AI-drafted text by a real person | A resident submits model-written suggestions | Policy undecided (see open questions) | [10](10-machine-access.md) |

### Risks to agents that read the product

| Threat | Attack | Defense | Owner |
| --- | --- | --- | --- |
| Injected instructions | A comment tells AI readers to misdescribe the bill | Every text field labelled `official`, `platform`, or `user`; no user text in summary fields; read-only agent tools; injection text moderated as spam | [10](10-machine-access.md#instruction-injection) |
| Injection into platform generation | User text steers generated explanations or brief summaries | Generation never takes user content as input, or only as quoted, labelled data | [10](10-machine-access.md#instruction-injection) |
| Misquotation | Agent quotes stale text, an unofficial preview, or an unreviewed explanation as fact | Every response carries version, as-of time, and provenance | [10](10-machine-access.md) |

### The platform's own generated content

| Threat | Attack or failure | Defense | Owner |
| --- | --- | --- | --- |
| Wrong change explanations | Automatic explanation misstates what a bill does | Provenance flag; steward review queue; accuracy bar before unreviewed display | [01](01-bill-ingestion-and-display.md) |
| Generated brief summaries | Summary shifts emphasis or invents content | Brief quotes authors' own words; any generated summary labelled | [07](07-legislator-brief-and-responses.md) |

## Rules and invariants

- No published figure changes in response to one person's action within an attributable window.
- Privacy is assessed against the union of every public output over time.
- District, tier, and precise timestamps never appear on public content.
- No interface lists or searches one person's activity across bills.
- User content is always distinguishable from official and platform content in machine responses.
- Suspected automation is disclosed, never silently discounted.

## Open questions

- **Noise or suppression:** add calibrated noise (differential privacy) to published counts, or publish exact counts with batching and suppression? Noise is stronger against differencing but harder to explain to legislators and auditors.
  - **Decided (operator, 2026-11-28, M2 prototype):** exact counts with batching and suppression.
- **Batch interval:** hourly, daily, or adaptive to the item's activity?
  - **Decided (operator, 2026-11-28, M2 prototype):** hourly.
- **Small-count threshold** for "fewer than N", and whether it matches the brief-eligibility minimum.
  - **Decided (operator, 2026-11-28, M2 prototype):** a configuration value that grows with the user base, starting at N = 5; separate from the brief-eligibility minimum.
- **AI-drafted text** submitted by a person: allowed, disclosed, or prohibited?
  - **Decided (operator, 2026-11-28):** allowed, with no label.
- **Privileged-role attribution:** principle 4 makes steward and moderator actions public and attributed, which exposes those people to harassment. Attribute to a public role ID with identities held by an oversight body?
  - **Decided (operator, 2026-11-28):** a stable public role id (for example `moderator-3`) in the public log; the operator holds the mapping to people, later an oversight body.
- How is detection of generated campaigns described publicly without teaching evasion?
