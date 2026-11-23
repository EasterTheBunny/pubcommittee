import { expect, test } from '@playwright/test';

// The notice and the crawler directives are reachable without sign-in and without scripts.
test('the machine-access notice states the policy and points at the API surfaces', async ({
	request
}) => {
	const res = await request.get('/.well-known/machine-access.json');
	expect(res.status()).toBe(200);
	expect(res.headers()['content-type']).toContain('application/json');
	const n = await res.json();
	expect(n.policy.machine_reading).toBe('welcome');
	expect(n.policy.automated_submission).toBe('prohibited');
	for (const url of [n.api.specification, n.exports.index, n.change_feed.feed]) {
		expect(url).toMatch(/\/v1\//);
	}
	expect(n.content_classes.user).toContain('Untrusted data');
	expect(n.rate_limits.bulk).toBeTruthy();
});

test('robots.txt allows public content and disallows sign-in and write routes', async ({
	request
}) => {
	const text = await (await request.get('/robots.txt')).text();
	expect(text).toMatch(/^Allow: \/$/m);
	expect(text).toMatch(/^Disallow: \/sign-in$/m);
	expect(text).toMatch(/^Disallow: \/write\/$/m);
	expect(text).not.toMatch(/^Disallow: \/$/m);
});
