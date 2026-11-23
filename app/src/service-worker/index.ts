// Service worker (architecture: Progressive web app; design/08 Installable app).
// - App shell (build output and static files) is cached on install, so the app starts offline.
// - Official bill text never changes, so version pages and read-api version resources are
//   cache-first once opened, bounded to the 50 most recently opened versions (policy.ts).
// - Everything else on this origin is network-first; the short fallback copy it falls back to is
//   labelled as possibly out of date (a <meta> the OfflineLabel component reads).
// - Only GET requests are handled; nothing credentialed is cached and no write is ever queued.
// - Notifications are content-free: they say updates are waiting, never what changed.

import { version } from '$app/env';
import { assets, immutable } from '$app/manifest';
import { self as sw } from '$app/service-worker';
import {
	cacheNames,
	classify,
	isImmutable,
	storable,
	touchFallback,
	touchText,
	versionUnit,
	type FallbackEntry,
	type TextEntry
} from './policy';

/** Caches are named per release, so a new deploy replaces all of them on activate. */
const NAMES = cacheNames(version);
/** Manifest paths are relative to the worker's location; resolve them once. */
const SHELL = [...immutable, ...assets].map((f) => new URL(f.path, sw.location.href).pathname);
const SHELL_SET = new Set(SHELL);
const INDEX_KEY = new URL('/__pc/index', sw.location.origin).href;
const CACHED_AT = 'x-pc-cached-at';

sw.addEventListener('install', (event) => {
	event.waitUntil(
		(async () => {
			const shell = await caches.open(NAMES.shell);
			await shell.addAll(SHELL);
			// The start page, best effort: lets the installed app open offline before a second visit.
			try {
				const start = await fetch('/');
				if (
					start.ok &&
					storable(
						start.status,
						start.headers.get('cache-control'),
						start.headers.get('vary'),
						false
					)
				) {
					await storeFallback(new Request(new URL('/', sw.location.origin)), start);
				}
			} catch {
				// Offline install: the shell alone is enough.
			}
			await sw.skipWaiting();
		})()
	);
});

sw.addEventListener('activate', (event) => {
	event.waitUntil(
		caches
			.keys()
			.then((keys) =>
				Promise.all(
					keys
						.filter((k) => k.startsWith('pubcommittee-') && !Object.values(NAMES).includes(k))
						.map((k) => caches.delete(k))
				)
			)
			.then(() => sw.clients.claim())
	);
});

// Index updates are serialised: two page loads must not overwrite each other.
let queue: Promise<unknown> = Promise.resolve();
const serial = <T>(job: () => Promise<T>): Promise<T> => {
	const run = queue.then(job, job);
	queue = run.catch(() => undefined);
	return run;
};

async function readIndex<T>(cache: Cache): Promise<T[]> {
	const hit = await cache.match(INDEX_KEY);
	return hit ? ((await hit.json()) as T[]) : [];
}
const writeIndex = (cache: Cache, index: unknown[]) =>
	cache.put(
		INDEX_KEY,
		new Response(JSON.stringify(index), { headers: { 'content-type': 'application/json' } })
	);

/** Marks a version as just opened and evicts the least recently opened ones beyond the bound. */
const touchOfficial = (unit: string, url: string) =>
	serial(async () => {
		const cache = await caches.open(NAMES.text);
		const { index, evict } = touchText(await readIndex<TextEntry>(cache), unit, url, Date.now());
		await Promise.all(evict.map((u) => cache.delete(u, { ignoreVary: true })));
		await writeIndex(cache, index);
	});

/** Stores a fallback copy, stamped with the time it was saved. */
async function storeFallback(request: Request, response: Response): Promise<void> {
	const headers = new Headers(response.headers);
	headers.set(CACHED_AT, new Date().toISOString());
	const stamped = new Response(await response.clone().blob(), { status: response.status, headers });
	await serial(async () => {
		const cache = await caches.open(NAMES.fallback);
		await cache.put(request.url, stamped);
		const { list, evict } = touchFallback(
			await readIndex<FallbackEntry>(cache),
			request.url,
			Date.now()
		);
		await Promise.all(evict.map((u) => cache.delete(u, { ignoreVary: true })));
		await writeIndex(cache, list);
	});
}

async function shellFirst(request: Request): Promise<Response> {
	const cache = await caches.open(NAMES.shell);
	const hit = await cache.match(request, { ignoreVary: true });
	return hit ?? fetch(request);
}

