<script lang="ts">
	import './layout.css';
	import { onMount } from 'svelte';
	import favicon from '#lib/assets/favicon.svg';
	import Header from '#lib/components/Header.svelte';
	import OpposeDialog from '#lib/components/OpposeDialog.svelte';
	import PlaceholderDialog from '#lib/components/PlaceholderDialog.svelte';
	import ShareDialog from '#lib/components/ShareDialog.svelte';
	import SignInDialog from '#lib/components/SignInDialog.svelte';
	import { demo } from '#lib/state/demo.svelte.ts';
	import { ui } from '#lib/state/ui.svelte.ts';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();

	onMount(() => demo.init());
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>PubCommittee · M0</title>
</svelte:head>

<div class="flex min-h-screen flex-col bg-paper">
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
	class="pointer-events-none fixed inset-x-0 bottom-6 z-50 flex justify-center px-4"
>
	{#if ui.toast}
		<div class="rounded-lg bg-ink px-4 py-2.5 text-sm text-white shadow-lg">{ui.toast}</div>
	{/if}
</div>
