# PubCommittee: Line-by-Line Public Input on Active Bills

Nov 12, 2026 · @Awbrey

## Summary

PubCommittee gives members of Congress a clear, specific read of what the public wants changed in a bill, and gives the public a direct, shareable path to the bill's actual text. It lets the public review every active bill the way an editor reviews a draft. People comment on any line, suggest exact replacement text, and support or oppose the bill as a whole. Offices receive a short brief they can act on in minutes.

It is not a vote. It is a more granular communication channel to existing representatives, and what they do with a bill remains their call.

**What it is not.** It does not make law, and nothing citizens do changes the bill's text. It adds no obligation: sponsors and committees keep full control of their bills, and answering a brief is optional, though visible.

The first version covers the US Congress, building on an earlier prototype for national bills. The model itself is general and can extend to state legislatures and city councils.

## Problem statement

Members hear from the public constantly, but rarely in a form they can act on, and the public conversation about bills now happens largely on social media, cut off from the text itself.

- **Public input is blunt.** Calls, petitions, and testimony are mostly for or against a whole bill. None of it says "change *shall* to *may* in section 4(b)", and none of it shows how many people agree.
- **Bills are hard to read, and riders hide inside them.** Complex bills bundle provisions unrelated to their subject. The public cannot easily see what a bill changes in existing law, and objections to a single rider get lost in general opposition.
- **The same legislation is hard to follow.** Bills are refiled as copies under new names, change names between the House and Senate, and compete with other bills on the same issue. Even engaged citizens lose track.
- **Social media misquotes and misinforms.** Posts paraphrase bills, pull clauses out of context, or attribute provisions that are not in the bill at all, usually with no link to the words in question. Corrections travel slower than the original claim.

**What changes.** A clear path straight to the bill text, linkable down to a single line, gives social media a low-friction way to quote a bill accurately and comment on it directly. Instead of arguing over a screenshot or a paraphrase, people open the exact line, see what it changes in current law, and leave a signal their representatives can use.

## Value by persona

Each group that touches a bill gets something it does not have today, and legislators gain the most: a clearer signal, a better use of social media, and a way to point every argument back to the text.

| Persona | What they get | In practice |
| --- | --- | --- |
| Members of Congress | A specific, countable read of public sentiment on individual provisions, and one more tool that turns social media attention toward the actual bill instead of against it | A member answers a viral misreading by sharing a link to the exact line, with its change explanation beside it |
| Legislative staff | Briefs written like amendment memos, rider tags that show where a bill loses support, and a visible, credible role as stewards | A staffer sees that most opposition to a bill cites one unrelated section, before the markup |
| Citizens | A way to send clearer signals than a call or petition: about a specific line, a specific fix, or a specific rider, with a public reply from their representative | A resident suggests replacing "shall" with "may", gathers support, and sees the office mark it under consideration |
| Anyone sharing on social media | A low-friction way to quote a bill accurately and link straight to the words, plus a place to comment on the text itself | A post quotes section 7 with a card that opens on section 7, not a screenshot |

**Hoped-for side effects.** These are hopes, not promises, but the design leans toward them.

- **Simpler legislation.** When "too complex" and "unrelated riders" are counted and delivered to every member, bundling unrelated provisions carries a visible cost, which may encourage shorter, single-subject bills.
- **More cross-aisle communication.** Stewards from both parties' offices sign off on shared work, competing bills are compared side by side in neutral terms, and people of every party review the same text. Each of these creates a reason for the two sides to talk about the same words rather than past each other.

## Launch scope: US Congress

The first version covers bills in the US Congress, building on an earlier prototype for national bills; the model stays general so state and local legislatures can follow.

| Decision | At launch | Later |
| --- | --- | --- |
| Legislature | US Congress, House and Senate bills | State legislatures and city councils |
| Who can participate | Anyone located in the US, after a social sign-in | Tiered address and ID confirmation (see Onboarding and verification) |
| Signal granularity | National totals only; every member receives the same figures | State and district breakdowns once addresses are confirmed |
| Bill text | Official XML published for each bill version, scraped automatically | Other legislatures' feeds, or steward entry |
| Existing law | The US Code, also published as XML | Each jurisdiction's consolidated code |
| Fast-moving bills | Not a priority; at the national level they are mostly procedural | Revisit if substantive bills start outpacing the brief |

