<script lang="ts">
	import { anchorText, rangeLabel } from '#lib/anchors.ts';
	import BillView from '#lib/components/BillView.svelte';
	import { billHref } from '#lib/links.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	const quote = $derived(
		anchorText(data.bill, {
			billId: data.bill.id,
			version: data.version.n,
			sectionId: data.section.id,
			from: data.from,
			to: data.to
		})
	);
</script>

<svelte:head>
	<title>
		{data.bill.number}
		{data.section.label}, {rangeLabel(data.from, data.to)} · {data.bill.title}
	</title>
	<!-- Summary metadata holds official text only, never user content (design/10). -->
	<meta name="description" content={quote} />
</svelte:head>

{#key `${data.version.n}-${data.section.id}-${data.from}-${data.to}`}
	<BillView
		bill={data.bill}
		version={data.version}
		initial={{ sectionId: data.section.id, from: data.from, to: data.to }}
		basePath={billHref(data.bill.id, data.version.n)}
	/>
{/key}
