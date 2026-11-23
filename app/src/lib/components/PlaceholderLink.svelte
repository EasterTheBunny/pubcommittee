<script lang="ts">
	import type { Snippet } from 'svelte';
	import { placeholders, type PlaceholderId } from '#lib/placeholders.ts';
	import PlaceholderNote from './PlaceholderNote.svelte';

	// A control for a feature that is not built yet. A native <details>, so it opens without
	// scripts and names the milestone that delivers the feature (placeholders.ts).
	let {
		id,
		children,
		class: klass = ''
	}: { id: PlaceholderId; children: Snippet; class?: string } = $props();
</script>

<details class="placeholder {klass}">
	<summary class="btn-future">
		{@render children()}
		<span class="text-xs font-bold text-faint">· {placeholders[id].milestone}</span>
	</summary>
	<div class="mt-2 max-w-md">
		<PlaceholderNote {id} />
	</div>
</details>