National-only totals suit high-level national conversation and let identity sort itself out over time. The cost is that a member cannot yet see what their own constituents think, so some offices may weigh the brief less until district breakdowns arrive.

## How a bill is shown

Every bill is shown in two layers: what it would change in current law, and the bill's own text, where people comment and suggest edits.

**Two kinds of comparison.** The two layers use different comparisons, kept visually distinct.

- **Bill against law: an explanation.** Bills are rarely clean replacement text. Many are instructions ("in section 12, strike 'thirty' and insert 'sixty'"), and others add whole new chapters. The system shows each affected provision as it currently reads, then explains what the bill would do to it and why that matters. Where the instruction is mechanical, it can also preview the provision as amended, labelled unofficial.
- **Citizen edit against bill: side by side.** Suggested edits target the bill's own text, so they are shown as a direct comparison: the bill's lines and the suggested replacement side by side, deletions struck and insertions marked.

**Finding existing law.** Locating the law as it stands today is its own problem. The system needs a consolidated, current version of the code (many jurisdictions publish one, though it often lags recent enactments), a parser that resolves the bill's citations, such as "Sec. 12 of Title 7", to specific provisions, and a steward check wherever a citation is ambiguous or the code is out of date. Where no consolidated code exists, the explanation is written from the bill alone and marked as such.

A suggestion that gathers support is delivered to legislators as a request. The bill changes only if its sponsors or a committee push a new official version, which citizens then review in turn.

## Bill lineage and competing bills

A bill's number and name are not a reliable identity, so the system tracks lineage separately: stewards link copies and renamed bills into one continuous review, while bills that tackle the same issue differently stay separate and are shown as competing implementations.

The earlier national prototype ran into all three cases below.

| Case | What happens in Congress | How the system handles it |
| --- | --- | --- |
| Reintroduced copy | A bill is filed again, often in a new Congress, with the same or nearly the same text under a new number or name | Stewards link it to the earlier bill. Comments and suggestions carry over, re-anchored to the new text; supporters are notified and can withdraw |
| Renamed between houses | The same legislation appears under a different name or number in the House and the Senate | Stewards link both into a single review page, so citizens see one bill and one set of signals, with each chamber's text shown as a version |
| Competing bills | Several bills address the same issue with different details at the same time, often because the parties disagree on implementation or riders | Grouped under a shared issue, never merged or marked as duplicates. Each bill keeps its own review and signals |

**Finding candidates.** The system compares new bills against existing ones by text similarity and by the US Code sections they amend. High text overlap suggests a copy or a renamed bill; overlap in the sections amended but different text suggests a competing bill. The system only suggests links; a steward confirms each one.

**Competing bills in the product.** An issue page lists the competing bills side by side, with a short neutral comparison of how each one differs and their support and opposition counts. Each bill's own page shows a banner naming the alternatives. The brief to a legislator shows the competing bills together, so an office sees whether the public prefers one approach over another rather than reading each bill in isolation.

## Participants and roles

Citizens review and support; everyone else exists to keep suggestions legible and the signal trustworthy. No role can hide a suggestion because of its position.

| Role | Who | Can do | Cannot do |
| --- | --- | --- | --- |
| Reader | Anyone, no account | Read bills, comments, suggestions, support counts | Comment, suggest, or support |
| Reviewer | Verified resident | Comment on lines, write and support suggested edits, support or oppose the bill | Support items on bills outside their jurisdiction |
| Steward | Verified staff of sitting members of Congress at launch; vetted volunteers later | Mark a suggestion "well formed", propose wording fixes, combine duplicates, tag topics | Hide a suggestion for its politics; add weight to support |
| Moderator | Platform staff plus elected community moderators | Remove abuse, spam, doxxing; suspend accounts; publish every action in a log | Change the substance of a suggestion |
| Legislator or staff | Verified official accounts | Read briefs, reply on suggestions, mark one "adopted", "under consideration", or "declined, with reason" | Support items, or hide citizen content |

### Stewards

Stewards are verified staff from the offices of sitting members of Congress who keep the review legible: they check that text and explanations are accurate, never whether an idea is good.

Staff already read bills, know drafting conventions, and know the US Code, so they can do this work faster and more credibly than volunteers. It also gives offices a direct, useful role in the system rather than only receiving briefs. Vetted volunteers, such as law school clinics and law librarians, can join later.

