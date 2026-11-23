import { expect, test } from '@playwright/test';

const bill = '/bills/us-119-hr-4127';

test('bill page shows the latest version with numbered lines and two distinct layers', async ({
	page
}) => {
	await page.goto(bill);
	await expect(page.getByRole('heading', { name: 'Example Clinic Act', level: 1 })).toBeVisible();
	await expect(page.locator('#s4-l1')).toContainText('The Secretary shall publish clinic hours');
	await expect(page.locator('#s4-l2')).toHaveAttribute('data-depth', '1');
	// explanation layer: separate element, with provenance label and the unofficial preview label
	const aside = page.getByRole('complementary', { name: /^Explanation of lines 4-6/ });
	await expect(aside.getByText('Auto-generated').first()).toBeVisible();
	await aside.getByText('Preview as amended').click();
	await expect(aside.getByText('Unofficial')).toBeVisible();
	await expect(page.locator('[data-bill-text] [data-content-class="platform"]')).not.toHaveCount(0);
	// summaries not published yet: designed empty state, no invented text
	await expect(page.getByText('No summary is published for this version yet.')).toBeVisible();
});

test('version switcher opens an older version with a notice', async ({ page }) => {
	await page.goto(bill);
	await page.getByRole('navigation', { name: 'Versions' }).getByRole('link').first().click();
	await expect(page).toHaveURL(/\/v\/1$/);
	await expect(page.getByText('You are reading an older version.')).toBeVisible();
	await page.getByRole('link', { name: 'See the current version' }).click();
	await expect(page).toHaveURL(/\/v\/2$/);
});

test('every page links to its read-api resource', async ({ page }) => {
	await page.goto(bill);
	await expect(page.locator('link[rel="alternate"][type="application/json"]')).toHaveAttribute(
		'href',
		/\/v1\/bills\/us-119-hr-4127$/
	);
});

test('Accept: application/json answers 303 to the read-api resource', async ({ request }) => {
	const res = await request.get(bill + '/v/1/s/s4/l/1-2', {
		headers: { accept: 'application/json' },
		maxRedirects: 0
	});
	expect(res.status()).toBe(303);
	expect(res.headers()['location']).toMatch(/\/v1\/bills\/us-119-hr-4127\/v\/1\/s\/s4\/l\/1-2$/);
	const html = await request.get(bill, { headers: { accept: 'text/html' }, maxRedirects: 0 });
	expect(html.status()).toBe(200);
});

test('a line range address highlights its lines and a fragment selects them', async ({ page }) => {
	await page.goto(bill + '/v/2/s/s4/l/2-3');
	await expect(page.getByRole('heading', { level: 1 })).toContainText('lines 2–3');
	await expect(page.locator('#s4-l2')).toHaveClass(/ln-on/);
	// a range address shows only its own lines
	await expect(page.locator('#s4-l1')).toHaveCount(0);
	await page.goto(bill + '/v/2#s4-l2-3');
	await expect(page.getByRole('region', { name: 'Selected lines' })).toContainText('lines 2–3');
});
