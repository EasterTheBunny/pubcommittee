import { appendFileSync } from 'node:fs';
import { expect, test, type Page } from '@playwright/test';

// M1-16 local gate, UI part: the three exit bills and the largest real bill available locally,
// read in a real browser: without scripts, as an installed-style offline reader, and for page cost.
const api = process.env.API_URL ?? 'http://read-api:8080';
const timings = process.env.TIMINGS_FILE ?? '/tmp/m1-16-timings.jsonl';
const exitBills = [
	{ id: 'us-119-hr-4313', title: 'Hospital Inpatient Services Modernization Act' },
	{ id: 'us-119-hr-3062', title: 'Promoting Cross-border Energy Infrastructure Act' },
	{ id: 'us-119-hr-3427', title: 'Water Resources Technical Assistance Review Act' }
];

type Line = { anchor: string; n: number; text: string };
async function lines(bill: string, v: number): Promise<Line[]> {
	const res = await fetch(`${api}/v1/bills/${bill}/v/${v}`);
	const j = (await res.json()) as { sections: { lines: Line[] }[] };
	return j.sections.flatMap((s) => s.lines);
}

/** Records how long a page takes to be usable, from the browser's own navigation timing. */
async function timed(page: Page, name: string, path: string) {
	await page.goto(path, { waitUntil: 'load' });
	const t = await page.evaluate(() => {
		const n = performance.getEntriesByType('navigation')[0] as PerformanceNavigationTiming;
		return {
			ttfb: Math.round(n.responseStart),
			response_end: Math.round(n.responseEnd),
			dom_content_loaded: Math.round(n.domContentLoadedEventEnd),
			load: Math.round(n.loadEventEnd),
			bytes: n.encodedBodySize
		};
	});
	appendFileSync(timings, JSON.stringify({ name, path, ...t }) + '\n');
	return t;
}

