import { expect, test, type Locator } from '@playwright/test';

// An explanation is pinned above the line it describes: a row of controls (what changes, why it matters,
// existing law cited), and one open block below it that pushes the bill text down. Fixture data: the
// explanation of s4 in version 2 of us-119-hr-4127 rests on lines 4-6 and changes 42 U.S.C. 1395.
const page2 = '/bills/us-119-hr-4127/v/2';
const aside = '#s4 aside[data-explanation]';

// Top of an element on the page, not the viewport: the page may scroll when a control is clicked.
const top = (loc: Locator) => loc.evaluate((el) => el.getBoundingClientRect().top + scrollY);

test('the explanation sits directly above the line it describes and names the provision', async ({
	page
}) => {
	await page.goto(page2);
	const ex = page.locator(aside);
	await expect(ex).toHaveCount(1);
	await expect(ex).toHaveAttribute('data-lines', '4-6');
	// the list that follows the explanation starts at line 4; the one before it ends at line 3
	await expect(ex.locator('+ ol > li').first()).toHaveAttribute('id', 's4-l4');
	await expect(ex.locator('xpath=preceding-sibling::ol[1]/li[last()]')).toHaveAttribute(
		'id',
		's4-l3'
	);
	await expect(ex).toContainText('Explains lines 4-6');
	// readable provision, never the internal id
	await expect(ex).toContainText('42 U.S.C. 1395');
	await expect(ex).not.toContainText('usc/42/1395');
	// provenance stays visible with every part closed
	await expect(ex.getByText('Auto-generated').first()).toBeVisible();
	await expect(ex.locator('details[open]')).toHaveCount(0);
	// small-print accuracy note, linking to the first line it explains
	const note = ex.locator('[data-accuracy-note]');
	await expect(note).toHaveText(
		'Auto-generated; tested accuracy 75–85%. Verify with the bill text.'
	);
	await expect(note.getByRole('link', { name: 'bill text' })).toHaveAttribute('href', '#s4-l4');
});

test('one control per part that has content, styled as text controls', async ({ page }) => {
	await page.goto(page2);
	const ctl = page.locator(aside + ' summary');
	await expect(ctl).toHaveText([
		/What changes/,
		/Why it matters/,
		/Existing law cited/,
		/Preview as amended\s*Unofficial/
	]);
});

test('a control opens its block below the controls; the bill text moves down; clicking again closes', async ({
	page
}) => {
	await page.goto(page2);
	const line = page.locator('#s4-l4');
	const before = await top(line);
	const what = page.locator(aside).getByText('What changes', { exact: true });
	await what.click();
	const body = page.locator(aside + ' [data-part="what"] .ex-body');
	await expect(body).toBeVisible();
	await expect(body).toContainText('Clinics must post hours.');
	// the open block sits below the whole row of controls
	expect(await top(body)).toBeGreaterThan(await top(page.locator(aside + ' summary').last()));
	expect(await top(line)).toBeGreaterThan(before + 20);
	await what.click();
	await expect(body).toBeHidden();
	expect(await top(line)).toBeCloseTo(before, 0);
});

test('another control of the same explanation switches the block; one is open at a time', async ({
	page
}) => {
	await page.goto(page2);
	const ex = page.locator(aside);
	await ex.getByText('What changes', { exact: true }).click();
	await expect(ex.locator('[data-part="what"] .ex-body')).toBeVisible();
	await ex.getByText('Why it matters', { exact: true }).click();
	await expect(ex.locator('[data-part="why"] .ex-body')).toBeVisible();
	await expect(ex.locator('[data-part="what"] .ex-body')).toBeHidden();
	await expect(ex.locator('details[open]')).toHaveCount(1);
	await ex.getByText('Existing law cited', { exact: true }).click();
	const law = ex.locator('[data-part="law"] .ex-body');
	await expect(law).toBeVisible();
	await expect(ex.locator('details[open]')).toHaveCount(1);
	// existing law is official text in a quote; the other parts are platform text
	await expect(law.locator('blockquote')).toHaveAttribute('data-content-class', 'official');
	await expect(law.locator('blockquote')).toContainText('Clinics may post hours.');
	await ex.getByText('Why it matters', { exact: true }).click();
	await expect(ex.locator('[data-part="why"] p')).toHaveAttribute('data-content-class', 'platform');
	const style = (sel: string) =>
		ex.locator(sel).evaluate((el) => {
			const s = getComputedStyle(el);
			return { font: s.fontFamily, bg: s.backgroundColor, left: s.borderLeftWidth };
		});
	const whyStyle = await style('[data-part="why"] .ex-body');
	await ex.getByText('Existing law cited', { exact: true }).click();
	const lawStyle = await style('[data-part="law"] .ex-body');
	expect(lawStyle.font).not.toBe(whyStyle.font);
	expect(lawStyle.left).not.toBe(whyStyle.left);
});

test('the controls work from the keyboard', async ({ page, isMobile }) => {
	test.skip(isMobile, 'keyboard');
	await page.goto(page2);
	const first = page.locator(aside + ' summary').first();
	await first.focus();
	await page.keyboard.press('Enter');
	await expect(page.locator(aside + ' [data-part="what"] .ex-body')).toBeVisible();
	await page.keyboard.press('Space');
	await expect(page.locator(aside + ' [data-part="what"] .ex-body')).toBeHidden();
});

test('a section that only gives the short title has no explanation', async ({ page }) => {
	await page.goto(page2);
	await expect(page.locator('#s1 aside[data-explanation]')).toHaveCount(0);
});
