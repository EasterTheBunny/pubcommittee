<script lang="ts">
	import './layout.css';
	import { onMount } from 'svelte';
	import favicon from '#lib/assets/favicon.svg';
	import Header from '#lib/components/Header.svelte';
	import OpposeDialog from '#lib/components/OpposeDialog.svelte';
	import PlaceholderDialog from '#lib/components/PlaceholderDialog.svelte';
	import ShareDialog from '#lib/components/ShareDialog.svelte';
	import SignInDialog from '#lib/components/SignInDialog.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import { repoHref } from '#lib/links.ts';
	import { demo } from '#lib/state/demo.svelte.ts';
	import { ui } from '#lib/state/ui.svelte.ts';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();

	onMount(() => {
		demo.init();
		// Chromium offers an install prompt; keep it so the header can show an Install button.
		const keep = (e: Event) => {
			e.preventDefault();
			ui.installPrompt = e as Event & { prompt: () => Promise<void> };
		};
		const installed = () => (ui.installPrompt = null);
		window.addEventListener('beforeinstallprompt', keep);
		window.addEventListener('appinstalled', installed);
		return () => {
			window.removeEventListener('beforeinstallprompt', keep);
			window.removeEventListener('appinstalled', installed);
		};
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>PubCommittee · M0</title>
</svelte:head>

<div class="flex min-h-screen flex-col bg-paper">
	<div
		role="region"
		aria-label="About this demo"
		class="flex flex-wrap items-center justify-center gap-x-3 gap-y-1 bg-ink px-4 py-2 text-center text-[13px] text-white"
	>
		<span>Proof of concept on mock data. PubCommittee is open source.</span>
		<a
			href={repoHref}
			target="_blank"
			rel="noopener noreferrer"
			class="inline-flex min-h-8 items-center gap-1 font-semibold text-white underline hover:text-[#d6d9de]"
		>
			View the code on GitHub <Icon name="arrowRight" size={14} />
		</a>
	</div>
	<Header />
	<main class="flex-1">
		{@render children()}
	</main>
	<footer class="border-t border-line px-4 py-6 text-center text-[13px] text-muted">
		M0 proof of concept. All bills, names, offices and numbers are mock data. Dashed buttons open a
		pop-up describing a future feature.
	</footer>
</div>

<SignInDialog />
<ShareDialog />
<OpposeDialog />
<PlaceholderDialog />

<div
	aria-live="polite"
	class="pointer-events-none fixed inset-x-0 z-50 flex justify-center px-4 {ui.actionBar
		? 'bottom-44'
		: 'bottom-6'}"
>
	{#if ui.toast}
		<div class="rounded-lg bg-ink px-4 py-2.5 text-sm text-white shadow-lg">{ui.toast}</div>
	{/if}
</div>
