<script lang="ts">
	import { onMount } from 'svelte';

	// Offered only when the browser says the app can be installed (beforeinstallprompt, Chromium).
	// Other browsers install from their own menu; the manifest and service worker are the same.
	type InstallEvent = Event & { prompt: () => Promise<void> };
	let prompt = $state<InstallEvent | null>(null);

	onMount(() => {
		const keep = (e: Event) => {
			e.preventDefault();
			prompt = e as InstallEvent;
		};
		const done = () => (prompt = null);
		window.addEventListener('beforeinstallprompt', keep);
		window.addEventListener('appinstalled', done);
		return () => {
			window.removeEventListener('beforeinstallprompt', keep);
			window.removeEventListener('appinstalled', done);
		};
	});

	async function install() {
		const event = prompt;
		prompt = null;
		await event?.prompt();
	}
</script>

{#if prompt}
	<button type="button" data-install class="font-medium text-ink underline" onclick={install}>
		Install app
	</button>
{/if}
