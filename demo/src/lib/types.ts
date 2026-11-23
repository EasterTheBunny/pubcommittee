// Domain types for the M0 proof of concept.
// Names follow the shared vocabulary in design/README.md so M1 can swap
// mock data for real ingestion without reshaping components.

export type Role = 'reader' | 'reviewer' | 'steward' | 'moderator' | 'office';

export interface Persona {
	id: string;
	name: string;
	role: Role;
	/** Office the persona works for (stewards, office staff). */
	office?: string;
	/** Party of the persona's member; used by the two-party and conflict rules. */
	party?: 'A' | 'B';
	blurb: string;
}

export interface Line {
	/** Line number, unique within a version. */
	n: number;
	text: string;
	/** Visual indent level for nested paragraphs. */
	indent?: number;
}

export interface Section {
	/** Stable id within a bill, e.g. "s4". */
	id: string;
	/** Citation label, e.g. "Sec. 4". */
	label: string;
	heading: string;
	lines: Line[];
	topics?: string[];
}

export type Provenance = 'auto' | 'steward-checked' | 'bill-only';

/** Bill-against-law explanation for one section (design/01). */
export interface Explanation {
	sectionId: string;
	citation?: string;
	currentText?: string;
	whatChanges: string;
	whyItMatters: string;
	provenance: Provenance[];
	/** Unofficial preview of the provision as amended, when mechanical. */
	asAmended?: string;
}

export interface BillVersion {
	n: number;
	type: string;
	date: string;
	/** Set when this version is another chamber's text joined into one review (design/04). */
	sourceBill?: string;
	sections: Section[];
	explanations: Explanation[];
}

export interface RiderNote {
	text: string;
	twoPartySignOff: boolean;
}

export interface Bill {
	id: string;
	number: string;
	chamber: 'House' | 'Senate';
	title: string;
	summary: string;
	sponsor: string;
	/** Party of the sponsor; drives the steward conflict rule. */
	sponsorParty: 'A' | 'B';
	status: string;
	versions: BillVersion[];
	issueId?: string;
	riderNotes?: Record<string, RiderNote>;
}

/** A stable reference to lines in one official version (design/01). */
export interface Anchor {
	billId: string;
	version: number;
	sectionId: string;
	from: number;
	to: number;
}

/** A public line comment. Never carries the author's district or tier (design/08). */
export interface Comment {
	id: string;
	/** Display name (a pseudonym by default). */
	author: string;
	body: string;
	at: string;
	stewardContext?: boolean;
	office?: string;
	removed?: boolean;
}

export interface Thread {
	id: string;
	anchor: Anchor;
	comments: Comment[];
}

export interface SuggestedEdit {
	id: string;
	anchor: Anchor;
	title: string;
	/** Replacement for the anchored lines, joined with spaces. Empty means delete. */
	replacement: string;
	reason: string;
	author: string;
	/** Seeded supporter count, before any demo persona supports it. */
	seedSupport: number;
	wellFormed: boolean;
	revisions: number;
	/** Integrity disclosure shown in the brief (design/09). */
	integrityNote?: string;
}

export type SignalValue = 'support' | 'oppose';
export type OpposeReason = 'complex' | 'riders' | 'policy';

export interface BillSignal {
	value: SignalValue;
	reasons: OpposeReason[];
	riderSections: string[];
	/** Private to the author; never shown with their name (design/03). */
	comment: string;
}

export interface BillSignalSeed {
	support: number;
	oppose: number;
	reviewers: number;
	reasons: Record<OpposeReason, number>;
	riderTags: Record<string, number>;
}

export type ResponseStatus = 'adopted' | 'consider' | 'declined';

export interface OfficeResponse {
	status: ResponseStatus;
	reason: string;
	office: string;
}

export interface IssueRow {
	label: string;
	values: Record<string, string>;
}

export interface LineageLink {
	from: string;
	to: string;
	kind: 'companion' | 'reintroduction';
	note: string;
}

export interface Issue {
	id: string;
	title: string;
	tags: string[];
	summary: string;
	comparison: string;
	twoPartySignOff: boolean;
	billIds: string[];
	rows: IssueRow[];
	links: LineageLink[];
}

export type ShareTarget =
	| { kind: 'bill'; billId: string; version: number }
	| { kind: 'section'; billId: string; version: number; sectionId: string }
	| { kind: 'lines'; anchor: Anchor }
	| { kind: 'suggestion'; suggestionId: string }
	| { kind: 'thread'; threadId: string };

// --- Bill selection and the public catalog (design/01) ---------------------

export type MarkupStatus = 'in_review' | 'eligible' | 'not_candidate';
export type Topic = 'healthcare' | 'deficit' | 'affordability';

export interface SelectionPolicy {
	version: number;
	mode: 'pilot' | 'general';
	/** early_pilot: administrators may select bills alone; afterwards two-party sign-off. */
	phase: 'early_pilot' | 'steward_signoff';
	topics: Topic[];
}

export interface MarkupDecision {
	outcome: 'accepted' | 'declined';
	reason: string;
	date: string;
}

export interface CatalogEntry {
	id: string;
	number: string;
	chamber: 'House' | 'Senate';
	title: string;
	sponsor: string;
	sponsorParty: 'A' | 'B';
	latestAction: string;
	/** Committee activity beyond referral; empty when only referred. */
	committeeActivity: string[];
	subjects: string[];
	topic?: Topic;
	markupStatus: MarkupStatus;
	/** Why an excluded measure kind is not a markup candidate. */
	exclusionReason?: string;
	/** Set when tracked: the review page's bill id and why it was selected. */
	tracked?: { billId: string; policy: number; reasons: string[]; byAdministrator?: boolean };
	/** Official text versions on the legislature's site (labels only in the mock). */
	officialVersions: string[];
	/** Seeded markup-request total, before demo personas act. */
	seedRequests: number;
	decision?: MarkupDecision;
}

// --- Notifications (design/08) ---------------------------------------------

export type FeedEventKind = 'new_version' | 'stale' | 'revised' | 'decision' | 'response';

/** A public change-feed event (design/10). Notifications join these with a person's own items. */
export interface FeedEvent {
	id: string;
	date: string;
	kind: FeedEventKind;
	/** What the event is about, for matching against the person's own items. */
	target: { kind: 'bill' | 'suggestion' | 'request'; id: string; billId: string };
	text: string;
	href: string;
}
