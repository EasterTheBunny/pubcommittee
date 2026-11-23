import { error } from '@sveltejs/kit';
import { readApi } from '#lib/server/api.ts';
import type { LogEntry } from '#lib/types.ts';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, url, setHeaders }) => {
	const cursor = url.searchParams.get('cursor') ?? undefined;
	const res = await readApi.listModerationLog(fetch, cursor);
	if (!res.ok) error(res.status, 'The public log is not available right now.');
	setHeaders({ 'cache-control': 'public, max-age=60' });
	return {
		entries: (res.data.items ?? []) as LogEntry[],
		nextHref: res.data.next ? '?cursor=' + encodeURIComponent(res.data.next) : null
	};
};
