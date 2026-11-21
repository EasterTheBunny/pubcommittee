// Registry of features that are not built in M0. Every "future" button opens a
// pop-up from this list, naming the milestone and the design doc (design/MILESTONES.md).

export interface Placeholder {
	title: string;
	milestone: 'M1' | 'M2' | 'M3' | 'M4' | 'M5' | 'M6';
	doc: string;
	body: string;
}

export const placeholders = {
	'share-social': {
		title: 'Post to social networks',
		milestone: 'M2',
		doc: 'design/06-sharing.md',
		body: 'Opens the network’s share dialog with a preview card of the shared text and a link pinned to this version. If the text changes later, the link opens on these words with a notice.'
	},
	'share-message': {
		title: 'Send in a messaging app',
		milestone: 'M2',
		doc: 'design/06-sharing.md',
		body: 'Hands the link and quote to the device share sheet or a messaging app. Link previews show the same card.'
	},
	'share-image': {
		title: 'Download a quote card image',
		milestone: 'M2',
		doc: 'design/06-sharing.md',
		body: 'Saves the preview card as an image for platforms that strip link previews. Includes the citation and a short link.'
	},
	'share-embed': {
		title: 'Embed on a website',
		milestone: 'M4',
		doc: 'design/06-sharing.md',
		body: 'Gives a snippet that renders the quoted lines with live support counts. Under consideration for a later milestone.'
	},
	'signin-google': {
		title: 'Google sign-in',
		milestone: 'M2',
		doc: 'design/08-onboarding-and-verification.md',
		body: 'Creates your account only. Location comes from a US IP or device check, asked once with consent. A higher tier never adds weight to your support.'
	},
	'signin-facebook': {
		title: 'Facebook sign-in',
		milestone: 'M2',
		doc: 'design/08-onboarding-and-verification.md',
		body: 'Creates your account only. Facebook city data is too coarse for districts, so location comes from IP or device checks.'
	},
	'address-confirm': {
		title: 'Address confirmation',
		milestone: 'M5',
		doc: 'design/08-onboarding-and-verification.md',
		body: 'Match your address to the voter roll, or enter a code mailed on a postcard. Unlocks state and district breakdowns in briefs.'
	},
	ingestion: {
		title: 'Automatic bill ingestion',
		milestone: 'M1',
		doc: 'design/01-bill-ingestion-and-display.md',
		body: 'New bills and versions are pulled from Congress’s published XML, split into addressable lines, and linked to the US Code.'
	},
	'as-amended': {
		title: 'Preview as amended (unofficial)',
		milestone: 'M1',
		doc: 'design/01-bill-ingestion-and-display.md',
		body: 'Applies mechanical “strike and insert” instructions to the current provision and shows the result, always labelled unofficial.'
	},
	'explanation-check': {
		title: 'Check change explanations',
		milestone: 'M2',
		doc: 'design/05-roles-and-stewardship.md',
		body: 'Stewards review automatic explanations, fix errors, and resolve citations the parser could not match. Every correction is signed and public.'
	},
	reanchor: {
		title: 'Re-anchor a stale suggestion',
		milestone: 'M2',
		doc: 'design/02-versions-and-review-lifecycle.md',
		body: 'The author moves the suggestion onto the new version’s text. If its meaning holds, support carries over and supporters are notified so they can withdraw. If not, support resets.'
	},
	'revision-history': {
		title: 'Revision history',
		milestone: 'M2',
		doc: 'design/03-review-features-and-support.md',
		body: 'Every revision of a suggestion is kept and viewable. A revision that changes meaning resets support.'
	},
	'wording-fix': {
		title: 'Offer a wording fix',
		milestone: 'M2',
		doc: 'design/05-roles-and-stewardship.md',
		body: 'A steward proposes cleaner legislative drafting. The author decides whether to accept it. Support is kept on a minor fix.'
	},
	'combine-duplicates': {
		title: 'Combine duplicates',
		milestone: 'M3',
		doc: 'design/05-roles-and-stewardship.md',
		body: 'Links suggestions that make the same change so support is not split. Needs sign-off from stewards of two different parties. Supporters keep their support.'
	},
	appeal: {
		title: 'Ask for a second review',
		milestone: 'M3',
		doc: 'design/05-roles-and-stewardship.md',
		body: 'Any author can ask a steward from another party’s office to review a steward action. Every appeal outcome is public.'
	},
	'steward-queues': {
		title: 'Steward work queues',
		milestone: 'M2',
		doc: 'design/05-roles-and-stewardship.md',
		body: 'Queues for unresolved citations, unreviewed suggestions, stale items, duplicate candidates, rider-heavy sections and pending sign-offs.'
	},
	'lineage-candidates': {
		title: 'Suggested bill links',
		milestone: 'M3',
		doc: 'design/04-bill-lineage-and-competing-bills.md',
		body: 'The system compares new bills by text similarity and by US Code sections amended, and suggests copies, companions or competing bills. A steward confirms each link.'
	},
	'email-delivery': {
		title: 'Email delivery of briefs',
		milestone: 'M3',
		doc: 'design/07-legislator-brief-and-responses.md',
		body: 'Briefs go out weekly while a bill is active, and immediately before committee markup and floor votes.'
	},
	export: {
		title: 'Export brief',
		milestone: 'M3',
		doc: 'design/07-legislator-brief-and-responses.md',
		body: 'Exports the brief in the formats drafting offices already use. Formats will be chosen with pilot offices.'
	},
	'responsiveness-record': {
		title: 'Responsiveness record',
		milestone: 'M3',
		doc: 'design/07-legislator-brief-and-responses.md',
		body: 'A public, per-member record of whether and how an office answered, not whether it agreed.'
	},
	'amendment-matching': {
		title: 'Closing the loop',
		milestone: 'M4',
		doc: 'design/07-legislator-brief-and-responses.md',
		body: 'When a new official version matches a citizen suggestion, everyone who supported it is notified.'
	},
	notifications: {
		title: 'Notifications',
		milestone: 'M2',
		doc: 'design/02-versions-and-review-lifecycle.md',
		body: 'Supporters hear about new versions, re-anchored suggestions, office responses and matching amendments.'
	},
	'moderation-remove': {
		title: 'Remove content',
		milestone: 'M2',
		doc: 'design/09-integrity-moderation-and-privacy.md',
		body: 'Moderators remove only content that breaks published rules: threats, harassment, personal data, spam. Every removal is logged with a reason and can be appealed.'
	},
	'moderation-log': {
		title: 'Public moderation log',
		milestone: 'M2',
		doc: 'design/09-integrity-moderation-and-privacy.md',
		body: 'Every removal and suspension is published with the rule it cites, without republishing the removed content.'
	},
	audit: {
		title: 'Independent audit',
		milestone: 'M4',
		doc: 'design/09-integrity-moderation-and-privacy.md',
		body: 'Support counts are independently audited each session without exposing who supported what.'
	},
	'machine-access': {
		title: 'Data for machines',
		milestone: 'M1',
		doc: 'design/10-machine-access.md',
		body: 'Every page gets a structured form, plus bulk exports, a change feed, and a read-only agent tool interface. Machines can read everything the public can; nothing lets software write on a person’s behalf. User content is always labelled as such. In M0, line ranges already have their own address, e.g. /bills/hr-4127/v/2/s/s4/l/12-13.'
	},
	'count-batching': {
		title: 'Counts update in batches',
		milestone: 'M2',
		doc: 'design/11-ai-security.md',
		body: 'Published counts change at a fixed interval, never the moment someone acts, so a change cannot be tied to one person. Small counts show as “fewer than N”. In M0, counts update instantly so the demo is easy to follow.'
	},
	'automation-disclosure': {
		title: 'Automation disclosure',
		milestone: 'M3',
		doc: 'design/10-machine-access.md',
		body: 'Suspected automated support is disclosed in the brief like a campaign spike: the item, the window, the size, and the share suspected. It is never silently discounted, and detection never looks at a position.'
	},
	'district-breakdown': {
		title: 'District breakdowns',
		milestone: 'M5',
		doc: 'design/07-legislator-brief-and-responses.md',
		body: 'Once addresses are confirmed, each member sees support from their own constituents beside the national figures.'
	}
} satisfies Record<string, Placeholder>;

export type PlaceholderId = keyof typeof placeholders;
