import { error } from '@sveltejs/kit';
import { readApi } from '#lib/server/api.ts';
import type { CatalogEntry } from '#lib/types.ts';
import type { PageServerLoad } from './$types';

const chambers = ['house', 'senate', 'joint'] as const;
type Chamber = (typeof chambers)[number];

const isChamber = (value: string): value is Chamber =>
	(chambers as readonly string[]).includes(value);

export const load: PageServerLoad = async ({ fetch, setHeaders, url }) => {
	const q = url.searchParams.get('q')?.trim() ?? '';
	const status = url.searchParams.get('status')?.trim() ?? '';
	const rawChamber = url.searchParams.get('chamber')?.trim() ?? '';
	const chamber = isChamber(rawChamber) ? rawChamber : '';
	const rawCommittee = url.searchParams.get('committee_activity')?.trim() ?? '';
	const committeeActivity = rawCommittee === 'true' || rawCommittee === 'false' ? rawCommittee : '';
	const subject = url.searchParams.get('subject')?.trim() ?? '';
	const cursor = url.searchParams.get('cursor')?.trim() ?? '';

	const filters = {
		q,
		status,
		chamber,
		committee_activity: committeeActivity,
		subject
	};

	const result = await readApi.listBills(fetch, {
		scope: 'catalog',
		q: q || undefined,
		status: status || undefined,
		chamber: chamber || undefined,
		committee_activity: committeeActivity === '' ? undefined : committeeActivity === 'true',
		subject: subject || undefined,
		cursor: cursor || undefined,
		limit: 50
	});
	if (!result.ok) error(result.status, 'The catalog is not available right now.');

	const entries = (result.data.items ?? []) as CatalogEntry[];
	const next: string | null = result.data.next ?? null;

	const activeParams = new URLSearchParams();
	for (const [key, value] of Object.entries(filters)) {
		if (value !== '') activeParams.set(key, value);
	}
	if (next !== null) activeParams.set('cursor', next);
	const nextHref = next === null ? null : `?${activeParams.toString()}`;

	const breakdown = {
		total: entries.length,
		tracked: entries.filter((entry) => entry.tracked).length,
		inReview: entries.filter((entry) => entry.markup_status === 'in-review').length,
		eligible: entries.filter((entry) => entry.markup_status === 'eligible').length,
		notCandidate: entries.filter((entry) => entry.markup_status === 'not-a-candidate').length
	};

	setHeaders({ 'cache-control': 'public, max-age=60' });
	return { entries, next, filters, nextHref, breakdown };
};
