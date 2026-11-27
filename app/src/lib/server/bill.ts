// Loaders shared by the bill pages. Everything comes from read-api; a route the API answers 404 (or
// empty) for renders its designed unavailable or empty state, never invented data (M1-13).
import { error, redirect } from '@sveltejs/kit';
import { billHref } from '#lib/links.ts';
import { readApi } from '#lib/server/api.ts';
import type {
	Bill,
	BillSummaryDoc,
	CatalogEntry,
	Citation,
	ComparisonSection,
	Explanation,
	LineRangeResource,
	OfficialVersion,
	SectionResource,
	VersionChangeSummary,
	VersionComparison
} from '#lib/types.ts';

type Fetch = typeof fetch;

/** The version path segment: a lower-case version code or the word "latest" (design/10 Canonical addresses). */
export const isVersionParam = (v: string) => /^([a-z]{2,4}|latest)$/.test(v);

export interface Unavailable {
	/** Why a part is missing: the API has no such document yet, or it could not be reached. */
	reason: 'not-published' | 'unreachable';
}

const unavailable = (status: number): Unavailable => ({
	reason: status === 404 ? 'not-published' : 'unreachable'
});

export type Part<T> = { ok: true; data: T } | ({ ok: false } & Unavailable);

async function part<T>(
	p: Promise<{ ok: true; data: T } | { ok: false; status: number }>
): Promise<Part<T>> {
	const r = await p;
	return r.ok ? { ok: true, data: r.data } : { ok: false, ...unavailable(r.status) };
}

export type BillPageData =
	| { kind: 'untracked'; entry: CatalogEntry }
	| {
			kind: 'tracked';
			bill: Bill;
			version: OfficialVersion;
			explanations: Explanation[];
			summary: Part<BillSummaryDoc>;
			changes: Part<VersionChangeSummary> | null;
			citations: Part<Citation[]>;
	  };

/** An entry that is in the catalog but not tracked: it links out and never shows text. */
async function catalogEntry(f: Fetch, billId: string): Promise<CatalogEntry | null> {
	const r = await readApi.listBills(f, { scope: 'catalog', q: billId, limit: 50 });
	if (!r.ok) return null;
	const items = (r.data.items ?? []) as CatalogEntry[];
	// Untracked, or tracked with no official text stored yet (the text pull is its own step).
	return items.find((e) => e.id === billId) ?? null;
}

export async function loadBillPage(
	f: Fetch,
	billId: string,
	versionParam: string
): Promise<BillPageData> {
	if (!isVersionParam(versionParam)) error(404, 'No such version.');
	const [bill, version] = await Promise.all([
		readApi.getBill(f, billId),
		readApi.getVersion(f, billId, versionParam)
	]);
	if (!bill.ok || !version.ok) {
		const status = !bill.ok ? bill.status : (version as { status: number }).status;
		if (status === 404) {
			const entry = await catalogEntry(f, billId);
			if (entry) return { kind: 'untracked', entry };
			error(404, 'This bill is not in the catalog.');
		}
		error(status === 429 ? 429 : 503, 'This bill is not available right now.');
	}
	const code = version.data.code;
	// A referral that collapsed into another version answers with that version (the API redirects);
	// the page address follows, so a link to the referral lands on the version it collapsed into.
	if (versionParam !== 'latest' && versionParam !== code) redirect(308, billHref(billId, code));
	const first = bill.data.versions[0]?.code === code;
	const [explanations, summary, changes, citations] = await Promise.all([
		readApi.listExplanations(f, billId, code),
		part(readApi.getBillSummary(f, billId, code)),
		first ? Promise.resolve(null) : part(readApi.getVersionChangeSummary(f, billId, code)),
		part(readApi.listCitations(f, billId, code))
	]);
	return {
		kind: 'tracked',
		bill: bill.data,
		version: version.data,
		explanations: explanations.ok ? ((explanations.data.items ?? []) as Explanation[]) : [],
		summary,
		changes,
		citations: citations.ok
			? { ok: true, data: (citations.data.items ?? []) as Citation[] }
			: citations
	};
}

/** What the "changed since" notice says about an old-version address (design/06 Link behavior). */
export type ChangedSince =
	| { kind: 'current' }
	| { kind: 'unknown'; currentVersion: string }
	| { kind: 'unchanged'; currentVersion: string }
	| {
			kind: 'changed';
			currentVersion: string;
			/** Where the same words now live in the current version, when the comparison says so. */
			now: { sectionId: string; from: number; to: number } | null;
	  };

const overlaps = (from: number, to: number, a: number, b: number) => from <= b && a <= to;

