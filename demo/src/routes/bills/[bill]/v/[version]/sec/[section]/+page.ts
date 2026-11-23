import { error } from '@sveltejs/kit';
import { billById, bills, sectionOf, versionOf } from '#lib/mock/index.ts';
import type { EntryGenerator, PageLoad } from './$types';

export const entries: EntryGenerator = () =>
	bills.flatMap((b) =>
		b.versions.flatMap((v) =>
			v.sections.map((s) => ({ bill: b.id, version: String(v.n), section: s.id }))
		)
	);

export const load: PageLoad = ({ params }) => {
	const bill = billById(params.bill);
	const version = bill && versionOf(bill, Number(params.version));
	const section = version && sectionOf(version, params.section);
	if (!bill || !version || !section) error(404, 'Section not found');
	return { bill, version, section };
};
