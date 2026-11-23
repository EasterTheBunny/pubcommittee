import type { BillSignalSeed, SuggestedEdit, Thread } from '#lib/types.ts';

// Seeded review activity. All numbers are illustrative.

export const seedThreads: Thread[] = [
	{
		id: 't1',
		anchor: { billId: 'hr-4127', version: 2, sectionId: 's4', from: 12, to: 13 },
		comments: [
			{
				id: 'c1',
				author: 'marisol.k',
				body: 'Our county clinic has one nurse practitioner. A yearly on-site visit means closing for a day. Could remote review count for low-risk clinics?',
				at: '2026-11-17'
			},
			{
				id: 'c2',
				author: '[Steward B]',
				office: 'Office of Sen. [Member B]',
				stewardContext: true,
				body: 'Current law sets a 36-month interval (mock citation, para. 2). The committee report estimates 4,100 covered clinics (mock figure).',
				at: '2026-11-18'
			},
			{
				id: 'c3',
				author: 'jt_ruralmed',
				body: 'Same here. We would need to schedule a locum just for inspection day.',
				at: '2026-11-18'
			}
		]
	},
	{
		id: 't2',
		anchor: { billId: 'hr-4127', version: 2, sectionId: 's9', from: 23, to: 24 },
		comments: [
			{
				id: 'c4',
				author: 'p.okafor',
				body: 'Why is a coin program in a clinic inspection bill?',
				at: '2026-11-12'
			}
		]
	},
	{
		id: 't3',
		anchor: { billId: 's-2210', version: 1, sectionId: 's2', from: 2, to: 3 },
		comments: [
			{
				id: 'c5',
				author: 'hollis.w',
				body: 'Who decides the risk rating? The bill does not say how patient volume is weighed.',
				at: '2026-11-10'
			}
		]
	}
];

export const seedSuggestions: SuggestedEdit[] = [
	{
		id: 'e1',
		anchor: { billId: 'hr-4127', version: 2, sectionId: 's4', from: 12, to: 13 },
		title: 'Replace “shall” with “may”',
		replacement:
			'“(b) INSPECTIONS.—The Secretary may conduct an on-site inspection of each covered rural health clinic not less than once every 12 months.”',
		reason: 'Mandatory inspections would close rural clinics with one staff member.',
		author: 'marisol.k',
		seedSupport: 18930,
		wellFormed: true,
		revisions: 1,
		integrityNote:
			'Support spiked on Nov 15 (+6,200 in 24 h). 58% arrived via shared links. Disclosed, not discounted.'
	},
	{
		id: 'e2',
		anchor: { billId: 'hr-4127', version: 2, sectionId: 's4', from: 13, to: 13 },
		title: 'Allow remote review for low-risk clinics',
		replacement:
			'covered rural health clinic not less than once every 12 months, except that the Secretary may conduct a remote review of a clinic with no deficiencies in its prior inspection.”',
		reason: 'Keeps oversight without forcing a closure day for clinics with a clean record.',
		author: 'jt_ruralmed',
		seedSupport: 3410,
		wellFormed: false,
		revisions: 0
	},
	{
		id: 'e3',
		anchor: { billId: 'hr-4127', version: 1, sectionId: 's4', from: 12, to: 13 },
		title: 'Keep the 36-month interval',
		replacement:
			'“(b) INSPECTIONS.—The Secretary shall conduct an inspection of each covered rural health clinic not less than once every 36 months.”',
		reason:
			'The current interval works. More frequent inspections cost money that clinics do not have.',
		author: 'r.delgado',
		seedSupport: 820,
		wellFormed: false,
		revisions: 0
	},
	{
		id: 'e4',
		anchor: { billId: 'hr-4127', version: 2, sectionId: 's5', from: 15, to: 15 },
		title: 'Publish the report online',
		replacement:
			'The Secretary shall report to Congress, and publish online, each year on the number of covered clinics inspected',
		reason: 'Residents should be able to see the results without a records request.',
		author: 'dana.r',
		seedSupport: 12,
		wellFormed: false,
		revisions: 0
	},
	{
		id: 'e5',
		anchor: { billId: 's-2210', version: 1, sectionId: 's2', from: 7, to: 7 },
		title: 'Cap the low-risk interval at 36 months',
		replacement: '(3) 36 months, if rated low risk.',
		reason: 'Going longer than today’s interval for any clinic is a step backwards.',
		author: 'hollis.w',
		seedSupport: 2140,
		wellFormed: true,
		revisions: 0
	}
];

export const seedSignals: Record<string, BillSignalSeed> = {
	'hr-4127': {
		support: 12480,
		oppose: 9210,
		reviewers: 40150,
		reasons: { policy: 4789, riders: 3776, complex: 1658 },
		riderTags: { s9: 3776, s6: 410 }
	},
	's-2210': {
		support: 15020,
		oppose: 4890,
		reviewers: 34200,
		reasons: { policy: 3100, riders: 0, complex: 880 },
		riderTags: {}
	},
	'hr-3981': {
		support: 6310,
		oppose: 7750,
		reviewers: 22100,
		reasons: { policy: 6200, riders: 0, complex: 310 },
		riderTags: {}
	}
};

/** Minimum supporters for a suggestion to enter the brief (design/03). */
export const BRIEF_MINIMUM = 25;

/**
 * Counts below this are shown as "fewer than N", so a small count cannot identify
 * the author or a known supporter (design/11). The real threshold is an open question.
 */
export const SMALL_COUNT = 10;
