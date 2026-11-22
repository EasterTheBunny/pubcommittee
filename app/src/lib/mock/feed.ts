import type { FeedEvent } from '#lib/types.ts';

// Mock public change feed (design/10). In M2, job-notify joins these with each person's own
// private items in memory; nothing records who took part in what.

export const seedFeed: FeedEvent[] = [
	{
		id: 'ev-1',
		date: '2026-11-15',
		kind: 'new_version',
		target: { kind: 'bill', id: 'hr-4127', billId: 'hr-4127' },
		text: 'H.R. 4127: a new official version (v2, reported in House) was published.',
		href: '/bills/hr-4127/v/2'
	},
	{
		id: 'ev-2',
		date: '2026-11-15',
		kind: 'stale',
		target: { kind: 'suggestion', id: 'e3', billId: 'hr-4127' },
		text: 'A suggestion you support now targets lines that changed in v2. It is waiting to be re-anchored.',
		href: '/bills/hr-4127/v/1/sec/s4#e3'
	},
	{
		id: 'ev-3',
		date: '2026-11-16',
		kind: 'revised',
		target: { kind: 'suggestion', id: 'e1', billId: 'hr-4127' },
		text: 'A suggestion you support was revised (steward wording fix accepted). Your support carries over; you can withdraw it.',
		href: '/bills/hr-4127/v/2/sec/s4#e1'
	},
	{
		id: 'ev-4',
		date: '2026-11-14',
		kind: 'decision',
		target: { kind: 'request', id: 's-990', billId: 's-990' },
		text: 'S. 990: your markup request was declined (outside the pilot topics).',
		href: '/catalog#s-990'
	}
];
