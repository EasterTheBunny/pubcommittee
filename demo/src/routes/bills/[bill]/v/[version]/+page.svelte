<script lang="ts">
	import BillView from '#lib/components/BillView.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head><title>{data.bill.number} · {data.bill.title}</title></svelte:head>

{#key data.version.n}
	<BillView bill={data.bill} version={data.version} />
{/key}
