<script lang="ts">
	import { rangeLabel } from '#lib/anchors.ts';
	import { billHref, linesHref, sectionHref } from '#lib/links.ts';
	import type { AddressPageData } from '#lib/server/bill.ts';
	import ChangedNotice from './ChangedNotice.svelte';
	import LineList from './LineList.svelte';
	import LineSelectionBar from './LineSelectionBar.svelte';

	// One section or line range of one official version, at its canonical address (design/10).
	let { data }: { data: AddressPageData } = $props();

	const lines = $derived(data.section ? data.section.lines : data.lines);
	const heading = $derived(
		data.section ? `${data.section.label} ${data.section.heading}` : `Section ${data.sectionId}`
	);
	const pinned = $derived(
		data.range
			? linesHref({
					billId: data.bill.id,
					version: data.code,
					sectionId: data.sectionId,
					...data.range
				})
			: sectionHref(data.bill.id, data.code, data.sectionId)
	);
</script>

<svelte:head>
	<title>
		{data.bill.title}, {data.range ? rangeLabel(data.range.from, data.range.to) : heading} · PubCommittee
	</title>
	<link rel="canonical" href={pinned} />
</svelte:head>

<div class="mx-auto max-w-[900px] px-4 py-6 sm:px-6">
	<nav aria-label="Breadcrumb" class="text-sm text-muted">
		<a href={billHref(data.bill.id)}>{data.bill.title}</a> /
		<a href={billHref(data.bill.id, data.code)}>{data.versionLabel}</a>
	</nav>
	<h1 class="mt-2 font-display text-xl font-bold sm:text-2xl">
		{heading}{#if data.range}, {rangeLabel(data.range.from, data.range.to)}{/if}
	</h1>
	<p class="mt-1 text-sm text-muted">
		{data.versionLabel}; the latest version is {data.address.latest_version}.
		{#if data.address.resolved_from_latest}
			This address follows the latest version; to cite it, use the
			<a href={pinned}>pinned address</a>.
		{/if}
	</p>

	<div class="mt-4">
		<ChangedNotice
			notice={data.notice}
			billId={data.bill.id}
			sectionId={data.sectionId}
			range={data.range}
		/>
	</div>

	<section class="mt-4 card p-4" aria-label="Bill text">
		<h2 class="mb-1 eyebrow">Bill text</h2>
		<LineList
			billId={data.bill.id}
			version={data.code}
			sectionId={data.sectionId}
			{lines}
			highlight={data.range}
			explanations={data.explanations}
		/>
		{#if data.range}
			<p class="mt-3 text-sm">
				<a href={sectionHref(data.bill.id, data.code, data.sectionId)}>The whole section</a>
			</p>
		{/if}
	</section>
</div>
<LineSelectionBar billId={data.bill.id} version={data.code} />
