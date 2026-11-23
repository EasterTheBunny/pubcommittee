import { loadAddressPage } from '#lib/server/bill.ts';
import type { PageServerLoad } from './$types';

// Version-less address: resolves to the latest official version and says so.
export const load: PageServerLoad = async ({ fetch, params, setHeaders }) => {
	setHeaders({ 'cache-control': 'public, max-age=60' });
	return loadAddressPage(fetch, { bill: params.bill, section: params.section });
};
