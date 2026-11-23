<script lang="ts">
	import BillVersionView from '#lib/components/BillVersionView.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<BillVersionView {data} />
