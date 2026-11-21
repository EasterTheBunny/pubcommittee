import type { CatalogEntry, SelectionPolicy, Topic } from '#lib/types.ts';

// Mock catalog (design/01 Bill selection, Public catalog). Numbers, titles and members are
// invented; none of these entries is a real bill.

export const selectionPolicy: SelectionPolicy = {
	version: 1,
	mode: 'pilot',
	phase: 'early_pilot',
	topics: ['healthcare', 'deficit', 'affordability']
};

export const topicLabel: Record<Topic, string> = {
	healthcare: 'Healthcare',
	deficit: 'Deficit reduction',
	affordability: 'Affordability'
};

/** Requests needed before a bill enters the steward queue (design/01 open question). */
export const REQUEST_THRESHOLD = 25;

export const catalog: CatalogEntry[] = [
	{
		id: 'hr-4127',
		number: 'H.R. 4127',
		chamber: 'House',
		title: 'Rural Clinic Inspection Modernization Act',
		sponsor: 'Rep. [Member A] (mock)',
		sponsorParty: 'A',
		latestAction: 'Reported by committee',
		committeeActivity: ['Hearing', 'Markup', 'Reported'],
		subjects: ['Health', 'Rural health'],
		topic: 'healthcare',
		markupStatus: 'in_review',
		tracked: {
			billId: 'hr-4127',
			policy: 1,
			reasons: ['Reported by committee', 'Topic: healthcare']
		},
		officialVersions: ['Introduced', 'Reported in House'],
		seedRequests: 0
	},
	{
		id: 's-2210',
		number: 'S. 2210',
		chamber: 'Senate',
		title: 'Risk-Based Clinic Oversight Act',
		sponsor: 'Sen. [Member B] (mock)',
		sponsorParty: 'B',
		latestAction: 'Hearing held',
		committeeActivity: ['Hearing'],
		subjects: ['Health', 'Health facilities'],
		topic: 'healthcare',
		markupStatus: 'in_review',
		tracked: {
			billId: 's-2210',
			policy: 1,
			reasons: ['Committee hearing held', 'Topic: healthcare']
		},
		officialVersions: ['Introduced'],
		seedRequests: 0
	},
	{
		id: 'hr-3981',
		number: 'H.R. 3981',
		chamber: 'House',
		title: 'Clinic Access Preservation Act',
		sponsor: 'Rep. [Member C] (mock)',
		sponsorParty: 'B',
		latestAction: 'Referred to committee',
		committeeActivity: [],
		subjects: ['Health'],
		topic: 'healthcare',
		markupStatus: 'in_review',
		tracked: {
			billId: 'hr-3981',
			policy: 1,
			reasons: ['Selected by an administrator: pilot seed, competing bill on an issue in review'],
			byAdministrator: true
		},
		officialVersions: ['Introduced'],
		seedRequests: 0
	},
	{
		id: 'hr-5502',
		number: 'H.R. 5502',
		chamber: 'House',
		title: 'Prescription Price Transparency Act (mock)',
		sponsor: 'Rep. [Member D] (mock)',
		sponsorParty: 'B',
		latestAction: 'Referred to committee',
		committeeActivity: [],
		subjects: ['Health', 'Prescription drugs'],
		topic: 'healthcare',
		markupStatus: 'eligible',
		officialVersions: ['Introduced'],
		seedRequests: 31
	},
	{
		id: 's-1876',
		number: 'S. 1876',
		chamber: 'Senate',
		title: 'Fiscal Commission Act (mock)',
		sponsor: 'Sen. [Member E] (mock)',
		sponsorParty: 'A',
		latestAction: 'Referred to committee',
		committeeActivity: [],
		subjects: ['Economics and public finance', 'Budget deficits and national debt'],
		topic: 'deficit',
		markupStatus: 'eligible',
		officialVersions: ['Introduced'],
		seedRequests: 6
	},
	{
		id: 'hr-6120',
		number: 'H.R. 6120',
		chamber: 'House',
		title: 'Starter Home Supply Act (mock)',
		sponsor: 'Rep. [Member F] (mock)',
		sponsorParty: 'A',
		latestAction: 'Markup held',
		committeeActivity: ['Markup'],
		subjects: ['Housing and community development'],
		topic: 'affordability',
		markupStatus: 'eligible',
		officialVersions: ['Introduced'],
		seedRequests: 14
	},
	{
		id: 's-990',
		number: 'S. 990',
		chamber: 'Senate',
		title: 'Rural Broadband Permitting Act (mock)',
		sponsor: 'Sen. [Member G] (mock)',
		sponsorParty: 'B',
		latestAction: 'Reported by committee',
		committeeActivity: ['Reported'],
		subjects: ['Science, technology, communications'],
		markupStatus: 'eligible',
		officialVersions: ['Introduced', 'Reported in Senate'],
		seedRequests: 40,
		decision: {
			outcome: 'declined',
			reason:
				'Outside the pilot topics (selection policy v1). Can be requested again if the scope widens.',
			date: '2026-11-14'
		}
	},
	{
		id: 'hjres-140',
		number: 'H.J.Res. 140',
		chamber: 'House',
		title: 'Directing the removal of United States Armed Forces from hostilities (mock)',
		sponsor: 'Rep. [Member H] (mock)',
		sponsorParty: 'A',
		latestAction: 'Referred to committee',
		committeeActivity: [],
		subjects: ['Armed forces and national security'],
		markupStatus: 'not_candidate',
		exclusionReason: 'Privileged war powers resolution: a yes-or-no question rather than drafting.',
		officialVersions: ['Introduced'],
		seedRequests: 0
	},
	{
		id: 'hres-812',
		number: 'H.Res. 812',
		chamber: 'House',
		title: 'Recognizing National Rural Health Week (mock)',
		sponsor: 'Rep. [Member I] (mock)',
		sponsorParty: 'B',
		latestAction: 'Referred to committee',
		committeeActivity: [],
		subjects: ['Health', 'Commemorations'],
		markupStatus: 'not_candidate',
		exclusionReason: 'Commemorative simple resolution: no law to draft.',
		officialVersions: ['Introduced'],
		seedRequests: 0
	}
];

export const catalogEntry = (id: string) => catalog.find((c) => c.id === id);
export const catalogForBill = (billId: string) => catalog.find((c) => c.tracked?.billId === billId);

/** Anonymous reasons stewards see in the queue (never published, no names). */
export const seedRequestReasons: Record<string, string[]> = {
	'hr-5502': [
		'Section 3 defines "list price" in a way that leaves out rebates.',
		'Our pharmacy would need clearer reporting deadlines.'
	],
	's-990': ['Permitting timelines matter for our county.']
};
