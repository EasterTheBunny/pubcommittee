<script lang="ts">
	import type { Snippet } from 'svelte';
	import { placeholders, type PlaceholderId } from '#lib/placeholders.ts';
	import { ui } from '#lib/state/ui.svelte.ts';

	// A control for a feature not built in M0. Dashed style; opens the placeholder pop-up.
	let {
		id,
		children,
		class: klass = ''
	}: { id: PlaceholderId; children: Snippet; class?: string } = $props();
</script>

<button type="button" class="btn-future {klass}" onclick={() => ui.future(id)}>
	{@render children()}
	<span class="text-xs font-bold text-faint">· {placeholders[id].milestone}</span>
</button>
