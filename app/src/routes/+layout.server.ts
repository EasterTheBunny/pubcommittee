import { apiPathFor } from '#lib/addresses.ts';
import { publicApiUrl } from '#lib/server/api.ts';
import type { LayoutServerLoad } from './$types';

// Every page links to its read-api resource with rel="alternate" (design/10 Machine-readable forms).
export const load: LayoutServerLoad = ({ url }) => {
	const path = apiPathFor(url);
	return { alternate: path ? publicApiUrl() + path : null };
};
