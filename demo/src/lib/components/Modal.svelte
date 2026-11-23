<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';

	let {
		open = $bindable(false),
		title,
		eyebrow,
		width = 'max-w-xl',
		children
	}: {
		open?: boolean;
		title: string;
		eyebrow?: string;
		width?: string;
		children: Snippet;
	} = $props();

	let dialog: HTMLDialogElement | undefined = $state();
	const titleId = `dlg-${Math.random().toString(36).slice(2, 8)}`;

	$effect(() => {
		if (!dialog) return;
		if (open && !dialog.open) dialog.showModal();
		if (!open && dialog.open) dialog.close();
	});
</script>

<dialog
	bind:this={dialog}
	aria-labelledby={titleId}
	onclose={() => (open = false)}
	onclick={(e) => {
		if (e.target === dialog) open = false;
	}}
	class="m-auto w-[calc(100%-2rem)] {width} rounded-2xl bg-card p-0 text-ink shadow-2xl"
>
	{#if open}
		<div class="flex flex-col gap-5 p-6 sm:p-7">
			<div class="flex items-start gap-3">
				<div class="flex flex-col gap-1">
					{#if eyebrow}<div class="text-sm text-muted">{eyebrow}</div>{/if}
					<h2 id={titleId} class="font-display text-2xl font-semibold">{title}</h2>
				</div>
				<button
					type="button"
					class="ml-auto inline-flex size-11 cursor-pointer items-center justify-center rounded-lg text-muted hover:bg-subtle"
					aria-label="Close"
					onclick={() => (open = false)}
				>
					<Icon name="close" size={18} />
				</button>
			</div>
			{@render children()}
		</div>
	{/if}
</dialog>
