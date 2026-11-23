// read-api client. Server-side GET only, with the generated types (architecture § Web tier).
// No credentials, no cookies: the web host holds none and sends none.
import type { components, operations } from '../api/schema';

/** Where this server reads from (the `local` target in development). */
export const readApiUrl = () => (process.env.READ_API_URL ?? 'http://localhost:8080').replace(/\/$/, '');

/** Where browsers and machines find the same resources (rel="alternate", 303); defaults to the read URL. */
export const publicApiUrl = () => (process.env.PUBLIC_API_URL ?? readApiUrl()).replace(/\/$/, '');

export type Problem = components['schemas']['Problem'];

export type ApiResult<T> =
	| { ok: true; data: T; cacheControl: string | null }
	| { ok: false; status: number; problem: Problem | null };

type Query = Record<string, string | number | boolean | undefined>;
type Fetch = typeof fetch;

/** GET one resource. Never throws on an HTTP error; a network failure is status 503. */
export async function apiGet<T>(f: Fetch, path: string, query: Query = {}): Promise<ApiResult<T>> {
	const url = new URL(readApiUrl() + path);
	for (const [k, v] of Object.entries(query)) {
		if (v !== undefined && v !== '') url.searchParams.set(k, String(v));
	}
	let res: Response;
	try {
		res = await f(url, {
			method: 'GET',
			headers: { accept: 'application/json', 'user-agent': 'pubcommittee-web' }
		});
	} catch {
		return { ok: false, status: 503, problem: null };
	}
	if (!res.ok) {
		let problem: Problem | null = null;
		try {
			problem = (await res.json()) as Problem;
		} catch {
			// not a problem document
		}
		return { ok: false, status: res.status, problem };
	}
	return { ok: true, data: (await res.json()) as T, cacheControl: res.headers.get('cache-control') };
}

type Body<Op extends keyof operations> = operations[Op]['responses'] extends { 200: infer R }
	? R extends { content: { 'application/json': infer J } }
		? J
		: never
	: never;

type Path<Op extends keyof operations> = operations[Op] extends { parameters: { path: infer P } }
	? P
	: never;

const enc = encodeURIComponent;

export const readApi = {
	listBills: (f: Fetch, q: operations['listBills']['parameters']['query'] = {}) =>
		apiGet<Body<'listBills'>>(f, '/v1/bills', q as Query),
	getBill: (f: Fetch, bill: string) => apiGet<Body<'getBill'>>(f, `/v1/bills/${enc(bill)}`),
	getVersion: (f: Fetch, bill: string, n: string | number) =>
		apiGet<Body<'getVersion'>>(f, `/v1/bills/${enc(bill)}/v/${enc(String(n))}`),
	getSection: (f: Fetch, p: Path<'getSection'>) =>
		apiGet<Body<'getSection'>>(f, `/v1/bills/${enc(p.bill)}/v/${enc(p.n)}/s/${enc(p.section)}`),
	getLineRange: (f: Fetch, p: Path<'getLineRange'>) =>
		apiGet<Body<'getLineRange'>>(
			f,
			`/v1/bills/${enc(p.bill)}/v/${enc(p.n)}/s/${enc(p.section)}/l/${enc(p.range)}`
		),
	getSectionLatest: (f: Fetch, p: Path<'getSectionLatest'>) =>
		apiGet<Body<'getSectionLatest'>>(f, `/v1/bills/${enc(p.bill)}/s/${enc(p.section)}`),
	getLineRangeLatest: (f: Fetch, p: Path<'getLineRangeLatest'>) =>
		apiGet<Body<'getLineRangeLatest'>>(
			f,
			`/v1/bills/${enc(p.bill)}/s/${enc(p.section)}/l/${enc(p.range)}`
		),
	listExplanations: (f: Fetch, bill: string, n: number) =>
		apiGet<Body<'listExplanations'>>(f, `/v1/bills/${enc(bill)}/v/${n}/explanations`, {
			limit: 200
		}),
	getBillSummary: (f: Fetch, bill: string, n: number) =>
		apiGet<Body<'getBillSummary'>>(f, `/v1/bills/${enc(bill)}/v/${n}/summary`),
	getVersionChangeSummary: (f: Fetch, bill: string, n: number) =>
		apiGet<Body<'getVersionChangeSummary'>>(f, `/v1/bills/${enc(bill)}/v/${n}/changes`),
	listCitations: (f: Fetch, bill: string, n: number) =>
		apiGet<Body<'listCitations'>>(f, `/v1/bills/${enc(bill)}/v/${n}/citations`, { limit: 200 }),
	compareVersions: (f: Fetch, bill: string, n: number, m: number) =>
		apiGet<Body<'compareVersions'>>(f, `/v1/bills/${enc(bill)}/v/${n}/compare/${m}`),
	getSelectionPolicy: (f: Fetch) => apiGet<Body<'getSelectionPolicy'>>(f, '/v1/selection-policy'),
	listModerationLog: (f: Fetch, cursor?: string) =>
		apiGet<Body<'listModerationLog'>>(f, '/v1/moderation-log', { cursor, limit: 50 })
};