**What a steward does**

| Activity | What it looks like |
| --- | --- |
| Check change explanations | Reviews the automatic explanation of what a bill changes in existing law, fixes errors, and resolves citations the parser could not match to a US Code section |
| Mark suggestions well formed | Confirms a suggested edit reads as legislative text, uses defined terms correctly, and does not break cross-references; unmarked suggestions still collect support, labelled "not reviewed" |
| Offer wording fixes | Proposes a cleaner drafting of a citizen's suggestion; the author decides whether to accept it |
| Combine duplicates | Links suggestions that make the same change so support is not split; authors are notified and supporters keep their support |
| Re-anchor stale suggestions | After a new official version, moves suggestions to the new line numbers where the meaning is unchanged, and flags the ones whose meaning changed |
| Add context to rider tags | When many residents tag a section as an unrelated rider, adds a short factual note on what the section does and how it relates to the bill's subject; the tags themselves are never removed |
| Answer factual questions | Replies in line comments with sourced facts, such as what a defined term means or what a cost estimate says, marked as steward context |
| Tag topics | Labels sections by subject so people can find every bill touching, for example, veterans' health care |
| Flag abuse | Sends harassment, spam, or personal data to moderators; stewards cannot remove content themselves |
| Link bill lineage | Confirms when a new bill is a reintroduced copy of an old one, or the same bill under another name in the other chamber, and joins them into one continuous review |
| Group competing bills | Confirms that bills address the same issue, places them under a shared issue page, and writes the neutral comparison of how they differ; needs sign-off from two parties' stewards |

**Safeguards on a partisan workforce.** Staff work for elected members of a party, so the role is built to keep that from shaping the signal.

- Every steward action is public and signed with the steward's name and their member's office.
- Stewards cannot hide, rank, or remove suggestions, and their own support or opposition carries no extra weight.
- Stewards do not mark, combine, or annotate rider tags on bills their own member sponsors or co-sponsors; they may still answer factual questions there.
- Rider-tag notes and combined duplicates need sign-off from two stewards whose members belong to different parties.
- Any author can ask for a second review by a steward from another party's office, and the outcome of every appeal is public.

## Review cycle of a bill

A bill enters review when it is introduced and leaves when the legislature takes its final vote or the bill dies. In between, citizens comment, suggest, and support, and every official amendment reopens review on the lines it touches.

&#91;embedded content: review cycle of a bill · 3 review features, 1 amendment loop\]

The accented step is the product: everything before it exists to make that delivery precise and credible.

1. **Bill opened for review.** Each official version is pulled from the legislature's publication feed, split into addressable sections and lines, linked to each provision of existing law it touches, and paired with an explanation of what would change. Where no machine-readable feed exists, stewards enter the text and mark it as such.
2. **Citizen review.** Residents leave line comments, write suggested edits, and mark the bill as a whole, all described in the next section. Authors can revise their own suggestions, and every revision is kept.
3. **Signals collected.** Residents support the suggestions they want and support or oppose the bill. There is no voting window: signals accumulate for as long as the bill is in review.
4. **Delivered to legislators.** Items that reach the minimum support are bundled into a brief, sent on a regular cadence and immediately before key events such as a committee markup or floor vote. The top line-comment concerns travel with them.
5. **Legislator responds.** Offices reply publicly on each item. A reply is not required, but silence is shown as "no response" next to the member's name.

When the legislature amends the bill, the new official version is loaded automatically. Suggestions on lines that changed are flagged stale. The author re-anchors the suggestion to the new text; if its meaning still holds, support carries over and supporters are notified so they can withdraw, and if the meaning changed, support resets.

## Review features and support

Citizens have three ways to review a bill: comment on lines, suggest changes, and give an overall verdict. Only the overall verdict allows opposition; suggested edits can only be supported.

| Feature | Scope | What a reviewer writes | Signal |
| --- | --- | --- | --- |
| Line comment | One line or a range of lines | Questions, concerns, context, local impact | None; threads are discussion only |
| Suggested edit | One line or a range of lines | Exact replacement text, or a deletion, plus a short plain-language reason | Support, or nothing |
| Bill-level signal | The whole bill | Nothing required; an optional comment and reason tags | Support or oppose the bill |

