<script lang="ts">
	import BillView from '#lib/components/BillView.svelte';
	import { billHref } from '#lib/links.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head>
	<title>{data.bill.number} {data.section.label} · {data.bill.title}</title>
</svelte:head>

{#key `${data.version.n}-${data.section.id}`}
	<BillView
		bill={data.bill}
		version={data.version}
		initial={{ sectionId: data.section.id }}
		basePath={billHref(data.bill.id, data.version.n)}
	/>
{/key}
