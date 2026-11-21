import type { Bill, Explanation, Section } from '#lib/types.ts';

// Mock bills. Titles, sponsors, citations and numbers are invented for the demo.

type RawLine = string | [text: string, indent: number];
type RawSection = Omit<Section, 'lines'> & { lines: RawLine[] };

/** Assigns version-wide line numbers in order, so anchors stay stable per version. */
function numbered(sections: RawSection[]): Section[] {
	let n = 0;
	return sections.map((s) => ({
		...s,
		lines: s.lines.map((l) => {
			n += 1;
			return typeof l === 'string' ? { n, text: l } : { n, text: l[0], indent: l[1] };
		})
	}));
}

// ---------------------------------------------------------------------------
// H.R. 4127: amendatory bill, two versions, one obvious rider (Sec. 9).
// ---------------------------------------------------------------------------

const hr4127Sections = (interval: string, onSite: boolean): RawSection[] => [
	{
		id: 's1',
		label: 'Sec. 1',
		heading: 'SHORT TITLE.',
		lines: ['This Act may be cited as the “Rural Clinic Inspection Modernization Act”.']
	},
	{
		id: 's2',
		label: 'Sec. 2',
		heading: 'DEFINITIONS.',
		lines: [
			'In this Act:',
			['(1) COVERED CLINIC.—The term “covered clinic” means a rural health clinic', 1],
			['certified under title XVIII of the Social Security Act (mock citation).', 1]
		],
		topics: ['Health care']
	},
	{
		id: 's3',
		label: 'Sec. 3',
		heading: 'FINDINGS.',
		lines: [
			'Congress finds the following:',
			['(1) Rural health clinics serve as the only source of primary care in many counties.', 1],
			['(2) Inspection intervals for covered clinics have not been updated in decades.', 1],
			['(3) Many covered clinics operate with a single clinician.', 1]
		]
	},
	{
		id: 's4',
		label: 'Sec. 4',
		heading: 'INSPECTION OF RURAL HEALTH CLINICS.',
		lines: [
			'(a) IN GENERAL.—Section 1861(aa) of the Social Security Act (mock citation) is amended—',
			[`(1) in paragraph (2), by striking “36 months” and inserting “${interval}”; and`, 1],
			['(2) by adding at the end the following:', 1],
			[
				`“(b) INSPECTIONS.—The Secretary shall conduct ${onSite ? 'an on-site' : 'an'} inspection of each`,
				2
			],
			[`covered rural health clinic not less than once every ${interval}.”`, 2],
			'(b) NOTICE.—The Secretary shall give each covered clinic not less than 30 days notice of an inspection.'
		],
		topics: ['Health care', 'Rural']
	},
	{
		id: 's5',
		label: 'Sec. 5',
		heading: 'REPORTING.',
		lines: [
			'The Secretary shall report to Congress each year on the number of covered clinics inspected',
			'and the number found to have deficiencies.'
		]
	},
	{
		id: 's6',
		label: 'Sec. 6',
		heading: 'INSPECTOR TRAINING GRANTS.',
		lines: [
			'(a) GRANTS.—The Secretary shall award grants to States to train inspectors of covered clinics.',
			'(b) PRIORITY.—In awarding grants, the Secretary shall give priority to States with the',
			'largest share of covered clinics.'
		]
	},
	{
		id: 's7',
		label: 'Sec. 7',
		heading: 'AUTHORIZATION OF APPROPRIATIONS.',
		lines: [
			'There is authorized to be appropriated to carry out section 6 $40,000,000 for each',
			'of fiscal years 2027 through 2031.'
		]
	},
	{
		id: 's8',
		label: 'Sec. 8',
		heading: 'EFFECTIVE DATE.',
		lines: ['The amendments made by section 4 take effect 1 year after the date of enactment.']
	},
	{
		id: 's9',
		label: 'Sec. 9',
		heading: 'COMMEMORATIVE COIN PROGRAM.',
		lines: [
			'(a) IN GENERAL.—The Secretary of the Treasury shall mint and issue not more than 50,000',
			'$1 coins commemorating the 250th anniversary of the founding of the Coast Guard Auxiliary.',
			'(b) DESIGN.—The design of the coins shall be emblematic of the Auxiliary (mock).'
		],
		topics: ['Treasury']
	}
];

const hr4127Explanations = (interval: string, onSite: boolean): Explanation[] => [
	{
		sectionId: 's4',
		citation: '42 U.S.C. 1395x(aa)(2) (mock citation)',
		currentText:
			'…shall be inspected not less than once every 36 months by the Secretary or a designated agency.',
		whatChanges: `Shortens the inspection interval from 36 months to ${interval}${onSite ? ', and requires the inspection to be on site' : ''}.`,
		whyItMatters: onSite
			? 'Roughly triples inspection frequency. Clinics with very small staff may need to pause service during visits.'
			: 'Increases inspection frequency by half. Inspections may still be done remotely.',
		provenance: onSite ? ['auto', 'steward-checked'] : ['auto'],
		asAmended: `…shall be inspected not less than once every ${interval} by the Secretary or a designated agency.`
	},
	{
		sectionId: 's6',
		whatChanges: 'Creates a new grant program for States to train clinic inspectors.',
		whyItMatters: 'New spending authority. Funding is set in Sec. 7.',
		provenance: ['bill-only']
	},
	{
		sectionId: 's9',
		citation: '31 U.S.C. 5112 (mock citation)',
		whatChanges: 'Adds a new commemorative coin to the list of coins the Treasury may mint.',
		whyItMatters: 'Unrelated to clinic inspections. Has no effect on health law.',
		provenance: ['auto', 'steward-checked']
	}
];

// ---------------------------------------------------------------------------
// S. 2210: competing bill, joined with its House companion H.R. 5530.
// ---------------------------------------------------------------------------

