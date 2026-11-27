import { expect, test } from '@playwright/test';
import {
	FALLBACK_LIMIT,
	TEXT_LIMIT,
	cacheNames,
	classify,
	isImmutable,
	storable,
	touchFallback,
	touchText,
	versionUnit,
	type TextEntry
} from '../src/service-worker/policy.ts';

// The service worker's rules, run in Node without a browser (M1-14).
const self = 'http://localhost:3000';
const shell = new Set(['/_app/immutable/a.js', '/manifest.webmanifest']);
const kind = (path: string, over: { method?: string; origin?: string; auth?: boolean } = {}) =>
	classify({
		method: over.method ?? 'GET',
		url: new URL(path, over.origin ?? self),
		selfOrigin: self,
		hasAuthorization: over.auth ?? false,
		shell
	});

test('resource classes', () => {
	expect(kind('/_app/immutable/a.js')).toBe('shell');
	expect(kind('/bills/us-119-hr-4127/v/eh')).toBe('official');
	expect(kind('/bills/us-119-hr-4127/v/eh/s/s4/l/1-3')).toBe('official');
	expect(kind('/bills/us-119-hr-4127/v/eh/s/s4/__data.json')).toBe('official');
	expect(kind('/bills/us-119-hr-4127')).toBe('dynamic');
	expect(kind('/bills/us-119-hr-4127/s/s4')).toBe('dynamic');
	for (const p of ['/', '/catalog', '/policy', '/log']) expect(kind(p)).toBe('dynamic');
});

test('writes, credentials, and other origins are never handled', () => {
	expect(kind('/catalog', { method: 'POST' })).toBe('skip');
	expect(kind('/bills/x/v/ih', { method: 'PUT' })).toBe('skip');
	expect(kind('/catalog', { auth: true })).toBe('skip');
	expect(kind('/v1/me/inbox')).toBe('skip');
	expect(kind('/auth/session')).toBe('skip');
	expect(kind('/v1/bills', { origin: 'https://api.example.test' })).toBe('skip');
	expect(kind('/v1/bills/x/v/eh/s/s1', { origin: 'https://api.example.test' })).toBe('official');
	expect(kind('/v1/me/inbox', { origin: 'https://api.example.test' })).toBe('skip');
});

test('version units group a version and its resources', () => {
	expect(versionUnit('/bills/b/v/eh/s/s4/l/1', true)).toBe('/bills/b/v/eh');
	expect(versionUnit('/v1/bills/b/v/eh/s/s4', false)).toBe('/bills/b/v/eh');
	expect(versionUnit('/bills/b', true)).toBeNull();
});

test('Cache-Control is respected', () => {
	expect(storable(200, 'public, max-age=60', 'Accept', false)).toBe(true);
	expect(storable(200, 'no-store', null, false)).toBe(false);
	expect(storable(200, 'private, max-age=60', null, false)).toBe(false);
	expect(storable(200, null, '*', false)).toBe(false);
	expect(storable(200, null, null, true)).toBe(false);
	expect(storable(404, null, null, false)).toBe(false);
	expect(isImmutable('public, max-age=31536000, immutable')).toBe(true);
	expect(isImmutable('public, max-age=60, stale-while-revalidate=300')).toBe(false);
});

test('the 50 most recently opened versions stay; the least recent goes first', () => {
	let index: TextEntry[] = [];
	for (let i = 1; i <= TEXT_LIMIT; i++) {
		index = touchText(index, `/bills/b/v/${i}`, `/bills/b/v/${i}`, i).index;
	}
	expect(index).toHaveLength(TEXT_LIMIT);
	// reopening version 1 makes version 2 the least recent
	index = touchText(index, '/bills/b/v/1', '/bills/b/v/1/s/s1', 100).index;
	const over = touchText(index, '/bills/b/v/51', '/bills/b/v/51', 101);
	expect(over.index).toHaveLength(TEXT_LIMIT);
	expect(over.evict).toEqual(['/bills/b/v/2']);
	expect(over.index.some((e) => e.unit === '/bills/b/v/1')).toBe(true);
});

test('evicting a version drops all its resources', () => {
	let r = touchText([], '/bills/b/v/1', '/bills/b/v/1', 1, 1);
	r = touchText(r.index, '/bills/b/v/1', '/bills/b/v/1/s/s4', 2, 1);
	const next = touchText(r.index, '/bills/b/v/2', '/bills/b/v/2', 3, 1);
	expect(next.evict).toEqual(['/bills/b/v/1', '/bills/b/v/1/s/s4']);
});

test('fallback copies are bounded, oldest first', () => {
	let list: { url: string; at: number }[] = [];
	let evicted: string[] = [];
	for (let i = 0; i <= FALLBACK_LIMIT; i++) {
		const r = touchFallback(list, `/catalog?p=${i}`, i);
		list = r.list;
		evicted = r.evict;
	}
	expect(list).toHaveLength(FALLBACK_LIMIT);
	expect(evicted).toEqual(['/catalog?p=0']);
});

test('cache names carry the release, so a new release replaces them', () => {
	const a = Object.values(cacheNames('r1'));
	const b = Object.values(cacheNames('r2'));
	expect(a.filter((n) => b.includes(n))).toEqual([]);
});
