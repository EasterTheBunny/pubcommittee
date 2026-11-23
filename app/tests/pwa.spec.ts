import { expect, test, type Page } from '@playwright/test';

// Installability and offline reading (M1-14). `localhost` is a secure context, so the service
// worker runs without HTTPS. Desktop Chromium only: CDP reports the browser's own install check.
test.skip(({ browserName }) => browserName !== 'chromium');

const version = '/bills/us-119-hr-4127/v/2';
const line = 'The Secretary shall publish clinic hours';

/** Opens `path`, waits until the service worker controls the page, then loads it again under control. */
async function openUnderWorker(page: Page, path: string) {
	await page.goto(path);
	await page.evaluate(() => navigator.serviceWorker.ready);
	await page.reload();
	await expect.poll(() => page.evaluate(() => !!navigator.serviceWorker.controller)).toBe(true);
}

/** Waits until a cache holds `path`: saving runs after the response is delivered. */
const saved = (page: Page, path: string) =>
	expect
		.poll(() =>
			page.evaluate(
				(p) => caches.match(new URL(p, location.href).href, { ignoreVary: true }).then(Boolean),
				path
			)
		)
		.toBe(true);

test('manifest and icons meet the install requirements', async ({ page, request }) => {
	await page.goto('/');
	const href = await page.locator('link[rel="manifest"]').getAttribute('href');
	expect(href).toBeTruthy();
	const res = await request.get(new URL(href!, page.url()).href);
	expect(res.ok()).toBe(true);
	const m = await res.json();
	expect(m).toMatchObject({ display: 'standalone', start_url: '/', scope: '/', id: '/' });
	expect(m.name).toBeTruthy();
	expect(m.short_name).toBeTruthy();
	const sizes = (m.icons as { sizes: string; purpose?: string; src: string }[]).map((i) => i.sizes);
	expect(sizes).toEqual(expect.arrayContaining(['192x192', '512x512']));
	expect(m.icons.some((i: { purpose?: string }) => i.purpose === 'maskable')).toBe(true);
	for (const icon of m.icons as { src: string }[]) {
		expect((await request.get(new URL(icon.src, page.url()).href)).ok(), icon.src).toBe(true);
	}
	expect(JSON.stringify(m)).not.toMatch(/mock|M0/);
});

test('the browser finds no installability errors and a worker controls the page', async ({
	page,
	context
}) => {
	await openUnderWorker(page, '/');
	const cdp = await context.newCDPSession(page);
	const { installabilityErrors } = await cdp.send('Page.getInstallabilityErrors');
	expect(installabilityErrors).toEqual([]);
	const { errors } = await cdp.send('Page.getAppManifest');
	expect(errors.filter((e) => e.critical)).toEqual([]);
});

test('a version page opened online opens offline with full text', async ({ page, context }) => {
	await openUnderWorker(page, version);
	await saved(page, version);
	await context.setOffline(true);
	await page.reload();
	await expect(page.locator('#s4-l1')).toContainText(line);
	await expect(page.getByRole('heading', { name: 'Example Clinic Act', level: 1 })).toBeVisible();
	// exact official text, but the page says other details may be out of date
	await expect(page.locator('[data-offline-label]')).toBeVisible();
	await context.setOffline(false);
});

test('a section address of an opened version also opens offline', async ({ page, context }) => {
	const section = `${version}/s/s4`;
	await openUnderWorker(page, section);
	await saved(page, section);
	await context.setOffline(true);
	await page.reload();
	await expect(page.locator('#s4-l1')).toContainText(line);
	await context.setOffline(false);
});

test('an offline catalog view shows the out-of-date label', async ({ page, context }) => {
	await openUnderWorker(page, '/catalog');
	await expect(page.locator('[data-offline-label]')).toHaveCount(0);
	await saved(page, '/catalog');
	await context.setOffline(true);
	await page.reload();
	await expect(page.getByText('Possibly out of date.')).toBeVisible();
	await expect(page.getByText('This is a copy saved')).toBeVisible();
	await context.setOffline(false);
});

test('a page never opened shows the offline page, not a browser error', async ({
	page,
	context
}) => {
	await openUnderWorker(page, '/catalog');
	await context.setOffline(true);
	await page.goto('/policy?never-opened=1').catch(() => undefined);
	await expect(page.locator('[data-offline-page]')).toBeVisible();
	await context.setOffline(false);
});

test('credentialed routes are never cached', async ({ page }) => {
	await openUnderWorker(page, '/');
	await page.evaluate(async () => {
		await fetch('/v1/me/inbox').catch(() => undefined);
		await fetch('/auth/session', { headers: { authorization: 'Bearer fake' } }).catch(
			() => undefined
		);
		await fetch('/catalog', { method: 'POST' }).catch(() => undefined);
	});
	const keys = await page.evaluate(async () => {
		const out: string[] = [];
		for (const name of await caches.keys()) {
			for (const req of await (await caches.open(name)).keys()) out.push(new URL(req.url).pathname);
		}
		return out;
	});
	expect(keys.filter((k) => /^\/(v1\/me|auth)/.test(k))).toEqual([]);
});

test('caches are named per release', async ({ page }) => {
	await openUnderWorker(page, '/');
	const names = await page.evaluate(() => caches.keys());
	expect(names.filter((n) => n.startsWith('pubcommittee-')).length).toBeGreaterThanOrEqual(2);
	expect(names.every((n) => /^pubcommittee-(shell|text|fallback)-.+/.test(n))).toBe(true);
});
