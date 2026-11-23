import { defineConfig, devices } from '@playwright/test';

// M1-16 local gate: UI checks of the exit criteria on the real bills. Run from app/ so that
// @playwright/test resolves from app/node_modules (NODE_PATH=/src/app/node_modules):
//   npx playwright test --config ../test/acceptance/M1-16/e2e/playwright.config.ts
// BASE_URL: the app. API_URL: the read-api as the app container reaches it.
export default defineConfig({
	testDir: '.',
	outputDir: process.env.PW_OUTPUT ?? '/tmp/pw-m1-16',
	fullyParallel: false,
	workers: 2,
	reporter: [['list']],
	timeout: 60_000,
	use: { baseURL: process.env.BASE_URL ?? 'http://localhost:3000' },
	projects: [{ name: 'desktop', use: { ...devices['Desktop Chrome'] } }]
});