test.describe('without scripts', () => {
	test.use({ javaScriptEnabled: false });
	test('every part of every explanation is reachable without scripts', async ({ page }) => {
		await page.goto('/bills/us-119-hr-4313/v/2');
		const asides = page.locator('[data-bill-text] aside[data-explanation]');
		const total = await asides.count();
		expect(total).toBeGreaterThan(0);
		for (const i of [0, total - 1]) {
			const ex = asides.nth(i);
			await expect(ex.getByText('Auto-generated').first()).toBeVisible();
			const ctl = ex.locator('summary');
			for (let k = 0; k < (await ctl.count()); k++) {
				await ctl.nth(k).click();
				await expect(ex.locator('details[open] .ex-body')).toBeVisible();
				await expect(ex.locator('details[open]')).toHaveCount(1);
			}
		}
	});
	for (const b of exitBills) {
		test(`${b.id}: find in the catalog, read every line of the latest version, follow a line address`, async ({ page }) => {
			await page.goto(`/catalog?q=${encodeURIComponent(b.title.split(' ').slice(0, 2).join(' '))}`);
			await page.getByRole('link', { name: b.title }).first().click();
			await expect(page.getByRole('heading', { name: b.title, level: 1 })).toBeVisible();
			const expected = await lines(b.id, 4);
			await expect(page.locator('[data-bill-text] [data-line]')).toHaveCount(expected.length);
			// the first and the last line of the official text are on the page as anchors
			for (const l of [expected[0], expected.at(-1)!]) {
				await expect(page.locator(`#${l.anchor}`)).toContainText(l.text.slice(0, 40));
			}
			await expect(page.getByText('Explanations are labelled').first()).toBeVisible();
			await page.locator(`#${expected[1].anchor}`).getByRole('link', { name: /Address of line/ }).click();
			await expect(page).toHaveURL(new RegExp(`/bills/${b.id}/v/4/s/[^/]+/l/${expected[1].n}$`));
			await expect(page.locator('[data-line]')).toHaveCount(1);
		});
	}
	test('an explanation is labelled auto-generated and every statement points at lines', async ({ page }) => {
		await page.goto('/bills/us-119-hr-4313/v/2');
		await expect(page.getByText('Auto-generated').first()).toBeVisible();
		await expect(page.locator('[data-content-class="platform"]').first()).toBeVisible();
		await expect(page.locator('[data-bill-text] [data-content-class="official"]').first()).toBeVisible();
	});
	test('explanations are pinned above the line they describe; the controls open, close and switch', async ({ page }) => {
		await page.goto('/bills/us-119-hr-4313/v/2');
		const asides = page.locator('[data-bill-text] aside[data-explanation]');
		expect(await asides.count()).toBeGreaterThan(0);
		for (const i of [0, (await asides.count()) - 1]) {
			const ex = asides.nth(i);
			const [from] = (await ex.getAttribute('data-lines'))!.split('-');
			// the list of lines that follows it starts with the first line it describes
			await expect(ex.locator('+ ol > li').first()).toHaveAttribute('id', new RegExp(`-l${from}$`));
			await expect(ex.getByText('Auto-generated').first()).toBeVisible();
			await expect(ex).not.toContainText('usc/');
			const ctl = ex.locator('summary');
			const body = ex.locator('details[open] .ex-body');
			await ctl.first().click();
			await expect(body).toBeVisible();
			await ctl.first().click();
			await expect(body).toBeHidden();
			if ((await ctl.count()) > 1) {
				await ctl.first().click();
				await ctl.nth(1).click();
				await expect(ex.locator('details[open]')).toHaveCount(1);
				await expect(ex.locator('details[open] summary')).toHaveText((await ctl.nth(1).textContent())!);
			}
		}
	});
	test('a section that only gives the short title shows no explanation', async ({ page }) => {
		for (const b of exitBills) {
			const res = await fetch(`${api}/v1/bills/${b.id}/v/1`);
			const j = (await res.json()) as { sections: { id: string; lines: Line[] }[] };
			await page.goto(`/bills/${b.id}/v/1`);
			for (const sec of j.sections.filter((x) => x.lines.every((l) => / may be cited as /i.test(l.text)))) {
				await expect(page.locator(`#${sec.id}`)).toBeVisible();
				await expect(page.locator(`#${sec.id} aside[data-explanation]`)).toHaveCount(0);
			}
		}
	});
	test('version history: the older version says what changed since; every version is a link', async ({ page }) => {
		await page.goto('/bills/us-119-hr-4313/v/1/s/s2/l/2');
		await expect(page.locator('[data-notice]')).toBeVisible();
		await page.goto('/bills/us-119-hr-4313/v/1');
		for (const n of [1, 2, 3, 4]) {
			await expect(page.getByRole("navigation", { name: "Versions" }).locator(`a[href$="/v/${n}"]`).first()).toBeVisible();
		}
	});
	test('catalog: markup status, official links, selection policy and public log, all server-rendered', async ({ page }) => {
		await page.goto('/catalog');
		await expect(page.getByText('Eligible for review').first()).toBeVisible();
		await expect(page.getByRole('link', { name: 'Congress.gov' }).first()).toBeVisible();
		await page.goto('/policy');
		await expect(page.getByText(/Policy version \d+/)).toBeVisible();
		await page.goto('/log');
		await expect(page.getByText(/Reason:/).first()).toBeVisible();
		const notice = await page.request.get('/.well-known/machine-access.json');
		expect(notice.ok()).toBe(true);
		expect((await page.request.get('/robots.txt')).ok()).toBe(true);
	});
});

