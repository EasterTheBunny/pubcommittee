import { error } from '@sveltejs/kit';
import { issueById, issues } from '#lib/mock/index.ts';
import type { EntryGenerator, PageLoad } from './$types';

export const entries: EntryGenerator = () => issues.map((i) => ({ issue: i.id }));

export const load: PageLoad = ({ params }) => {
	const issue = issueById(params.issue);
	if (!issue) error(404, 'Issue not found');
	return { issue };
};
