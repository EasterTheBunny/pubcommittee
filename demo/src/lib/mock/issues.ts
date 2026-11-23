import type { Issue } from '#lib/types.ts';

export const issues: Issue[] = [
	{
		id: 'rural-clinic-inspections',
		title: 'Rural clinic inspections',
		tags: ['Health care', 'Rural'],
		summary:
			'Three bills address how often rural health clinics are inspected. They are separate bills with separate reviews. They are never merged.',
		comparison:
			'H.R. 4127 sets a fixed yearly on-site inspection. S. 2210 sets the interval by each clinic’s risk rating. H.R. 3981 keeps the current 36-month interval and allows remote reviews. Only H.R. 4127 contains a section outside the issue (a commemorative coin program).',
		twoPartySignOff: true,
		billIds: ['hr-4127', 's-2210', 'hr-3981'],
		rows: [
			{
				label: 'Inspection interval',
				values: {
					'hr-4127': 'Every 12 months',
					's-2210': '12–48 months by risk rating',
					'hr-3981': 'Every 36 months (unchanged)'
				}
			},
			{
				label: 'On-site required',
				values: {
					'hr-4127': 'Yes',
					's-2210': 'High-risk clinics only',
					'hr-3981': 'No, remote allowed'
				}
			},
			{
				label: 'Inspector training',
				values: {
					'hr-4127': 'New grant program',
					's-2210': 'Not addressed',
					'hr-3981': 'Not addressed'
				}
			},
			{
				label: 'Sections outside the issue',
				values: { 'hr-4127': '1 (Sec. 9)', 's-2210': 'None', 'hr-3981': 'None' }
			}
		],
		links: [
			{
				from: 'S. 2210',
				to: 'H.R. 5530',
				kind: 'companion',
				note: 'House companion, joined into one review. Each chamber’s text is shown as a version.'
			}
		]
	}
];
