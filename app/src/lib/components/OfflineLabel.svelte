<script lang="ts">
	import { afterNavigate } from '$app/navigation';
	import { onMount } from 'svelte';

	// Shown when the device is offline, or when the service worker served a saved copy of this page
	// (it adds <meta name="pc-offline-copy" content="<time saved>">). Official text opened before is
	// exact because versions never change; everything else here may be out of date.
	let offline = $state(false);
	let savedAt = $state<string | null>(null);

	onMount(() => {
		offline = !navigator.onLine;
		const meta = document.querySelector('meta[name="pc-offline-copy"]');
		savedAt = meta ? (meta.getAttribute('content') ?? '') : null;
		const down = () => (offline = true);
		const up = () => (offline = false);
		window.addEventListener('offline', down);
		window.addEventListener('online', up);
		return () => {
			window.removeEventListener('offline', down);
			window.removeEventListener('online', up);
		};
	});

	// The marker describes the page that was first loaded, not later in-app navigations.
	afterNavigate(({ type }) => {
		if (type !== 'enter') savedAt = null;
	});

	const saved = $derived(savedAt ? new Date(savedAt) : null);
	const when = $derived(
		saved && !Number.isNaN(saved.getTime())
			? saved.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
			: null
	);
</script>

{#if offline || savedAt !== null}
	<div
		role="status"
		data-offline-label
		class="border-b border-line bg-card px-4 py-2 text-center text-[13px] text-ink"
	>
		<strong class="font-semibold">Possibly out of date.</strong>
		{#if offline}You are offline.{/if}
		{#if when}This is a copy saved {when}.{/if}
		Official text you opened before is exact because versions never change; status, counts, and other
		details may have changed.
	</div>
{/if}