async function officialFirst(event: FetchEvent, unit: string): Promise<Response> {
	const { request } = event;
	const cache = await caches.open(NAMES.text);
	const hit = await cache.match(request.url, { ignoreVary: true });
	if (hit) {
		event.waitUntil(touchOfficial(unit, request.url));
		return hit;
	}
	let response: Response;
	try {
		response = await fetch(request);
	} catch {
		return offlineResponse(request);
	}
	const cc = response.headers.get('cache-control');
	const sameOrigin = new URL(request.url).origin === sw.location.origin;
	const keep =
		storable(
			response.status,
			cc,
			response.headers.get('vary'),
			response.headers.has('set-cookie')
		) && (sameOrigin ? response.type === 'basic' : response.type === 'cors' && isImmutable(cc));
	if (keep) {
		event.waitUntil(
			cache.put(request.url, response.clone()).then(() => touchOfficial(unit, request.url))
		);
	}
	return response;
}

async function networkFirst(event: FetchEvent): Promise<Response> {
	const { request } = event;
	const cache = await caches.open(NAMES.fallback);
	let response: Response | undefined;
	try {
		response = await fetch(request);
		if (response.status < 500) {
			if (
				response.type === 'basic' &&
				storable(
					response.status,
					response.headers.get('cache-control'),
					response.headers.get('vary'),
					response.headers.has('set-cookie')
				)
			) {
				event.waitUntil(storeFallback(request, response));
			}
			return response;
		}
	} catch {
		// Offline: fall through to the saved copy.
	}
	const copy = await cache.match(request.url, { ignoreVary: true });
	if (copy) return label(copy);
	return response ?? offlineResponse(request);
}

/** Adds the possibly-out-of-date marker to a saved HTML page; other types pass unchanged. */
async function label(copy: Response): Promise<Response> {
	const savedAt = copy.headers.get(CACHED_AT) ?? '';
	if (!(copy.headers.get('content-type') ?? '').includes('text/html')) return copy;
	const html = (await copy.text()).replace(
		'</head>',
		`<meta name="pc-offline-copy" content="${savedAt}"></head>`
	);
	const headers = new Headers(copy.headers);
	headers.delete('content-length');
	return new Response(html, { status: copy.status, headers });
}

function offlineResponse(request: Request): Response {
	if (request.mode !== 'navigate') return Response.error();
	return new Response(
		`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Offline · PubCommittee</title></head><body style="font-family:system-ui,sans-serif;max-width:36rem;margin:3rem auto;padding:0 1rem"><h1>You are offline</h1><p data-offline-page>This page has not been opened on this device yet, so it is not saved. Bill versions you opened before are still available.</p><p><a href="/">Back to the start page</a></p></body></html>`,
		{ status: 503, headers: { 'content-type': 'text/html; charset=utf-8' } }
	);
}

sw.addEventListener('fetch', (event) => {
	const { request } = event;
	const url = new URL(request.url);
	const kind = classify({
		method: request.method,
		url,
		selfOrigin: sw.location.origin,
		hasAuthorization: request.headers.has('authorization'),
		shell: SHELL_SET
	});
	if (kind === 'skip') return;
	if (kind === 'shell') event.respondWith(shellFirst(request));
	else if (kind === 'official') {
		const unit = versionUnit(url.pathname, url.origin === sw.location.origin);
		if (unit) event.respondWith(officialFirst(event, unit));
	} else event.respondWith(networkFirst(event));
});

/** Shows the one content-free notification the product ever sends. */
function showUpdates() {
	return sw.registration.showNotification('PubCommittee', {
		body: 'You have updates.',
		icon: '/icons/icon-192.png',
		badge: '/icons/icon-192.png',
		tag: 'updates'
	});
}

// Web Push (M2): the payload is only { kind: "updates" }; whatever arrives, nothing is shown
// beyond the content-free message.
sw.addEventListener('push', (event) => {
	event.waitUntil(showUpdates());
});

sw.addEventListener('notificationclick', (event) => {
	event.notification.close();
	event.waitUntil(
		sw.clients.matchAll({ type: 'window', includeUncontrolled: true }).then((list) => {
			const open = list.find((c) => 'focus' in c) as WindowClient | undefined;
			if (open) return open.navigate('/inbox').then((c) => c?.focus());
			return sw.clients.openWindow('/inbox');
		})
	);
});
