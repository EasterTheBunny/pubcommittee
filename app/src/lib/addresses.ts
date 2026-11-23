// Maps a page address to the read-api resource that holds the same data (design/10 Machine-readable
// forms). Pure, so the server hook (303 for Accept: application/json) and the layout (rel="alternate")
// share it. Page paths mirror API paths on purpose.

const BILL = '[^/]+';

const ROUTES: { page: RegExp; api: (m: RegExpExecArray) => string }[] = [
	{ page: /^\/$/, api: () => '/v1' },
	{ page: /^\/catalog\/?$/, api: () => '/v1/bills?scope=catalog' },
	{ page: /^\/policy\/?$/, api: () => '/v1/selection-policy' },
	{ page: /^\/log\/?$/, api: () => '/v1/moderation-log' },
	{ page: new RegExp(`^/bills/(${BILL})/?$`), api: (m) => `/v1/bills/${m[1]}` },
	{
		page: new RegExp(`^/bills/(${BILL})/v/([^/]+)/?$`),
		api: (m) => `/v1/bills/${m[1]}/v/${m[2]}`
	},
	{
		page: new RegExp(`^/bills/(${BILL})/v/([^/]+)/s/([^/]+)/?$`),
		api: (m) => `/v1/bills/${m[1]}/v/${m[2]}/s/${m[3]}`
	},
	{
		page: new RegExp(`^/bills/(${BILL})/v/([^/]+)/s/([^/]+)/l/([^/]+)/?$`),
		api: (m) => `/v1/bills/${m[1]}/v/${m[2]}/s/${m[3]}/l/${m[4]}`
	},
	{
		page: new RegExp(`^/bills/(${BILL})/s/([^/]+)/?$`),
		api: (m) => `/v1/bills/${m[1]}/s/${m[2]}`
	},
	{
		page: new RegExp(`^/bills/(${BILL})/s/([^/]+)/l/([^/]+)/?$`),
		api: (m) => `/v1/bills/${m[1]}/s/${m[2]}/l/${m[3]}`
	}
];

/** The read-api path (with the query string it needs) for a page URL, or null when none exists. */
export function apiPathFor(url: URL): string | null {
	for (const r of ROUTES) {
		const m = r.page.exec(url.pathname);
		if (!m) continue;
		const path = r.api(m);
		if (url.pathname.replace(/\/$/, '') !== '/catalog') return path;
		// Catalog filters travel to the API unchanged; the page's own scope is always catalog.
		const q = new URLSearchParams(url.search);
		q.set('scope', 'catalog');
		return `/v1/bills?${q.toString()}`;
	}
	return null;
}

/**
 * True when the request prefers JSON over HTML: `application/json` is listed with a quality at
 * least that of every HTML type. `*` + `/*` alone never counts (curl and crawlers send it).
 */
export function prefersJson(accept: string | null): boolean {
	if (!accept) return false;
	let json = 0;
	let html = 0;
	for (const part of accept.split(',')) {
		const [type, ...params] = part.trim().toLowerCase().split(';');
		const qp = params.map((p) => p.trim()).find((p) => p.startsWith('q='));
		const q = qp ? Number(qp.slice(2)) : 1;
		if (Number.isNaN(q)) continue;
		if (type === 'application/json') json = Math.max(json, q);
		if (type === 'text/html' || type === 'application/xhtml+xml') html = Math.max(html, q);
	}
	return json > 0 && json >= html;
}
