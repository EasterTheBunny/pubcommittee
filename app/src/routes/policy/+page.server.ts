import { error } from '@sveltejs/kit';
import { readApi } from '#lib/server/api.ts';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, setHeaders }) => {
	const res = await readApi.getSelectionPolicy(fetch);
	if (!res.ok) error(res.status, 'The selection policy is not available right now.');
	setHeaders({ 'cache-control': 'public, max-age=60' });
	return { policy: res.data };
};
