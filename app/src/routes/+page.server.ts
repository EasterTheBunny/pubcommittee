import { error } from '@sveltejs/kit';
import { readApi } from '#lib/server/api.ts';
import type { BillSummary } from '#lib/types.ts';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, setHeaders }) => {
	const [bills, policy] = await Promise.all([
		readApi.listBills(fetch, { scope: 'tracked', limit: 50 }),
		readApi.getSelectionPolicy(fetch)
	]);
	if (!bills.ok) error(bills.status, 'The list of bills in review is not available right now.');
	setHeaders({ 'cache-control': 'public, max-age=60' });
	return {
		bills: (bills.data.items ?? []) as BillSummary[],
		next: bills.data.next ?? null,
		policy: policy.ok ? policy.data : null
	};
};
