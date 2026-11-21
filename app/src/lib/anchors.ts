import type { Anchor, Bill, Line } from '#lib/types.ts';
import { latestVersion, sectionOf, versionOf } from '#lib/mock/index.ts';

/** Hash fragment for a section or line range: "#s4" or "#s4-l12-13". */
export function anchorHash(sectionId: string, from?: number, to?: number): string {
	if (from === undefined) return `#${sectionId}`;
	return to === undefined || to === from
		? `#${sectionId}-l${from}`
		: `#${sectionId}-l${from}-${to}`;
}

export interface HashSelection {
	sectionId: string;
	from?: number;
	to?: number;
}

export function parseHash(hash: string): HashSelection | null {
	const m = /^#?(s\d+)(?:-l(\d+)(?:-(\d+))?)?$/.exec(hash);
	if (!m) return null;
	const from = m[2] ? Number(m[2]) : undefined;
	const to = m[3] ? Number(m[3]) : from;
	return { sectionId: m[1], from, to };
}

/** Path segment for a line range: "12" or "12-13". */
export const rangeParam = (from: number, to: number) =>
	from === to ? String(from) : `${from}-${to}`;

export function parseRange(param: string): { from: number; to: number } | null {
	const m = /^(\d+)(?:-(\d+))?$/.exec(param);
	if (!m) return null;
	const from = Number(m[1]);
	const to = m[2] ? Number(m[2]) : from;
	return to >= from ? { from, to } : null;
}

export function rangeLabel(from: number, to: number): string {
	return from === to ? `line ${from}` : `lines ${from}–${to}`;
}

export function linesFor(bill: Bill, anchor: Anchor): Line[] {
	const version = versionOf(bill, anchor.version);
	const section = version && sectionOf(version, anchor.sectionId);
	return section ? section.lines.filter((l) => l.n >= anchor.from && l.n <= anchor.to) : [];
}

export function anchorText(bill: Bill, anchor: Anchor): string {
	return linesFor(bill, anchor)
		.map((l) => l.text)
		.join(' ');
}

/**
 * An item is stale when the latest official version changed the text it targets (design/02).
 * M0 compares the same line numbers; M1 replaces this with a real version-to-version mapping.
 */
export function isStale(bill: Bill, anchor: Anchor): boolean {
	const latest = latestVersion(bill);
	if (anchor.version === latest.n) return false;
	return anchorText(bill, anchor) !== anchorText(bill, { ...anchor, version: latest.n });
}

export const overlaps = (a: Anchor, b: Anchor) =>
	a.billId === b.billId && a.sectionId === b.sectionId && a.from <= b.to && b.from <= a.to;