const s2210Sections = (chamberNote: string): RawSection[] => [
	{
		id: 's1',
		label: 'Sec. 1',
		heading: 'SHORT TITLE.',
		lines: [`This Act may be cited as the “Risk-Based Clinic Oversight Act”.${chamberNote}`]
	},
	{
		id: 's2',
		label: 'Sec. 2',
		heading: 'RISK-BASED INSPECTION INTERVALS.',
		lines: [
			'(a) RISK RATING.—The Secretary shall assign each covered clinic a risk rating of low,',
			'moderate, or high, based on its inspection history and patient volume.',
			'(b) INTERVALS.—A covered clinic shall be inspected not less than once every—',
			['(1) 12 months, if rated high risk;', 1],
			['(2) 24 months, if rated moderate risk; and', 1],
			['(3) 48 months, if rated low risk.', 1],
			'(c) ON-SITE.—Inspections of high-risk clinics shall be on site.'
		],
		topics: ['Health care', 'Rural']
	},
	{
		id: 's3',
		label: 'Sec. 3',
		heading: 'REVIEW OF RATINGS.',
		lines: ['A covered clinic may request review of its risk rating once each year.']
	}
];

const s2210Explanations: Explanation[] = [
	{
		sectionId: 's2',
		citation: '42 U.S.C. 1395x(aa)(2) (mock citation)',
		currentText:
			'…shall be inspected not less than once every 36 months by the Secretary or a designated agency.',
		whatChanges:
			'Replaces the single 36-month interval with 12, 24 or 48 months depending on a new risk rating.',
		whyItMatters:
			'Low-risk clinics would be inspected less often than today; high-risk clinics more often.',
		provenance: ['auto', 'steward-checked']
	}
];

// ---------------------------------------------------------------------------
// H.R. 3981: competing bill that keeps the interval and allows remote review.
// ---------------------------------------------------------------------------

const hr3981Sections: RawSection[] = [
	{
		id: 's1',
		label: 'Sec. 1',
		heading: 'SHORT TITLE.',
		lines: ['This Act may be cited as the “Clinic Access Preservation Act”.']
	},
	{
		id: 's2',
		label: 'Sec. 2',
		heading: 'REMOTE INSPECTIONS.',
		lines: [
			'Section 1861(aa)(2) of the Social Security Act (mock citation) is amended by adding at the end',
			'the following: “An inspection under this paragraph may be conducted remotely.”'
		],
		topics: ['Health care', 'Rural']
	}
];

const hr3981Explanations: Explanation[] = [
	{
		sectionId: 's2',
		citation: '42 U.S.C. 1395x(aa)(2) (mock citation)',
		currentText:
			'…shall be inspected not less than once every 36 months by the Secretary or a designated agency.',
		whatChanges: 'Keeps the 36-month interval and allows any inspection to be done remotely.',
		whyItMatters:
			'No clinic would need to close for an on-site visit. Inspectors would not see facilities in person.',
		provenance: ['auto'],
		asAmended:
			'…shall be inspected not less than once every 36 months by the Secretary or a designated agency. An inspection under this paragraph may be conducted remotely.'
	}
];

export const bills: Bill[] = [
	{
		id: 'hr-4127',
		number: 'H.R. 4127',
		chamber: 'House',
		title: 'Rural Clinic Inspection Modernization Act',
		summary:
			'Requires yearly on-site inspections of rural health clinics, funds inspector training, and creates a commemorative coin program.',
		sponsor: 'Rep. [Member A] (mock)',
		sponsorParty: 'A',
		status: 'Reported by committee',
		issueId: 'rural-clinic-inspections',
		riderNotes: {
			s9: {
				text: 'This section directs the Treasury to mint a commemorative coin. It does not refer to clinics or inspections. Rider tags are never removed.',
				twoPartySignOff: true
			}
		},
		versions: [
			{
				n: 1,
				type: 'Introduced',
				date: '2026-09-08',
				sections: numbered(hr4127Sections('24 months', false)),
				explanations: hr4127Explanations('24 months', false)
			},
			{
				n: 2,
				type: 'Reported in House',
				date: '2026-10-30',
				sections: numbered(hr4127Sections('12 months', true)),
				explanations: hr4127Explanations('12 months', true)
			}
		]
	},
	{
		id: 's-2210',
		number: 'S. 2210',
		chamber: 'Senate',
		title: 'Risk-Based Clinic Oversight Act',
		summary:
			'Sets clinic inspection intervals by risk rating, from 12 to 48 months, with on-site visits for high-risk clinics.',
		sponsor: 'Sen. [Member B] (mock)',
		sponsorParty: 'B',
		status: 'In committee',
		issueId: 'rural-clinic-inspections',
		versions: [
			{
				n: 1,
				type: 'Introduced in Senate',
				date: '2026-08-21',
				sections: numbered(s2210Sections('')),
				explanations: s2210Explanations
			},
			{
				n: 2,
				type: 'House companion text',
				date: '2026-09-15',
				sourceBill: 'H.R. 5530',
				sections: numbered(s2210Sections(' (House companion)')),
				explanations: s2210Explanations
			}
		]
	},
	{
		id: 'hr-3981',
		number: 'H.R. 3981',
		chamber: 'House',
		title: 'Clinic Access Preservation Act',
		summary:
			'Keeps the 36-month clinic inspection interval and allows inspections to be done remotely.',
		sponsor: 'Rep. [Member D] (mock)',
		sponsorParty: 'B',
		status: 'In committee',
		issueId: 'rural-clinic-inspections',
		versions: [
			{
				n: 1,
				type: 'Introduced',
				date: '2026-07-30',
				sections: numbered(hr3981Sections),
				explanations: hr3981Explanations
			}
		]
	}
];
