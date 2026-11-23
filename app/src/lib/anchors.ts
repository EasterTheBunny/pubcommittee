import type { Line } from '#lib/types.ts';

/** Hash fragment for a section or line range: "#s4" or "#s4-l12-13". Line numbers are version-wide. */
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

/**
 * The block starting at line n is the line plus every deeper line nested under it.
 * The API gives it as `block_to` on each line (design/10 Canonical addresses).
 */
export function blockRange(lines: Line[], n: number): { from: number; to: number } | null {
	const line = lines.find((l) => l.n === n);
	return line ? { from: line.n, to: line.block_to } : null;
}
