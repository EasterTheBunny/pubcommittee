import { error } from '@sveltejs/kit';
import { billById, bills, versionOf } from '#lib/mock/index.ts';
import type { EntryGenerator, PageLoad } from './$types';

export const entries: EntryGenerator = () =>
	bills.flatMap((b) => b.versions.map((v) => ({ bill: b.id, version: String(v.n) })));

export const load: PageLoad = ({ params }) => {
	const bill = billById(params.bill);
	const version = bill && versionOf(bill, Number(params.version));
	if (!bill || !version) error(404, 'Bill version not found');
	return { bill, version };
};
