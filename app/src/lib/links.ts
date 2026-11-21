import { resolve } from '$app/paths';
import { anchorHash, rangeParam } from '#lib/anchors.ts';
import type { Anchor } from '#lib/types.ts';

export const homeHref = () => resolve('/');

export const billHref = (billId: string, version?: number) =>
	version === undefined
		? resolve('/bills/[bill]', { bill: billId })
		: resolve('/bills/[bill]/v/[version]', { bill: billId, version: String(version) });

/**
 * Canonical, server-resolvable addresses (design/10). Unlike "#s4-l12-13" fragments,
 * these reach the server, so crawlers, link previews, and agents get the exact text.
 */
export const sectionHref = (billId: string, version: number, sectionId: string) =>
	resolve('/bills/[bill]/v/[version]/s/[section]', {
		bill: billId,
		version: String(version),
		section: sectionId
	});

export const linesHref = (anchor: Anchor) =>
	resolve('/bills/[bill]/v/[version]/s/[section]/l/[range]', {
		bill: anchor.billId,
		version: String(anchor.version),
		section: anchor.sectionId,
		range: rangeParam(anchor.from, anchor.to)
	});

export const sectionSuggestionsHref = (
	billId: string,
	version: number,
	sectionId: string,
	from?: number,
	to?: number
) =>
	resolve('/bills/[bill]/v/[version]/sec/[section]', {
		bill: billId,
		version: String(version),
		section: sectionId
	}) + (from === undefined ? '' : anchorHash(sectionId, from, to));

export const issuesHref = () => resolve('/issues');
export const catalogHref = () => resolve('/catalog');
export const issueHref = (issueId: string) => resolve('/issues/[issue]', { issue: issueId });
export const briefsHref = () => resolve('/briefs');
export const briefHref = (billId: string) => resolve('/briefs/[bill]', { bill: billId });

/** Public source repository (design/09: the code is open source). */
export const repoHref = 'https://github.com/EasterTheBunny/pubcommittee';
