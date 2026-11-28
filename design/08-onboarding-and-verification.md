# 08 — Onboarding and Verification

## Purpose

Keep entry easy while making the signal credible over time. At launch anyone located in the US can take part after a social sign-in, and everything is reported as national totals. Verification tiers let state and district breakdowns be added later without changing the product.

## Scope

**In scope**
- Account creation and sign-in
- Verification tiers and how a person moves between them
- Low-friction location and uniqueness signals at tier 1
- Assigning jurisdiction and (later) district
- Official and steward identity at a high level

**Out of scope**
- How tiers appear in briefs (see [07](07-legislator-brief-and-responses.md))
- Fraud defenses beyond verification (see [09](09-integrity-moderation-and-privacy.md))

## Tiers

| Tier | How reached | Can do | How support appears |
| --- | --- | --- | --- |
| 1. Signed in | Social sign-in from a US location (checked by IP or device location). Only tier at launch | Comment, suggest, support, oppose | Counted in national totals; labelled "unverified" once higher tiers exist |
| 2. Address confirmed | Address matched to the voter roll, or a mailed code entered on the site | Everything in tier 1 | Unlocks state and district breakdowns (later phase) |
| 3. ID verified (optional) | Government ID or a digital ID service | Everything in tier 2; eligible to be a steward | Same weight as tier 2; reported as a share of total for credibility |

**A higher tier never adds weight.** It only changes whether support counts in district totals.

## Social sign-in

At launch: Google and Facebook sign-in, used for **account creation only, not location**.

What they can tell us (needs re-checking before build; permissions change often):
- **Google:** name, email, language/region setting. No home location. Contact-address access is rare and needs provider approval.
- **Facebook:** "current city" and "hometown" only after app review; self-entered, often stale, and city-level — too coarse for districts.

## Tier 1 signals

| Signal | Gives | Weakness | Use |
| --- | --- | --- | --- |
| IP location | Rough city, free and invisible | VPN defeats it | Consistency check only |
| Browser/phone location, asked once with consent | Precise enough for a district | Fakeable; where they are, not where they live | Consistency check; optional |
| Phone number verified by text code | Raises cost of many accounts | Little location value | Uniqueness |
| Entered home address | District, if true | Self-reported until tier 2 | Provisional district |

- An address that conflicts with other signals is accepted but **flagged** for a tier 2 check before its support can count in district totals.
- Location consent is asked once and can be revoked.

## Eligibility

- **Launch:** US location at sign-in → eligible reviewer for US Congress bills.
- **Later:** jurisdiction and district come from a confirmed (tier 2) address. Reviewers may signal only on bills in their jurisdiction. National bills remain open to every US resident; state and local bills are limited to residents of that jurisdiction.
- Each signal records the reviewer's tier and (later) district **at the time of signalling**, so later tier changes are applied consistently.

## Official and steward identity

- **Legislator and office staff accounts:** verified through the office, not through the citizen tiers.
- **Stewards:** must be tier 3 and confirmed as staff of a sitting member's office ([05](05-roles-and-stewardship.md)).

## Installable app

The web app must be an installable app: people can add it to a phone's home screen or a desktop from the browser, with no app store. Installed, it opens like a native app, starts without network for its shell, and keeps official bill text the person has already opened readable offline (official versions never change, so cached copies stay correct). Commenting, suggesting, and signalling always need a connection. On some phones, device notifications only work once the app is installed.

## Notifications

People hear about changes to things they took part in: a suggestion they supported was revised or went stale, a new official version arrived, a markup request was decided, an office responded, or a suggestion appeared in the official text ([02](02-versions-and-review-lifecycle.md), [03](03-review-features-and-support.md), [07](07-legislator-brief-and-responses.md)).

### Channels
| Channel | How | Default |
| --- | --- | --- |
| In-app inbox | A list of updates shown when the person is signed in | Always on |
| Device notifications | Browser or installed-app notifications, turned on per device | Off; opt-in per device |
| Email | Not offered for now | — |

### Rules
- **Opt-in, per device, revocable.** Turning notifications on is a choice made on each device. Turning it off, signing out, or the browser revoking permission removes that device's subscription.
- **Content-free messages.** A device notification says only that updates are waiting, with a link. What changed is shown after the person opens the app while signed in. A notification never names a bill, item, or position, so a lock screen or a shared device reveals nothing.
- **No list of who took part.** The system does not keep a record of which people supported, opposed, or requested which items in order to notify them. It works out each person's updates from their own private record of their signals, at the time it notifies them ([09](09-integrity-moderation-and-privacy.md)).
- **Batched.** Notifications go out on a schedule, not the moment something happens, matching how counts are published ([11](11-ai-security.md)).
- **No contact details.** Device notifications need no email address or phone number. A device subscription identifies the device to the browser's notification service, not the person.

## Domain concepts

- **Account** — display name, linked sign-in identities, notification preference, device subscriptions, current tier.
- **Verification record** — method, result, timestamp, expiry. For ID checks, only a one-way token is kept, never the document ([09](09-integrity-moderation-and-privacy.md)).
- **Location evidence** — IP-derived region, device location (if consented), entered address, conflict flags.
- **Residency** — jurisdiction(s) and district(s) derived from a confirmed address; effective dates (people move).

## Rules and invariants

- Raw ID documents are never stored.
- Precise location and home addresses are private and never shown publicly or to legislators.
- District and tier are **never shown on public content** (comments, suggestions, profiles). They are used only in aggregates ([11](11-ai-security.md)).
- Display names are pseudonyms by default; people are not asked to use their real name.
- Notifications never require an email address, and never name a bill, item, or position in the message itself.
- Tier changes never retroactively add weight; they may change which totals a signal is included in.
- A person holds at most one reviewer account (enforced as far as signals allow).

## Dependencies

- [03](03-review-features-and-support.md) eligibility checks on every signal.
- [05](05-roles-and-stewardship.md) steward and official eligibility.
- [07](07-legislator-brief-and-responses.md) tier and district splits.
- [09](09-integrity-moderation-and-privacy.md) uniqueness and fraud signals.
- [11](11-ai-security.md) re-identification threats.
- External: social sign-in providers; later, voter-roll matching, postal mail, ID verification services.

## Open questions

- What level of participation justifies adding address confirmation and district breakdowns?
- After district breakdowns exist, should unconfirmed (tier 1) support remain in briefs at all?
- How is residency updated when someone moves, and what happens to their existing signals?
- How are non-citizen US residents treated? The doc says "located in the US" and "resident"; eligibility needs a firm definition.
- Is phone verification required at tier 1, or optional?
- Will other sign-in options be needed for people without Google or Facebook accounts?
- Should reviewers be offered a separate pseudonym per bill, to resist cross-bill profiling ([11](11-ai-security.md))?
  - **Decided (operator, 2026-11-28):** yes, for everyone: pages and exports show a per-bill pseudonym, never one name across bills.
- Is email worth adding later for people who never install the app or allow device notifications, given that it requires collecting and protecting an address?
- How long should offline copies of official text be kept on a device?
