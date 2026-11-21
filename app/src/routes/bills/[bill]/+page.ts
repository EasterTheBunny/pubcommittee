import { error } from '@sveltejs/kit';
import { billById, bills, latestVersion } from '#lib/mock/index.ts';
import type { EntryGenerator, PageLoad } from './$types';

export const entries: EntryGenerator = () => bills.map((b) => ({ bill: b.id }));

export const load: PageLoad = ({ params }) => {
	const bill = billById(params.bill);
	if (!bill) error(404, 'Bill not found');
	return { bill, version: latestVersion(bill) };
};
