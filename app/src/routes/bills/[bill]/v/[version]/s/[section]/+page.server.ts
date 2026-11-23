import { loadAddressPage } from '#lib/server/bill.ts';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, params, setHeaders }) => {
	setHeaders({ 'cache-control': 'public, max-age=60' });
	return loadAddressPage(fetch, {
		bill: params.bill,
		version: params.version,
		section: params.section
	});
};