**Line comments.** Any verified resident can comment on any line. Comments are for understanding and debate, so they carry no count. The brief quotes the most active thread on each section as its top concern.

**Suggested edits.** A suggestion is shown as a side-by-side comparison: the bill's current lines beside the suggested replacement, deletions struck and insertions marked. Several suggestions can target the same lines; they are shown side by side, ranked by support.

**Bill-level signal.** At the top of every bill, each resident can mark the bill *Support* or *Oppose*, or leave it unmarked. It is a plain signal, with no editor and no drafting. An optional comment explains the position, and an Oppose can carry one or more reason tags:

- **Too complex:** the bill is hard to follow or tries to do too much at once.
- **Unrelated riders:** the bill carries provisions disconnected from its main purpose. The resident can tick which sections they consider riders.
- **Disagree with the policy:** opposition to the bill's substance.

Residents cannot propose how to split or rewrite the bill; restructuring stays with the legislature. The rider tag gives legislators a countable answer to one of the most common complaints about legislation: complex bills carrying sections that have nothing to do with their subject.

**How support works.**

- One support per resident per suggested edit, and one support or oppose per resident per bill, withdrawable while the bill is in review.
- Residents can support as many items as they like, including competing suggestions on the same lines.
- Suggested edits have no opposition, no majority, and no pass or fail. An item enters the brief once it reaches a small minimum (for example, 25 supporters in the jurisdiction), which keeps one-off ideas from cluttering it.
- Counts are shown as raw numbers and as a share of residents who reviewed the bill, so a count can be read against how many people looked.

**Why suggested edits stay support-only.** It rewards constructive work: the only way to push back on a suggestion is to write a better one, or explain the problem in a line comment. It also removes any reason to brigade a suggestion down. Opposition still has a place, at the level of the whole bill, where it is most meaningful to a legislator.

## Sharing

Anyone, signed in or not, can share a whole bill or any part of it down to a single line, and every shared link opens on exactly that text with the full bill one tap away.

Most people will meet a bill through a friend's post about one clause, not by browsing a legislature's site. Sharing at the level of a line lets that post quote the exact words, and the link brings readers straight into review.

| What is shared | How to share it | What the preview card shows |
| --- | --- | --- |
| Whole bill | Share button at the top of the bill | Bill title and number, sponsor, status, one-line change explanation, current support and opposition |
| Section | Share icon beside each section heading | Section heading and its first lines, the bill title, and the share of opposers tagging it as a rider if notable |
| Line or range of lines | Select text, then Share | The quoted words, with the section citation and bill title |
| Suggested edit | Share button on the suggestion | The side-by-side comparison of bill text and replacement, the reason, and support count |
| Line comment thread | Share button on the thread | The quoted line and the opening comment |

**How a shared link behaves.**

- Each link is pinned to the official version it was shared from. If the legislature later changes that text, the page opens on the shared words with a notice and a one-tap view of the current version, so an old post never misquotes the bill.
- Opening a link scrolls to and highlights the shared text. The full bill, its comments, and its suggestions are all available from there.
- Readers who are not signed in can read everything and are invited to join at the point they try to comment or signal.
- Share targets are the usual social networks and messaging apps, plus copy link and a downloadable image of the quote card for platforms that strip previews.

**What sharing never reveals.** A share card never shows the sharer's own support or opposition unless they choose to add it in their own post. Share counts are not part of the signal to legislators. Referral spikes from shared links are disclosed in the brief, as described under Integrity and trust.

## The signal to legislators

Each legislator receives a short brief per bill. It opens with the bill-level signal, then ranks the most-supported suggested edits. At launch every member receives the same national figures; breakdowns by their own constituents come in a later phase.

A staffer should be able to act on an item in under five minutes, so the brief reads like an amendment memo, not a social feed. The bill-level signal shows support and opposition counts, how much of the opposition cites complexity or riders, and the sections most often tagged as riders. Suggested edits follow, grouped by section.

**What one suggested edit looks like in the brief:**

| Field | Example |
| --- | --- |
| Suggestion | Sec. 4(b): replace "shall" with "may" |
| Comparison | Bill text and suggested replacement side by side, against the named official version |
| Why, in the author's words | Mandatory inspections would close rural clinics with one staff member |
| Support, your district (later phase) | 1,204 residents, 9% of the 13,400 who reviewed this bill |
| Support, whole jurisdiction | 18,930 residents |
| Competing suggestions | 2 others on these lines, with 3,410 and 820 supporters |
| Top concern on this section | The most active line-comment thread, quoted |
| Steward review | Well formed; no cross-reference conflicts |
| Status | Open for your response |