/** Pure: decide the notice from a comparison (M1-08 data) for the lines [from, to] of a section. */
export function changedSince(
	cmp: VersionComparison,
	sectionId: string,
	from: number,
	to: number
): ChangedSince {
	const currentVersion = cmp.to_code;
	const sec: ComparisonSection | undefined = cmp.sections.find((s) => s.section_id === sectionId);
	if (!sec || sec.status === 'unchanged') return { kind: 'unchanged', currentVersion };
	if (sec.status === 'removed') return { kind: 'changed', currentVersion, now: null };
	for (const c of sec.changes) {
		const b = c.before_span;
		if (!b || b.section_id !== sectionId || !overlaps(from, to, b.from, b.to)) continue;
		if (c.kind === 'added') continue;
		const a = c.after_span;
		return {
			kind: 'changed',
			currentVersion,
			now: c.kind !== 'removed' && a ? { sectionId: a.section_id, from: a.from, to: a.to } : null
		};
	}
	return { kind: 'unchanged', currentVersion };
}

export interface AddressPageData {
	bill: Bill;
	address: SectionResource['address'];
	code: string;
	sectionId: string;
	sectionPath: string[];
	section: SectionResource['section'] | null;
	lines: LineRangeResource['lines'];
	range: { from: number; to: number } | null;
	explanations: Explanation[];
	notice: ChangedSince;
	versionLabel: string;
}

/** Load a section or line range address, pinned (`version`) or version-less (undefined). */
export async function loadAddressPage(
	f: Fetch,
	p: { bill: string; version?: string; section: string; range?: { from: number; to: number } }
): Promise<AddressPageData> {
	if (p.version !== undefined && !isVersionParam(p.version)) error(404, 'No such version.');
	const rangeStr = p.range
		? p.range.from === p.range.to
			? String(p.range.from)
			: `${p.range.from}-${p.range.to}`
		: undefined;

	const fetchAddress = async () => {
		if (rangeStr !== undefined) {
			const r =
				p.version === undefined
					? await readApi.getLineRangeLatest(f, {
							bill: p.bill,
							section: p.section,
							range: rangeStr
						})
					: await readApi.getLineRange(f, {
							bill: p.bill,
							version: p.version,
							section: p.section,
							range: rangeStr
						});
			if (!r.ok) return r;
			const d: LineRangeResource = r.data;
			return {
				ok: true as const,
				code: d.code,
				address: d.address,
				sectionId: d.section_id,
				sectionPath: d.section_path,
				section: null,
				lines: d.lines,
				range: { from: d.from, to: d.to }
			};
		}
		const r =
			p.version === undefined
				? await readApi.getSectionLatest(f, { bill: p.bill, section: p.section })
				: await readApi.getSection(f, { bill: p.bill, version: p.version, section: p.section });
		if (!r.ok) return r;
		const d: SectionResource = r.data;
		return {
			ok: true as const,
			code: d.code,
			address: d.address,
			sectionId: d.section.id,
			sectionPath: d.section.path,
			section: d.section,
			lines: [] as LineRangeResource['lines'],
			range: null
		};
	};

	const [bill, res] = await Promise.all([readApi.getBill(f, p.bill), fetchAddress()]);
	if (!bill.ok) error(bill.status === 404 ? 404 : 503, 'This bill is not available right now.');
	if (!res.ok)
		error(res.status === 404 || res.status === 400 ? res.status : 503, 'No such address.');

	const explanations = await readApi.listExplanations(f, p.bill, res.code);
	let notice: ChangedSince = { kind: 'current' };
	if (!res.address.is_latest) {
		const latest = res.address.latest_version;
		const cmp = await readApi.compareVersions(f, p.bill, res.code, latest);
		if (cmp.ok) {
			const lines = res.section ? res.section.lines : res.lines;
			const from = res.range?.from ?? lines[0]?.n ?? 1;
			const to = res.range?.to ?? lines[lines.length - 1]?.n ?? from;
			notice = changedSince(cmp.data, res.sectionId, from, to);
		} else {
			notice = { kind: 'unknown', currentVersion: latest };
		}
	}
	const v = bill.data.versions.find((x) => x.code === res.code);
	return {
		bill: bill.data,
		address: res.address,
		code: res.code,
		sectionId: res.sectionId,
		sectionPath: res.sectionPath,
		section: res.section,
		lines: res.lines,
		range: res.range,
		explanations: explanations.ok
			? ((explanations.data.items ?? []) as Explanation[]).filter(
					(e) => e.section_id === res.sectionId
				)
			: [],
		notice,
		versionLabel: v ? `${v.type} (${v.code})` : `version ${res.code}`
	};
}
