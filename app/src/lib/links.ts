import { resolve } from '$app/paths';
import { rangeParam } from '#lib/anchors.ts';
import type { Anchor } from '#lib/types.ts';

export const homeHref = () => resolve('/');
export const catalogHref = () => resolve('/catalog');
export const policyHref = () => resolve('/policy');
export const logHref = () => resolve('/log');

/** Bill page; without a version it shows the latest official version. */
export const billHref = (billId: string, version?: string) =>
	version === undefined
		? resolve('/bills/[bill]', { bill: billId })
		: resolve('/bills/[bill]/v/[version]', { bill: billId, version });

/**
 * Canonical, server-resolvable addresses (design/10). Unlike "#s4-l12-13" fragments,
 * these reach the server, so crawlers, link previews, and agents get the exact text.
 */
export const sectionHref = (billId: string, version: string, sectionId: string) =>
	resolve('/bills/[bill]/v/[version]/s/[section]', {
		bill: billId,
		version,
		section: sectionId
	});

export const linesHref = (anchor: Anchor) =>
	resolve('/bills/[bill]/v/[version]/s/[section]/l/[range]', {
		bill: anchor.billId,
		version: anchor.version,
		section: anchor.sectionId,
		range: rangeParam(anchor.from, anchor.to)
	});

/** Version-less addresses resolve to the latest official version (design/10). */
export const latestSectionHref = (billId: string, sectionId: string) =>
	resolve('/bills/[bill]/s/[section]', { bill: billId, section: sectionId });

export const latestLinesHref = (billId: string, sectionId: string, from: number, to: number) =>
	resolve('/bills/[bill]/s/[section]/l/[range]', {
		bill: billId,
		section: sectionId,
		range: rangeParam(from, to)
	});

/** Public source repository (design/09: the code is open source). */
export const repoHref = 'https://github.com/EasterTheBunny/pubcommittee';
