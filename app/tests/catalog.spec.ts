import { expect, test } from '@playwright/test';

test('catalog lists tracked and untracked entries with official links', async ({ page }) => {
	await page.goto('/catalog');
	await expect(page.getByRole('heading', { name: 'Catalog', level: 1 })).toBeVisible();
	const tracked = page.getByRole('article').filter({ hasText: 'Example Clinic Act' });
	await expect(tracked.getByRole('link', { name: 'Example Clinic Act' })).toBeVisible();
	await expect(tracked.getByText('In review').first()).toBeVisible();
	const untracked = page.getByRole('article').filter({ hasText: 'Example Post Office Naming Act' });
	await expect(untracked.getByRole('link', { name: 'Congress.gov' })).toHaveAttribute(
		'href',
		/example\.test/
	);
	await expect(untracked.getByText('Not a candidate').first()).toBeVisible();
	await expect(untracked.getByRole('link', { name: 'Example Post Office Naming Act' })).toHaveCount(
		0
	);
	await expect(page.getByText('Coverage on this page')).toBeVisible();
});

test('catalog search filters entries', async ({ page }) => {
	await page.goto('/catalog');
	await page.getByLabel('Search by number or title').fill('naming');
	await page.getByRole('button', { name: 'Search' }).click();
	await expect(page).toHaveURL(/q=naming/);
	await expect(page.getByRole('article')).toHaveCount(1);
});

test('an untracked bill page shows no text', async ({ page }) => {
	await page.goto('/bills/us-119-hr-9');
	await expect(page.getByText('does not show its text')).toBeVisible();
	await expect(page.locator('[data-line]')).toHaveCount(0);
});