test.describe('installable and offline, on real bills', () => {
	const path = '/bills/us-119-hr-4313/v/2';
	async function openUnderWorker(page: Page, p: string) {
		await page.goto(p);
		await page.evaluate(() => navigator.serviceWorker.ready);
		await page.reload();
		await expect.poll(() => page.evaluate(() => !!navigator.serviceWorker.controller)).toBe(true);
	}
	const saved = (page: Page, p: string) =>
		expect
			.poll(() => page.evaluate((q) => caches.match(new URL(q, location.href).href, { ignoreVary: true }).then(Boolean), p))
			.toBe(true);

	test('the browser finds no installability errors', async ({ page, context }) => {
		await openUnderWorker(page, '/');
		const cdp = await context.newCDPSession(page);
		const { installabilityErrors } = await cdp.send('Page.getInstallabilityErrors');
		expect(installabilityErrors).toEqual([]);
	});
	test('an opened real version is readable offline with its official text', async ({ page, context }) => {
		const expected = await lines('us-119-hr-4313', 2);
		await openUnderWorker(page, path);
		await saved(page, path);
		await context.setOffline(true);
		await page.reload();
		await expect(page.locator('[data-bill-text] [data-line]')).toHaveCount(expected.length);
		await expect(page.locator(`#${expected[1].anchor}`)).toContainText(expected[1].text.slice(0, 40));
		await expect(page.locator('[data-offline-label]')).toBeVisible();
		await context.setOffline(false);
	});
	test('the catalog opened before shows the out-of-date label offline', async ({ page, context }) => {
		await openUnderWorker(page, '/catalog');
		await saved(page, '/catalog');
		await context.setOffline(true);
		await page.reload();
		await expect(page.getByText('Possibly out of date.')).toBeVisible();
		await context.setOffline(false);
	});
});

test.describe('page cost on real bills', () => {
	test('exit bill pages and a canonical range load promptly', async ({ page }) => {
		for (const b of exitBills) {
			const t = await timed(page, `${b.id} v4 page`, `/bills/${b.id}/v/4`);
			expect(t.load).toBeLessThan(5000);
		}
		const t = await timed(page, 'hr-4313 v2 section 3 lines 12-20', '/bills/us-119-hr-4313/v/2/s/s3/l/12-20');
		expect(t.load).toBeLessThan(5000);
		await timed(page, 'catalog', '/catalog');
	});

	// M1-13 deferred this: a 10,000-line stub only. hr-3115 is the largest real bill of the corpus.
	test('the longest real bill (hr-3115, thousands of lines) renders, scrolls to a deep anchor, and stays responsive', async ({ page }) => {
		const all = await lines('us-119-hr-3115', 1);
		expect(all.length).toBeGreaterThan(2000);
		const t = await timed(page, `us-119-hr-3115 v1 page (${all.length} lines)`, '/bills/us-119-hr-3115/v/1');
		expect(t.load).toBeLessThan(10_000);
		await expect(page.locator('[data-bill-text] [data-line]')).toHaveCount(all.length);
		const last = all.at(-1)!;
		await page.goto(`/bills/us-119-hr-3115/v/1#${last.anchor}`);
		await expect(page.locator(`#${last.anchor}`)).toBeInViewport();
		// still interactive after hydration: with scripts a line-address click moves to the line in place
		const mid = all[Math.floor(all.length / 2)];
		const start = Date.now();
		await page.locator(`#${mid.anchor}`).getByRole('link', { name: /Address of line/ }).click();
		await expect(page).toHaveURL(new RegExp(`#${mid.anchor}$`));
		await expect(page.locator(`#${mid.anchor}`)).toBeInViewport();
		appendFileSync(timings, JSON.stringify({ name: 'long page: click a line address', ms: Date.now() - start }) + '\n');
	});
	test('a long range on the largest bill is a short page, not the whole bill', async ({ page }) => {
		const t = await timed(page, 'us-119-hr-3115 v1 s2 lines 2-51', '/bills/us-119-hr-3115/v/1/s/s2/l/2-51');
		expect(t.load).toBeLessThan(5000);
		expect(await page.locator('[data-line]').count()).toBe(50);
	});
});
