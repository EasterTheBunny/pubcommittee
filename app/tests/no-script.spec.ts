import { expect, test } from '@playwright/test';

test.use({ javaScriptEnabled: false });

test('without scripts: browse the catalog, open a bill, read every line, follow a range address', async ({
	page
}) => {
	await page.goto('/catalog');
	await page.getByRole('link', { name: 'Example Clinic Act' }).click();
	await expect(page.getByRole('heading', { name: 'Example Clinic Act', level: 1 })).toBeVisible();
	const lines = page.locator('[data-bill-text] [data-line]');
	await expect(lines.first()).toBeVisible();
	expect(await lines.count()).toBeGreaterThan(3);
	await expect(page.locator('#s4-l3')).toContainText('weekend and holiday hours');
	// the per-line address link is a plain link to the canonical range page
	await page
		.locator('#s4-l2')
		.getByRole('link', { name: /Address of line 2/ })
		.click();
	await expect(page).toHaveURL(/\/v\/2\/s\/s4\/l\/2$/);
	await expect(page.locator('#s4-l2')).toContainText('weekday hours');
	await page.goto('/bills/us-119-hr-4127/v/2/s/s4/l/1-3');
	await expect(page.locator('[data-line]')).toHaveCount(3);
	// no content is gated on scripts: the version switcher and sections nav are plain links
	await expect(page.getByRole('link', { name: 'The whole section' })).toBeVisible();
});

test('without scripts: placeholders open natively and name their milestone', async ({ page }) => {
	await page.goto('/');
	await page.getByText('Sign in').click();
	await expect(page.getByText(/Coming in M2/)).toBeVisible();
});