The example numbers are illustrative.

**Delivery.** Briefs go out on a fixed cadence (weekly while a bill is active) and on demand before committee markup and floor votes. They arrive by email and as a page offices can export to the formats their drafting offices already use.

**Responding.** An office marks each item *Adopted* (offered as an amendment), *Under consideration*, or *Declined*, with an optional reason. Responses are public and feed a per-member responsiveness record. The record shows only whether and how an office answered, not whether it agreed, so the system rewards engagement rather than obedience.

**Closing the loop.** When an official amendment matches a citizen suggestion, the system detects the overlap and notifies everyone who supported it, so participants see their input land.

## Onboarding and verification

At launch, anyone located in the US can take part after a social sign-in, and all support and opposition is reported as national totals. The tiers below are how state and district breakdowns can be added later without changing the product.

A strict ID check at sign-up would make the signal credible on day one but stop most people from joining. Instead, the brief shows support split by tier, so legislators can weigh it themselves and the system stays easy to enter.

| Tier | How a person gets there | Can do | How their support appears in the brief |
| --- | --- | --- | --- |
| 1. Signed in | Google or Facebook sign-in from a US location, checked by IP or device location; the only tier at launch | Comment, suggest, support or oppose | Counted in national totals; shown as "unverified" once higher tiers exist |
| 2. Address confirmed | Address matched to the voter roll, or a code mailed on a postcard and entered on the site | Everything in tier 1 | Unlocks state and district breakdowns (later phase) |
| 3. ID verified (optional) | Government ID or a digital ID service | Everything in tier 2; eligible to be a steward | Counted the same as tier 2, and reported as a share of the total for credibility |

A higher tier never adds extra weight to a person's support; it only changes whether that support counts in district totals.

**What social sign-in can and cannot tell us about location.** This is based on general knowledge of these sign-in services, whose permissions change often, so it needs checking before build.

- **Google sign-in:** name, email, and a language or region setting. No home location. Google's contacts data has an address field, but few people fill it in, and access needs Google's approval.
- **Facebook sign-in:** "current city" and "hometown" are available only after Facebook approves the app. Both are typed in by the person, often out of date, and city-level, too coarse to place someone in a district.

Social sign-in is therefore used for easy account creation, not for location.

**Low-friction signals combined at tier 1:**

| Signal | What it gives | Weakness |
| --- | --- | --- |
| IP location | Rough city, invisibly and for free | A VPN defeats it; used only as a consistency check |
| Browser or phone location, asked once with consent | Precise enough to place someone in a district | Can be faked; shows where they are, not where they live |
| Phone number verified by text code | Makes running many accounts costly | Says little about location |
| Entered home address | The district, if true | Self-reported until tier 2 confirms it |

An address that conflicts with the other signals is still accepted but flagged for a tier 2 check before its support can count.

## Integrity and trust

The signal is only worth sending if legislators believe it, so every supporter must be a real, unique resident, and every moderation action must be visible.

| Threat | Defense |
| --- | --- |
| Fake or duplicate accounts | Tiered verification (see Onboarding and verification): only address-confirmed support counts in district totals, and any ID check stores a one-way token, not the document |
| Out-of-jurisdiction brigading | Support counts only from verified residents; district comes from verified address, so per-district numbers are clean |
| Burying a suggestion | Not possible by design: there is no opposing signal, so the only response to a suggestion is a better one or a line comment |
| Organized campaigns inflating one item | Campaigns are allowed but disclosed: sudden spikes and shared-link referrals are flagged in the brief, not hidden or deleted |
| Pressure to support | Who supported what is private from the public and legislators; only totals leave the system, and support can be withdrawn while the bill is in review |
| Bad-faith suggestions | Stewards and moderators can remove abuse; substance is never removed, only labelled |
| Platform operator bias | Open-source code, published moderation log, independent audit of support counts each session |
| Low participation skew | Briefs show support as a share of residents who reviewed the bill, plus demographic coverage where available |

**Privacy.** Comments and suggestions are public under a display name; support and opposition are private. Legislators see counts by district, never who supported what.

