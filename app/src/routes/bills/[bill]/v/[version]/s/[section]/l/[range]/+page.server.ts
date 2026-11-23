import { error } from '@sveltejs/kit';
import { parseRange } from '#lib/anchors.ts';
import { loadAddressPage } from '#lib/server/bill.ts';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, params, setHeaders }) => {
	const range = parseRange(params.range);
	if (!range) error(400, 'A line range looks like 12 or 12-13.');
	setHeaders({ 'cache-control': 'public, max-age=60' });
	return loadAddressPage(fetch, {
		bill: params.bill,
		version: params.version,
		section: params.section,
		range
	});
};
