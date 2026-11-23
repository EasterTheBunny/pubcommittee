import { defineConfig, devices } from '@playwright/test';

// BASE_URL: the app on the fixture read-api. STUB_URL: the app on tests/stub-api.mjs (comparisons,
// a very long bill). Never hard-code hosts: both come from the environment.
const baseURL = process.env.BASE_URL ?? 'http://localhost:3000';

export default defineConfig({
	testDir: 'tests',
	fullyParallel: true,
	workers: 2,
	reporter: process.env.CI ? 'github' : 'line',
	use: { baseURL },
	projects: [
		{ name: 'desktop', use: { ...devices['Desktop Chrome'] } },
		{
			name: 'phone',
			use: { ...devices['Pixel 7'] },
			testMatch: /(phone|catalog|bill|explanation)\.spec\.ts/
		}
	]
});