**Moderation principles.** Remove only content that breaks published rules (threats, harassment, personal data, spam). Never remove or down-rank an edit for its position. Every removal gets a logged reason and an appeal to an elected community panel.

## Where the idea comes from

The design borrows from how software teams change shared code, a review process millions of developers use every day on text that many people depend on.

In software, the shared working version of a program is the *main branch*. A proposed change is drafted separately on a *branch* and submitted as a *pull request*: a request for the owners to pull the change into the main version. Reviewers see a *diff*, the old and new text side by side. They comment on specific lines, suggest exact replacement wording, and finally approve or request changes. Each revision the author makes is a *commit*, and only the owners decide whether to *merge* the change.

A bill is already a proposed change to shared text, the law, so the process maps closely. One difference is deliberate: citizens review, but only the legislature can change a bill or the law. There is no citizen merge.

| Code concept | In this system | Notes |
| --- | --- | --- |
| Repository | The body of existing law | Statutes, code, or ordinances in their current, consolidated form |
| Main branch | Law as currently in force | Read-only; changes only when the legislature enacts a bill |
| Working branch | A bill | Written by its sponsors; reads like a set of edits and feature additions to the law |
| Pull request | The bill opened for public review | Source is the bill; target is the existing law it touches |
| Commits on the PR | Official versions and amendments | Introduced, committee substitute, engrossed; each new version updates what citizens review |
| PR author | Sponsor and co-sponsors | The only people who can change the bill's text |
| Reviewers | Citizens | Verified residents of the jurisdiction |
| PR diff | Change explanation | Not a literal diff: what the bill would change in existing law, explained and linked to each provision it touches |
| Line comment | Comment on a line of the bill | Questions, concerns, and context, threaded, with no support count |
| Suggested change | Suggested edit | A true code diff: bill text juxtaposed with the citizen's replacement; support or nothing |
| Top-level review (approve or request changes) | Bill-level signal | Support or oppose the whole bill, with an optional comment and reason tags such as "unrelated riders" |
| Outdated comment | Stale suggestion | Flagged when a new official version changes the lines it targets |
| Review summary | Legislator brief | The bill-level signal and the most-supported suggestions, with counts by district |
| Reopened PR | Reintroduced or renamed bill | Linked by stewards into one continuous review |
| Competing PRs for one issue | Competing bills | Different implementations of the same goal, compared side by side and never merged |

## Open questions and next steps

The biggest unknowns are whether legislators will read the brief and whether ordinary residents can write usable legal text; both are best tested with one real legislature before building for many.

**Open questions**

- **Support-only blind spot.** Without opposition, a suggestion backed by 20,000 people could still be opposed by many more. Is the share-of-reviewers figure enough, or should the brief also count line comments that raise concerns about the same lines?
- **Delivery minimum.** Is 25 supporters the right bar for an item to reach the brief, and should it scale with the jurisdiction's size?
- **Rider tags.** How many rider tags should a section need before the brief calls it out, and should a steward first confirm the section is unrelated to the bill's subject?
- **Identity versus participation.** Launch accepts anyone in the US and reports national totals only. What level of participation makes it worth adding address confirmation and district breakdowns, and should unconfirmed support stay in briefs after that?
- **Steward selection.** Will enough offices from both parties lend staff time to make two-party sign-off workable, and how is staff identity verified with each office?
- **Speed.** Low priority for launch, since fast-moving national bills are mostly procedural. Revisit if substantive bills start moving faster than briefs go out.
- **Legislator incentives.** What makes a busy office answer? Options include public responsiveness records, press partnerships, and local media digests.

**Risks**

- A system dominated by highly engaged minorities could be read as "the public" when it is not. Showing turnout and coverage beside every number is the main mitigation. The intent of the system is not to vote. Instead it is a more granular communication tool to existing representatives. Actual support remains as their call.
- If a legislature does not publish versioned, machine-readable bill text and a current consolidated code, finding existing law and writing change explanations becomes manual and slow.

**Suggested next steps**

- [ ] Rebuild the earlier national prototype on Congress's XML bill text and the US Code
- [ ] Recruit two or three legislative offices willing to receive briefs and respond
- [ ] Prototype the side-by-side view and line comments on three real bills
- [ ] Write the support and moderation rules as a public charter before launch
- [ ] Run a closed pilot for one session and measure response rate from offices
