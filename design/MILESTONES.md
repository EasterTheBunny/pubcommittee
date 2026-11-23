# Delivery Milestones

High-level delivery plan for PubCommittee. Each milestone has a goal, what is in and out, the feature docs it touches, and exit criteria. Milestones stay stack-agnostic; stack decisions are made at the start of M0 and revisited at M1. [Prior work](#prior-work-ourdistrict) maps milestone features to reusable pieces of the earlier prototype.

| Milestone | Theme | Data | Users |
| --- | --- | --- | --- |
| [M0](#m0--proof-of-concept) | Proof of concept | Mocked | Internal team, demo audiences |
| [M1](#m1--real-bill-text-read-only) | Real bill text, read-only | Real Congress XML and US Code | Internal, invited readers |
| [M2](#m2--interactive-prototype) | Interactive prototype | Real bills, test accounts | Internal plus small invited testers |
| [M3](#m3--closed-pilot) | Closed pilot | Real | Invited residents, 2–3 legislative offices, stewards |
| [M4](#m4--public-launch-national) | Public launch, national | Real | Anyone in the US |
| [M5](#m5--verification-and-district-breakdowns) | Verification and district breakdowns | Real | All, with tiers |
| [M6](#m6--more-jurisdictions) | More jurisdictions | State and local feeds | State and local residents and officials |

---

## M0 — Proof of Concept

**Goal.** Show the whole experience end to end, on mocked data, so the concept can be demonstrated, tested with people, and critiqued. Every essential feature appears in its most basic form. Anything not yet real shows a pop-up describing the future behavior rather than doing it.

**Stack (M0 only).** SvelteKit with Tailwind CSS, built as a static site with no backend, archived at [`demo/`](../demo/) since M1-13 (Netlify builds `demo/` with `npm run build` and publishes `build`). The M1 app in `app/` is server-rendered from `read-api`.

**Principle.** Breadth over depth. A viewer should be able to walk every path in [the review cycle](02-versions-and-review-lifecycle.md#review-cycle-steps) and understand what the finished product will do at each step.

### Mocked data set
- 3 bills, hand-authored or adapted, chosen to cover:
  - an amendatory bill ("strike X, insert Y") with existing-law provisions;
  - a long bill with an obvious unrelated rider;
  - a competing pair on one issue, plus one cross-chamber companion.
- 2 official versions for at least one bill, to demonstrate stale suggestions.
- Pre-written change explanations and matching existing-law excerpts.
- Seeded comments, suggested edits, support counts, bill-level signals, and rider tags.
- A few mock personas: reader, reviewer, steward (two parties), moderator, office staffer.

### Feature coverage

| Feature | Working in M0 (basic) | Placeholder pop-up in M0 |
| --- | --- | --- |
| [01 Ingestion and display](01-bill-ingestion-and-display.md) | Bill page with sections and lines; two-layer view: explanation beside existing-law excerpt; side-by-side for suggestions | Automatic feed ingestion, citation parsing, as-amended preview |
| [02 Lifecycle](02-versions-and-review-lifecycle.md) | Version switcher; stale badge on suggestions touched by the second version | Automatic re-anchoring, supporter notifications |
| [03 Review and support](03-review-features-and-support.md) | Add line comment; write suggested edit; support a suggestion; support/oppose the bill with reason tags and rider section picks; counts as number and share; minimum threshold indicator | Revision history, withdraw-after-close rules |
| [04 Lineage and competing](04-bill-lineage-and-competing-bills.md) | Issue page with competing bills side by side and a canned neutral comparison; "alternatives" banner on bill page | Candidate detection, steward linking flow |
| [05 Roles and stewardship](05-roles-and-stewardship.md) | Persona switcher; per-role buttons shown or hidden; steward can mark "well formed" | Wording fixes, combine duplicates, two-party sign-off, appeals, work queues |
| [06 Sharing](06-sharing.md) | Share buttons on bill, section, line selection, suggestion, and thread; deep link opens on and highlights the target | **Social share buttons open a pop-up** describing the preview card and target networks; quote image download; version-pinned "changed since" notice |
| [07 Brief and responses](07-legislator-brief-and-responses.md) | Static brief page for one bill built from mocked data; office persona can set Adopted / Under consideration / Declined | Email delivery, export, cadence and event triggers, responsiveness record, amendment matching |
| [08 Onboarding and verification](08-onboarding-and-verification.md) | Fake "Sign in" that picks a persona | **Social sign-in buttons open a pop-up** describing tiers and location checks |
| [09 Integrity and privacy](09-integrity-moderation-and-privacy.md) | "Flag" button; canned spike disclosure in the brief | Moderation queue, public log, appeals, audit |
| [01 Bill selection and catalog](01-bill-ingestion-and-display.md#bill-selection) | Catalog page listing every mock bill with markup status, scope statement, and coverage breakdown (topic, sponsor party, administrator-selected); markup requests on eligible bills with private requests and "fewer than N" totals; steward queue with anonymous reasons; "in review because" line on bill pages | Official bill and text links (mock bills have none), steward accept/decline with two-party sign-off, outcome email |
| [08 Installable app and notifications](08-onboarding-and-verification.md#installable-app) | Installable progressive web app (manifest, icons, service worker): app shell offline, opened official text cached; in-app inbox joining mock public events with the persona's own supports, signals, and requests; per-browser notification opt-in with a content-free local notification on "simulate a batch" | Web Push delivery from the server, subscription storage |
| [10 Machine access](10-machine-access.md) | Path-based addresses for sections and line ranges alongside in-page fragments; user content marked as `user` in page markup | Structured forms, bulk exports, change feed, agent tool interface |
| [11 AI security](11-ai-security.md) | Bill-signal comments never shown with their author; small counts shown as "fewer than N"; seeded authors do not auto-support; no district or tier on public content | Batched count publishing, automation disclosure |

### Out of scope
Real data, real accounts, persistence beyond a demo session (reset to seed is acceptable), security hardening, accessibility audit, performance.

### Exit criteria
- A presenter can demo the full cycle in under 15 minutes: read bill, see explanation, comment, suggest, support, oppose with rider tag, share a line, switch versions and see stale, view issue page, view brief, respond as an office.
- Every placeholder pop-up names the feature doc it refers to.
- Feedback gathered from at least a handful of outside viewers, including at least one person with legislative staff experience.
- Stack decision recorded for M1 in [architecture/](../architecture/README.md).

---

## M1 — Real Bill Text, Read-Only

**Goal.** Rebuild the earlier national prototype on Congress's XML bill text and the US Code. Prove that real bills can be ingested, made addressable, and explained.

**In scope**
- Manual pull of bills and their official versions from the official source, started by an operator, with text stored as the architecture specifies ([01](01-bill-ingestion-and-display.md)). No automatic feed ingestion.
- Bill catalog for the current Congress; tracked set selected by the published policy in pilot mode (committee action and a focus topic) ([01](01-bill-ingestion-and-display.md#bill-selection)).
- Privileged administration (system access only, no product screens): select a catalog bill for review directly, correct ingestion errors, re-run ingestion, parsing, and generation, publish selection policy versions; every action in the public log ([05](05-roles-and-stewardship.md#platform-administration)).
- Installable app: add to home screen or desktop, app shell available offline, previously opened official text readable offline ([08](08-onboarding-and-verification.md#installable-app)).
- Public catalog: browse and search every bill, with links to the legislature's bill page and text versions, and markup status (*in review*, *eligible*, *not a markup candidate*) ([01](01-bill-ingestion-and-display.md#public-catalog)).
- Stable anchors on real text; line-level deep links ([06](06-sharing.md), without social cards).
- Citation resolution against the US Code; unresolved citations listed.
- Change explanations for a target set of bills; quality measured.
- Version history and stale classification computed between real versions ([02](02-versions-and-review-lifecycle.md)).
- Read access for machines ([10](10-machine-access.md)):
  - server-resolvable canonical address for every anchor, pinned to a version;
  - pages delivered with content already rendered, in semantic markup with anchors;
  - structured form of every page, with version, as-of time, provenance, and content class (`official`, `platform`);
  - bulk export of bill text, cited provisions, and change explanations per version;
  - change feed of new bills and versions;
  - long-lived caching of immutable versions; published rate limits;
  - machine-access notice and crawler directives.

**Out of scope.** Accounts, signals, briefs. Automatic feed ingestion (milestone not yet assigned).

**Exit criteria**
- Three real bills, one with multiple versions, display correctly with explanations.
- Measured citation resolution rate and explanation accuracy on a sample, with a target set for M3.
- Answers recorded for the M1 open questions in [01](01-bill-ingestion-and-display.md) (line unit, non-prose content).
- An automated client can fetch any line range by canonical address, without running page scripts, and the structured form matches the page.
- Structure of machine responses documented and versioned.

---

## M2 — Interactive Prototype

**Goal.** The M0 experience on M1's real text, with real persistence and test accounts. Prototype the side-by-side view and line comments on three real bills.

**In scope**
- Social sign-in, tier 1 only, test users ([08](08-onboarding-and-verification.md)).
- Comments, suggested edits, support, bill-level signal persisted ([03](03-review-features-and-support.md)).
- Markup requests on eligible catalog bills: one per reviewer, withdrawable, private, batched totals ([01](01-bill-ingestion-and-display.md#markup-requests)).
- Notifications: in-app inbox, and opt-in device notifications with content-free messages, sent in batches; no email ([08](08-onboarding-and-verification.md#notifications)).
- Basic steward actions: mark well formed, wording fix, re-anchor ([05](05-roles-and-stewardship.md)).
- Generated brief from live data, viewable as a page ([07](07-legislator-brief-and-responses.md)).
- Real share links with preview cards ([06](06-sharing.md)).
- Basic moderation: flag, remove with reason, log ([09](09-integrity-moderation-and-privacy.md)).
- Write barriers ([10](10-machine-access.md)): no programmatic write path, writes only from the product's own pages, per-account rate limits.
- Instruction-injection defenses ([10](10-machine-access.md#instruction-injection)): every text field labelled `official`, `platform`, or `user`; no user text in summary or preview fields; platform generation never takes user content as input.
- Inference defenses ([03](03-review-features-and-support.md), [11](11-ai-security.md)): counts published in batches; small counts shown as "fewer than N"; signal comments never shown with their author; no author auto-support; reading measured only in aggregate; notifications without position detail; public-content warning when writing.
- No listing or search of content by author; coarse public timestamps ([10](10-machine-access.md)).
- Review items added to bulk exports, keyed by item with per-bill pseudonyms; removal events in the feed.

**Exit criteria**
- Testers can complete the review cycle on real bills without guidance.
- Evidence on whether ordinary people can write usable suggested edits.
- Draft public charter for support and moderation rules, including the rule against automated submission and injection text treated as spam.
- Seeded injection comments never appear in summary or preview fields, and are labelled `user` in every machine response.
- A single test account's signal cannot be observed in published counts within one batch window.
- Decisions on: batch interval and the small-count threshold N.

---

## M3 — Closed Pilot

**Goal.** Test the two biggest unknowns: will offices read and answer the brief, and can residents produce usable legal text. Run for one legislative session.

**In scope**
- 2–3 recruited legislative offices receiving briefs and responding.
- Pilot scope: bills with committee action on healthcare, deficit reduction, and affordability, plus companions and two-party-approved nominations ([01](01-bill-ingestion-and-display.md#bill-selection)). Party balance of the tracked set published ([09](09-integrity-moderation-and-privacy.md#coverage-neutrality)).
- Steward program with staff from both parties; two-party sign-off; conflict rule; appeals ([05](05-roles-and-stewardship.md)).
- Steward review of markup requests and office nominations, with public accept or decline reasons ([01](01-bill-ingestion-and-display.md#markup-requests)).
- Administrator direct selection moves under two-party steward sign-off once stewards from two parties are active; role grants for stewards, moderators, and offices through privileged administration ([05](05-roles-and-stewardship.md#platform-administration)).
- Brief delivery by email on weekly cadence and before markup/floor events; export ([07](07-legislator-brief-and-responses.md)).
- Responses and responsiveness record.
- Lineage linking and issue pages with real competing bills ([04](04-bill-lineage-and-competing-bills.md)).
- Spike and referral disclosure ([09](09-integrity-moderation-and-privacy.md)), with coarse windows.
- Automation events detected and disclosed in the brief; confirmation step on signals and suggested edits ([10](10-machine-access.md)).
- Steward and office tools verified to show no individual signals or supporter lists; appeal requesters private ([09](09-integrity-moderation-and-privacy.md)).
- Optional read-only agent tool interface ([10](10-machine-access.md)).
- Public charter published.

**Exit criteria**
- Measured office response rate and time to respond.
- Measured share of suggestions marked well formed.
- Decisions on: delivery minimum, rider-tag threshold, brief recipients, and whether to show line-comment concerns beside suggestions.
- Decisions on: markup request threshold and steward capacity for accepted requests.
- Decisions on: AI-drafted text policy, when presence checks apply, per-bill pseudonyms on public pages, and export licence for user content ([10](10-machine-access.md), [11](11-ai-security.md)).

---

## M4 — Public Launch, National

**Goal.** Open to anyone located in the US, with national totals only, as in the product doc's launch scope.

**In scope**
- Everything from M3 hardened for public scale.
- Selection policy moves to general mode (committee action, no topic filter), or the topic list widens, as volume allows.
- Full moderation: elected community moderators, public log, appeals panel.
- Amendment matching and supporter notifications ("closing the loop").
- Independent audit of counts for the session, using per-bill unlinkable tokens and coarse timestamps ([09](09-integrity-moderation-and-privacy.md)).
- Write defenses at public scale: rate limits, confirmation step, behavioral detection, and proof of presence where decided in M3 ([10](10-machine-access.md)).
- Public bulk exports under the chosen licence, with removal events honored and old dumps rebuilt.
- Open-source release.

**Exit criteria**
- Audit completed without exposing individual signals.
- Stable operation through at least one markup and one floor vote on a high-interest bill.
- Inference review against every public output combined ([11](11-ai-security.md)): a model-assisted red team working from pages, exports, feed, and briefs cannot infer an individual's signal or link one display name across bills.

---

## M5 — Verification and District Breakdowns

**Goal.** Add tiers 2 and 3 so members see what their own constituents think.

**In scope**
- Address confirmation (voter-roll match or mailed code), optional ID verification ([08](08-onboarding-and-verification.md)).
- State and district breakdowns in briefs; tier splits; minimum cell size for privacy ([07](07-legislator-brief-and-responses.md), [09](09-integrity-moderation-and-privacy.md)).
- Complementary suppression across overlapping breakdowns; lineage merges applied at batch boundaries; decision on calibrated noise ([09](09-integrity-moderation-and-privacy.md#inference-threats), [11](11-ai-security.md)).
- District and tier never shown on public content ([08](08-onboarding-and-verification.md)).
- Vetted volunteer stewards.

**Trigger.** The participation level chosen under the open question in [08](08-onboarding-and-verification.md).

**Exit criteria**
- Inference review repeated with district and tier breakdowns included.

---

## M6 — More Jurisdictions

**Goal.** Extend to state legislatures and city councils.

**In scope**
- Per-jurisdiction ingestion, or steward manual entry where no feed exists.
- Per-jurisdiction consolidated code, or bill-only explanations.
- Jurisdiction-scoped eligibility and stewards.
- Machine access, exports, and feed per jurisdiction under the same rules ([10](10-machine-access.md)).

**Exit criteria**
- One state legislature live, using the same product model without special cases.

---

## Prior Work: ourdistrict

An earlier prototype, [`EasterTheBunny/ourdistrict`](https://github.com/EasterTheBunny/ourdistrict), covers parts of M1–M3. Its `development` branch (commit `6b3966a`) is the most complete. It is a Scala 2.11 / Lift 3 / Mapper app, so its value here is logic and data handling, not drop-in code.

- **License.** ourdistrict is GPLv3. Copying its code carries that license; re-implementing from its approach does not.
- **Paths.** Paths below are relative to `src/main/scala/code/` on the `development` branch unless noted.
- **Data source.** It consumes output from the [`unitedstates/congress`](https://github.com/unitedstates/congress) and [`unitedstates/congress-legislators`](https://github.com/unitedstates/congress-legislators) scrapers rather than calling official feeds directly.

### M1 — Real bill text, read-only

| Feature | Prior code | Reuse and gaps |
| --- | --- | --- |
| Feed ingestion of bill metadata (background job) | `lib/ParseBillsToDB.scala` (`JSONBillExtraction`, `ParseBillsToDB` actor) | Incremental merge of scraper `data.json` keyed by per-type timestamp lookup; reschedules every 7 days; merges actions, committees, sponsors, cosponsors, subjects. Known bug: the update path passes the lastmod file, not `data.json`, to `parseJSONFileToBill`. The job is disabled in `bootstrap/liftweb/Boot.scala:151`. |
| Bill text into addressable units | `snippet/DocumentPage.scala` `processBillLayer`, with `nodesToExtract` / `nodesToStore` / `nodesToTraverse` | Recursive walk of bill XML (`legis-body`, `resolution-body`, `preamble`, `quoted-block`) into a parent/child tree. Closest baseline for sections and lines. Runs on page view (`?initialize`); M1 must move it into the ingest job. |
| Stable anchors | `model/BillLayer.scala` (`hash` from XML `@id`, `parent`, `enum`, `header`, `text`, `proviso`, `*_raw`) | Keep the XML-id-as-anchor idea. Gaps: no version column, and the fallback hash is random, so anchors are not stable across versions. M1 needs (bill, version, id) keys. |
| Non-prose content ([01](01-bill-ingestion-and-display.md) open question) | `nodesToStore`: `graphic`, `formula`, `toc`, `list`, `table`, stored raw in `layer_raw` | Starting answer: treat each as one addressable unit stored raw. |
| Version selection | `DocumentPage.mostRecentBillMetaData` | Picks only the newest `text-versions/*/lastmod.json`. M1 must keep every version for the switcher and stale classification. |
| XML schema reference | `bill.dtd`, `res.dtd`, `amend.dtd` (repo root) | Use to define the parser's element list and tests. |
| Read API shape | `lib/JsonBillHandler.scala` (`/api/v1/bills/{id}/parts/{layer}/nodes/...`, JSON:API style) | Model for resource nesting, pagination, and filtering of the structured forms in [10](10-machine-access.md). Read routes only; its PUT routes are not carried over. |
| US Code citation resolution, change explanations, stale classification | None | New work. |

Swap points (done in M1-13: `types.ts` is now thin aliases over the generated API types; `mock/` and `diff.ts` remain only in `demo/`; `anchors.ts` and `links.ts` gained path addresses; the paths below are in `demo/`): `app/src/lib/types.ts` (`Section`/`Line` replace `BillLayer`; `Anchor` should carry the source XML id), `app/src/lib/mock/bills.ts` (replace with ingest output), `app/src/lib/anchors.ts` `isStale` (replace with a computed version diff), `app/src/lib/anchors.ts` and `app/src/lib/links.ts` (add path-based canonical addresses beside the `#s4-l12-13` fragment form), and `app/src/lib/diff.ts`.

### M2 — Interactive prototype

| Feature | Prior code | Reuse and gaps |
| --- | --- | --- |
| Threaded line comments | `model/NodeComment.scala` (`parent`/`parentHash`; `commentsForNode` builds the tree) | Flat-rows-to-tree pattern. Maps to `Thread`/`Comment` and `demo.reply` in `app/src/lib/state/demo.svelte.ts`. |
| Suggested edits | `model/LayerNode.scala` (statement and details on a `BillLayer`); PUT routes in `lib/JsonBillHandler.scala` | Per-unit edit proposal, maps to `SuggestedEdit`. `model/Node.scala` `addVersion` shows a revision-history pattern. Do not reuse the PUT routes: [10](10-machine-access.md) forbids a programmatic write path. |
| One support per user | `model/UserCommentVote.scala`, `NodeComment.doVote` | (user, item) row as source of truth with denormalized counters. Drop downvotes ([03](03-review-features-and-support.md) is support-only on suggestions). `doVote` builds SQL by string concatenation; use parameterized queries. Maps to `demo.toggleSupport`. |
| Sign-in | `model/User.scala` (`MegaProtoUser`, email and password, email validation skipped) | No social sign-in. Replace; keep only the username-uniqueness rule. |
| Share links | `src/main/webapp/assets/js/documentController.js` (Facebook and Twitter intent pop-ups) | Minimal. Preview cards are new work. |

### M3 — Closed pilot

| Feature | Prior code | Reuse and gaps |
| --- | --- | --- |
| Legislator roster for offices, steward party, conflict rule, brief recipients | `lib/ParseLegislatorsToDB.scala` (legislators YAML, reschedules every 30 days); `model/Sponsor.scala` (`bioguide_id`, `party`, `state`, `district`, contact fields) | Data source for office records and the party used by `app/src/lib/rules.ts` `stewardConflict`. Sponsor matching falls back to name, state, and title in `ParseBillsToDB.scala`; match on bioguide id only. |
| Lineage and competing bills ([04](04-bill-lineage-and-competing-bills.md)) | `Bill.related_bills` (`bill_id`, `reason`, `related_type`) | Direct input to companion and reintroduction candidate detection; replaces `LineageLink` in `app/src/lib/mock/issues.ts`. |
| Issue pages and topics | `model/Subject.scala`, `model/BillSubject.scala`, `lib/AutocompleteHandler.scala`, `snippet/SearchDocuments.scala` | Subject search and autocomplete can seed `Issue.tags` and `Section.topics`. |
| Brief export ([07](07-legislator-brief-and-responses.md)) | `lib/PDFServer.scala` (HTML to PDF) | Export pattern only; it currently exports discussion topics, not briefs. |
| Event-triggered brief delivery | `model/Action.scala` (`acted_at`, `type`, `text`) | Detect markup and floor events from bill actions to trigger pre-event briefs. |

### M4–M6

- **M4 integrity.** `User.ipaccesses`, `lastAccess`, `accessRep`, and `mapper/MappedAccessLog.scala` log per-user IP history. Treat as a caution, not a baseline: it conflicts with the privacy boundaries in [09](09-integrity-moderation-and-privacy.md).
- **M5 districts.** `Sponsor.state` and `Sponsor.district` give the member-to-district mapping for breakdowns. Nothing exists for address or ID verification.
- **M6 jurisdictions.** `settings.congress` and the bill-type directory walk are Congress-specific. Nothing generalizes; carry a jurisdiction key in the schema from M1.

### Not reused

`lib/RankedPairSort.scala` (a voting method; conflicts with "not a vote"), `model/Topic.scala` and `model/Node.scala` discussion trees, `model/Soapbox.scala`, `model/Visualization.scala` and `snippet/ChartsPage.scala` (user-submitted scripts), and the jQuery/D3 tree UI.
