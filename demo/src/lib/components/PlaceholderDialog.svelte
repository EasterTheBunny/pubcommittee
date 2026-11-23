<script lang="ts">
	import { ui } from '#lib/state/ui.svelte.ts';
	import Modal from './Modal.svelte';
	import PlaceholderNote from './PlaceholderNote.svelte';

	let open = $state(false);
	let current = $state(ui.placeholder);

	// Keep the last id while the dialog closes, so content does not vanish mid-close.
	$effect(() => {
		if (ui.placeholder) {
			current = ui.placeholder;
			open = true;
		}
	});
	$effect(() => {
		if (!open) ui.placeholder = null;
	});
</script>

<Modal bind:open title="Not built yet" eyebrow="M0 proof of concept" width="max-w-lg">
	{#if current}
		<PlaceholderNote id={current} />
	{/if}
	<div class="flex justify-end">
		<button type="button" class="btn-primary" onclick={() => (open = false)}>Got it</button>
	</div>
</Modal>
