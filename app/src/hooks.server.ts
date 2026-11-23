import type { Handle } from '@sveltejs/kit/hooks';
import { apiPathFor, prefersJson } from '#lib/addresses.ts';
import { publicApiUrl } from '#lib/server/api.ts';

// Content negotiation (design/10 Machine-readable forms): a page asked for as JSON answers
// 303 See Other to the read-api resource that holds the same data. Pages vary on Accept.
export const handle: Handle = async ({ event, resolve }) => {
	const { request, url } = event;
	if (request.method === 'GET' || request.method === 'HEAD') {
		const path = apiPathFor(url);
		if (path && prefersJson(request.headers.get('accept'))) {
			return new Response(null, {
				status: 303,
				headers: { location: publicApiUrl() + path, vary: 'Accept' }
			});
		}
	}
	const response = await resolve(event);
	if (apiPathFor(url)) response.headers.append('vary', 'Accept');
	return response;
};
