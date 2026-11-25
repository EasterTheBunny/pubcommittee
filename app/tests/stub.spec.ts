import { expect, test } from '@playwright/test';

// These run against the app wired to tests/stub-api.mjs (STUB_URL), which serves a comparison and
// a very long bill. Skipped when STUB_URL is not set.
const stub = process.env.STUB_URL;
test.skip(!stub, 'STUB_URL not set');
test.use({ baseURL: stub });

test('old-version address to changed words shows the notice and a one-tap link to the current version', async ({
	page
}) => {
	await page.goto('/bills/us-119-hr-1/v/1/s/s2/l/2');
	const notice = page.getByRole('status');
	await expect(notice).toContainText('These words changed in a newer version.');
	await notice.getByRole('link', { name: 'See the current version' }).click();
	await expect(page).toHaveURL(/\/bills\/us-119-hr-1\/s\/s2\/l\/2$/);
});

test('old-version address to unchanged words says so', async ({ page }) => {
	await page.goto('/bills/us-119-hr-1/v/1/s/s2/l/1');
	await expect(page.getByRole('status')).toContainText('unchanged in version 2');
	await page.goto('/bills/us-119-hr-1/v/1/s/s1');
	await expect(page.getByRole('status')).toContainText('unchanged in version 2');
});

test('the current version has no notice', async ({ page }) => {
	await page.goto('/bills/us-119-hr-1/v/2/s/s2/l/2');
	await expect(page.getByRole('status')).toHaveCount(0);
});

test('a very long bill renders every line, fast, without horizontal overflow', async ({ page }) => {
	const t0 = Date.now();
	const res = await page.goto('/bills/us-119-hr-2');
	const ms = Date.now() - t0;
	expect(res?.status()).toBe(200);
	await expect(page.locator('[data-bill-text] [data-line]')).toHaveCount(10000);
	// server time to first full HTML; generous bound for a shared CI container
	expect(ms).toBeLessThan(5000);
	const overflow = await page.evaluate(
		() => document.documentElement.scrollWidth - document.documentElement.clientWidth
	);
	expect(overflow).toBeLessThanOrEqual(0);
});

test('a generated summary carries the accuracy note with a link to the bill text', async ({
	page
}) => {
	await page.goto('/bills/us-119-hr-1/v/2');
	const summary = page.locator('section', {
		has: page.getByRole('heading', { name: 'Bill summary' })
	});
	const note = summary.locator('[data-accuracy-note]');
	await expect(note).toHaveText(
		'Auto-generated; tested accuracy 75–85%. Verify with the bill text.'
	);
	await expect(note.getByRole('link', { name: 'bill text' })).toHaveAttribute('href', '#bill-text');
	await expect(page.locator('#bill-text')).toHaveCount(1);
});

test('a version with no text changes says so, without the accuracy note', async ({ page }) => {
	await page.goto('/bills/us-119-hr-3/v/2');
	const changes = page.locator('section', {
		has: page.getByRole('heading', { name: 'What changed from the previous version' })
	});
	await expect(changes).toContainText('No text changes from version 1.');
	await expect(changes).toContainText('From the version comparison');
	await expect(changes.locator('[data-accuracy-note]')).toHaveCount(0);
});

test('a long bill page is sent compressed', async ({ request }) => {
	const res = await request.get('/bills/us-119-hr-2', {
		headers: { 'accept-encoding': 'br, gzip' }
	});
	expect(res.status()).toBe(200);
	expect(res.headers()['content-encoding']).toBe('br');
	expect(res.headers()['vary']).toContain('Accept-Encoding');
	expect((await res.text()).length).toBeGreaterThan(1_000_000);
});
