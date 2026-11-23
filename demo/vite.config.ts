import tailwindcss from '@tailwindcss/vite';
import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit({
			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},
			adapter: adapter(),
			prerender: {
				// Line-range hashes (#s4-l12-13) are parsed in the browser, not element ids.
				handleMissingId: ({ id, message }) => {
					if (/^s\d+-l\d+(-\d+)?$/.test(id)) return;
					throw new Error(message);
				}
			}
		})
	]
});
