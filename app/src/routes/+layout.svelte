<script lang="ts">
	import './layout.css';
	import favicon from '#lib/assets/favicon.svg';
	import Header from '#lib/components/Header.svelte';
	import OfflineLabel from '#lib/components/OfflineLabel.svelte';
	import { repoHref } from '#lib/links.ts';
	import type { LayoutProps } from './$types';

	let { data, children }: LayoutProps = $props();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>PubCommittee</title>
	{#if data.alternate}
		<link rel="alternate" type="application/json" href={data.alternate} />
	{/if}
</svelte:head>

<div class="flex min-h-screen flex-col bg-paper">
	<OfflineLabel />
	<Header />
	<main class="flex-1">
		{@render children()}
	</main>
	<footer class="border-t border-line px-4 py-6 text-center text-[13px] text-muted">
		Official bill text is public domain. Explanations are labelled with how they were produced.
		PubCommittee is
		<a href={repoHref} rel="noopener noreferrer">open source</a>.
	</footer>
</div>
