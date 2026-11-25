import { expect, test } from '@playwright/test';
import { brotliDecompressSync, gunzipSync } from 'node:zlib';
import { MIN_BYTES, chooseEncoding, compressResponse } from '../src/lib/server/compress.ts';

// Unit tests of the response compression in hooks.server.ts (M1-16 page weight).

const page = '<p>' + 'Line of bill text. '.repeat(200) + '</p>';
const req = (accept: string | null, method = 'GET') =>
	new Request('http://app.test/bills/x', {
		method,
		headers: accept === null ? {} : { 'accept-encoding': accept }
	});
const html = (body: string | null, extra: Record<string, string> = {}, status = 200) =>
	new Response(body, { status, headers: { 'content-type': 'text/html', ...extra } });
const bytes = async (r: Response) => Buffer.from(await r.arrayBuffer());

test('chooses brotli, then gzip, honouring q values', () => {
	expect(chooseEncoding('gzip, deflate, br')).toBe('br');
	expect(chooseEncoding('gzip')).toBe('gzip');
	expect(chooseEncoding('br;q=0, gzip')).toBe('gzip');
	expect(chooseEncoding('br;q=0.5, gzip;q=1')).toBe('gzip');
	expect(chooseEncoding('*')).toBe('br');
	expect(chooseEncoding('identity')).toBeNull();
	expect(chooseEncoding(null)).toBeNull();
});

test('a page goes out brotli- or gzip-encoded and decodes to the same text', async () => {
	const br = compressResponse(req('gzip, br'), html(page));
	expect(br.headers.get('content-encoding')).toBe('br');
	expect(br.headers.get('vary')).toContain('Accept-Encoding');
	expect(br.headers.get('content-length')).toBeNull();
	expect(brotliDecompressSync(await bytes(br)).toString()).toBe(page);

	const gz = compressResponse(req('gzip'), html(page, { etag: '"abc"' }));
	expect(gz.headers.get('content-encoding')).toBe('gzip');
	expect(gz.headers.get('etag')).toBe('W/"abc"');
	expect(gunzipSync(await bytes(gz)).toString()).toBe(page);
});

test('small, binary, encoded, empty, and no-transform responses are left alone', async () => {
	const small = 'x'.repeat(MIN_BYTES - 1);
	const cases = [
		html(small, { 'content-length': String(small.length) }),
		new Response(page, { headers: { 'content-type': 'image/png' } }),
		html(page, { 'content-encoding': 'gzip' }),
		html(page, { 'cache-control': 'public, no-transform' }),
		html(null, {}, 304)
	];
	for (const r of cases) {
		const out = compressResponse(req('br'), r);
		expect(out).toBe(r);
	}
	expect(compressResponse(req('br', 'HEAD'), html(null))).toBeInstanceOf(Response);
});

test('without an accepted encoding the body is unchanged but varies on Accept-Encoding', async () => {
	const out = compressResponse(req('identity'), html(page));
	expect(out.headers.get('content-encoding')).toBeNull();
	expect(out.headers.get('vary')).toContain('Accept-Encoding');
	expect((await bytes(out)).toString()).toBe(page);
});
