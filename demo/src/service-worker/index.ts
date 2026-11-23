// Service worker (architecture: Progressive web app; design/08 Installable app).
// - App shell (build output and static files) is cached on install, so the app starts offline.
// - Official bill text never changes, so version pages are cache-first once opened.
// - Everything else is network-first with the cached copy as an offline fallback.
// - Only same-origin GET requests are handled; nothing credentialed or written is cached.
// - Notifications are content-free: they say updates are waiting, never what changed.

import { version } from '$app/env';
import { assets, immutable } from '$app/manifest';
import { self as sw } from '$app/service-worker';

const CACHE = `pubcommittee-${version}`;
/** Manifest paths are relative to the worker's location; resolve them once. */
const SHELL = [...immutable, ...assets].map((f) => new URL(f.path, sw.location.href).pathname);
const SHELL_SET = new Set(SHELL);

/** Pages showing one official version: /bills/{bill}/v/{n}, plus its section and line addresses. */
const OFFICIAL_TEXT = /^\/bills\/[^/]+\/v\/\d+(\/s\/[^/]+(\/l\/[^/]+)?)?\/?$/;

sw.addEventListener('install', (event) => {
	event.waitUntil(
		caches
			.open(CACHE)
			.then((cache) => cache.addAll(SHELL))
			.then(() => sw.skipWaiting())
	);
});

sw.addEventListener('activate', (event) => {
	event.waitUntil(
		caches
			.keys()
			.then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
			.then(() => sw.clients.claim())
	);
});

async function cacheFirst(request: Request): Promise<Response> {
	const cache = await caches.open(CACHE);
	const hit = await cache.match(request);
	if (hit) return hit;
	const response = await fetch(request);
	if (response.ok && response.type === 'basic') cache.put(request, response.clone());
	return response;
}

async function networkFirst(request: Request): Promise<Response> {
	const cache = await caches.open(CACHE);
	try {
		const response = await fetch(request);
		if (response.ok && response.type === 'basic') cache.put(request, response.clone());
		return response;
	} catch (err) {
		const hit = await cache.match(request);
		if (hit) return hit;
		throw err;
	}
}

sw.addEventListener('fetch', (event) => {
	const { request } = event;
	if (request.method !== 'GET') return;
	const url = new URL(request.url);
	if (url.origin !== sw.location.origin) return;
	// Never cache credentialed or per-person API routes (none exist in M0; kept for M2).
	if (/^\/(v1\/me|auth)(\/|$)/.test(url.pathname) || request.headers.has('authorization')) return;

	if (SHELL_SET.has(url.pathname) || OFFICIAL_TEXT.test(url.pathname)) {
		event.respondWith(cacheFirst(request));
	} else {
		event.respondWith(networkFirst(request));
	}
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

// M0 demo: the page asks for the same notification to show what a batch looks like.
sw.addEventListener('message', (event) => {
	if (event.data?.kind === 'demo-updates') event.waitUntil(showUpdates());
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
