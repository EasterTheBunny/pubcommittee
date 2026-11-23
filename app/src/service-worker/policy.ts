// Pure rules for the service worker (architecture: Progressive web app). No browser APIs here, so
// tests/sw-policy.spec.ts can run them in Node. index.ts applies them.

/** Official version pages kept offline, least recently opened evicted first (architecture § PWA). */
export const TEXT_LIMIT = 50;
/** Short fallback copies of catalog, explanations, summaries, policy, and log pages. */
export const FALLBACK_LIMIT = 40;

export type Kind =
	/** Never touched by the service worker (credentialed, written, other origins). */
	| 'skip'
	/** Build output and static files: cache-first, replaced by the next release. */
	| 'shell'
	/** An official version page or read-api version resource: cache-first, bounded. */
	| 'official'
	/** Everything else on this origin: network-first with a labelled fallback copy. */
	| 'dynamic';

/** Routes that carry or will carry per-person data (M2). Never cached, whatever the response says. */
const CREDENTIALED = /^\/(v1\/me|auth|api\/me)(\/|$)/;
/** Pages and data for one official version: /bills/{bill}/v/{n}, its sections and lines. */
const OFFICIAL_PAGE = /^\/bills\/([^/]+)\/v\/(\d+)(\/s\/[^/]+(\/l\/[^/]+)?)?(\/__data\.json)?\/?$/;
/** The same version on the read-api origin (immutable resources). */
const OFFICIAL_API = /^\/v1\/bills\/([^/]+)\/v\/(\d+)(\/|$)/;

/** The eviction unit: one bill version, e.g. /bills/us-119-hr-4127/v/2, or null. */
export function versionUnit(pathname: string, sameOrigin: boolean): string | null {
	const m = (sameOrigin ? OFFICIAL_PAGE : OFFICIAL_API).exec(pathname);
	return m ? `/bills/${m[1]}/v/${m[2]}` : null;
}

export interface RequestFacts {
	method: string;
	url: URL;
	selfOrigin: string;
	hasAuthorization: boolean;
	shell: ReadonlySet<string>;
}

export function classify(r: RequestFacts): Kind {
	// Writes are never handled, so they are never queued or replayed: a write offline fails visibly.
	if (r.method !== 'GET') return 'skip';
	if (r.hasAuthorization) return 'skip';
	const { pathname } = r.url;
	if (CREDENTIALED.test(pathname)) return 'skip';
	if (r.url.origin !== r.selfOrigin) {
		// Cross-origin: only immutable read-api version resources (checked again on the response).
		return OFFICIAL_API.test(pathname) ? 'official' : 'skip';
	}
	if (pathname === '/service-worker.js') return 'skip';
	if (r.shell.has(pathname)) return 'shell';
	if (versionUnit(pathname, true)) return 'official';
	return 'dynamic';
}

/** Respect Cache-Control: never keep a response the server marked private or no-store. */
export function storable(
	status: number,
	cacheControl: string | null,
	vary: string | null,
	hasSetCookie: boolean
): boolean {
	if (status !== 200 || hasSetCookie) return false;
	if (vary?.split(',').some((v) => v.trim() === '*')) return false;
	return !/\b(no-store|private)\b/i.test(cacheControl ?? '');
}

/** Cross-origin responses are kept only when the API says they never change. */
export const isImmutable = (cacheControl: string | null) =>
	/\bimmutable\b/i.test(cacheControl ?? '');

export interface TextEntry {
	unit: string;
	at: number;
	urls: string[];
}

/** Records that `url` of `unit` was opened at `now`; returns the new index and URLs to delete. */
export function touchText(
	index: readonly TextEntry[],
	unit: string,
	url: string,
	now: number,
	limit = TEXT_LIMIT
): { index: TextEntry[]; evict: string[] } {
	const prior = index.find((e) => e.unit === unit);
	const entry: TextEntry = {
		unit,
		at: now,
		urls: prior && prior.urls.includes(url) ? prior.urls : [...(prior?.urls ?? []), url]
	};
	const next = [...index.filter((e) => e.unit !== unit), entry].sort((a, b) => a.at - b.at);
	const evicted = next.splice(0, Math.max(0, next.length - limit));
	return { index: next, evict: evicted.flatMap((e) => e.urls) };
}

export interface FallbackEntry {
	url: string;
	at: number;
}

/** Same idea for fallback copies: the oldest copies go first once over the limit. */
export function touchFallback(
	list: readonly FallbackEntry[],
	url: string,
	now: number,
	limit = FALLBACK_LIMIT
): { list: FallbackEntry[]; evict: string[] } {
	const next = [...list.filter((e) => e.url !== url), { url, at: now }].sort((a, b) => a.at - b.at);
	const evicted = next.splice(0, Math.max(0, next.length - limit));
	return { list: next, evict: evicted.map((e) => e.url) };
}

/** Cache names for one release; every other `pubcommittee-` cache is deleted on activate. */
export function cacheNames(version: string) {
	return {
		shell: `pubcommittee-shell-${version}`,
		text: `pubcommittee-text-${version}`,
		fallback: `pubcommittee-fallback-${version}`
	};
}
