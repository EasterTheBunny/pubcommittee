import { error } from '@sveltejs/kit';
import { parseRange, rangeParam } from '#lib/anchors.ts';
import { billById, bills, sectionOf, versionOf } from '#lib/mock/index.ts';
import type { EntryGenerator, PageLoad } from './$types';

// Canonical address of a line range (design/10), e.g. /bills/hr-4127/v/2/s/s4/l/12-13.
// The static M0 build prerenders every contiguous range in every section, so any
// selection a person can share resolves without client-side state.

export const entries: EntryGenerator = () =>
	bills.flatMap((b) =>
		b.versions.flatMap((v) =>
			v.sections.flatMap((s) =>
				s.lines.flatMap((a, i) =>
					s.lines.slice(i).map((z) => ({
						bill: b.id,
						version: String(v.n),
						section: s.id,
						range: rangeParam(a.n, z.n)
					}))
				)
			)
		)
	);

export const load: PageLoad = ({ params }) => {
	const bill = billById(params.bill);
	const version = bill && versionOf(bill, Number(params.version));
	const section = version && sectionOf(version, params.section);
	const range = parseRange(params.range);
	const first = section?.lines[0]?.n ?? 0;
	const last = section?.lines.at(-1)?.n ?? -1;
	if (!bill || !version || !section || !range || range.from < first || range.to > last) {
		error(404, 'Lines not found');
	}
	return { bill, version, section, ...range };
};
