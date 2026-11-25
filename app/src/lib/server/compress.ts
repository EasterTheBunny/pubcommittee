import { Readable } from 'node:stream';
import type { ReadableStream as NodeReadableStream } from 'node:stream/web';
import { constants, createBrotliCompress, createGzip } from 'node:zlib';

// Response compression for pages and JSON (M1-16 page weight). The web host sends what the app
// returns unchanged: the edge in front of it does not compress proxied responses, so a long bill
// page went out as about 1 MB of HTML that compresses to a few percent of that.

/** Bodies with a known length below this are sent as they are: compression would not pay. */
export const MIN_BYTES = 1024;

/** Brotli quality per response: fast enough per request, still far smaller than gzip. */
const BROTLI_QUALITY = 5;

export type Encoding = 'br' | 'gzip';

const compressible =
	/^(text\/|application\/(json|problem\+json|javascript|xml|manifest\+json|ld\+json)\b)/i;

/** The encoding to use for an Accept-Encoding header: brotli, then gzip, else none. */
export function chooseEncoding(accept: string | null): Encoding | null {
	if (!accept) return null;
	const q = new Map<string, number>();
	for (const part of accept.split(',')) {
		const [name, ...params] = part.trim().toLowerCase().split(';');
		if (!name) continue;
		const qp = params.map((p) => p.trim()).find((p) => p.startsWith('q='));
		const v = qp === undefined ? 1 : Number(qp.slice(2));
		q.set(name, Number.isFinite(v) ? v : 0);
	}
	const weight = (n: string) => q.get(n) ?? q.get('*') ?? 0;
	const br = weight('br');
	const gzip = weight('gzip');
	if (br > 0 && br >= gzip) return 'br';
	if (gzip > 0) return 'gzip';
	return null;
}

/** Whether a response may be compressed: a body, a text type, not already encoded. */
function eligible(request: Request, response: Response): boolean {
	if (request.method === 'HEAD' || !response.body) return false;
	if (response.status < 200 || response.status === 204 || response.status === 206) return false;
	const h = response.headers;
	if (h.has('content-encoding') || h.has('content-range')) return false;
	if (/\bno-transform\b/i.test(h.get('cache-control') ?? '')) return false;
	if (!compressible.test(h.get('content-type') ?? '')) return false;
	const length = h.get('content-length');
	return length === null || Number(length) >= MIN_BYTES;
}

/**
 * Returns the response compressed for the request's Accept-Encoding, or unchanged. The body is
 * streamed through the encoder, so a streamed page stays streamed.
 */
export function compressResponse(request: Request, response: Response): Response {
	if (!eligible(request, response)) return response;
	const encoding = chooseEncoding(request.headers.get('accept-encoding'));
	const headers = new Headers(response.headers);
	headers.append('vary', 'Accept-Encoding');
	if (!encoding) return new Response(response.body, { status: response.status, headers });
	const encoder =
		encoding === 'br'
			? createBrotliCompress({
					params: {
						[constants.BROTLI_PARAM_QUALITY]: BROTLI_QUALITY,
						[constants.BROTLI_PARAM_MODE]: constants.BROTLI_MODE_TEXT
					}
				})
			: createGzip();
	const body = Readable.fromWeb(response.body as unknown as NodeReadableStream).pipe(encoder);
	headers.delete('content-length');
	headers.set('content-encoding', encoding);
	// A strong validator names exact bytes; the encoded bytes differ.
	const etag = headers.get('etag');
	if (etag && !etag.startsWith('W/')) headers.set('etag', 'W/' + etag);
	return new Response(Readable.toWeb(body) as unknown as ReadableStream, {
		status: response.status,
		statusText: response.statusText,
		headers
	});
}
