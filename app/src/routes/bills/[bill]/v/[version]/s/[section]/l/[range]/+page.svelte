<script lang="ts">
	import AddressView from '#lib/components/AddressView.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<AddressView {data} />
