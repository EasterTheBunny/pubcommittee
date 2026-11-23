import { expect, test } from '@playwright/test';

test('bill page fits a phone width', async ({ page }) => {
	await page.goto('/bills/us-119-hr-4127');
	const overflow = await page.evaluate(
		() => document.documentElement.scrollWidth - document.documentElement.clientWidth
	);
	expect(overflow).toBeLessThanOrEqual(0);
	await expect(page.locator('#s4-l1')).toBeVisible();
});
