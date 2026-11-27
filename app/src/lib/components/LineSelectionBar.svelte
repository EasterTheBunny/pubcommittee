<script lang="ts">
	import { onMount } from 'svelte';
	import { rangeLabel } from '#lib/anchors.ts';
	import { linesHref } from '#lib/links.ts';
	import { selection } from '#lib/selection.svelte.ts';

	// Enhancement for readers with scripts: shows the selected lines and copies their canonical
	// address. Without scripts the "address of line" links on every line do the same job.
	let { billId, version }: { billId: string; version: string } = $props();

	let mounted = $state(false);
	let copied = $state(false);

	onMount(() => {
		mounted = true;
		selection.fromHash(location.hash);
		const onHash = () => selection.fromHash(location.hash);
		window.addEventListener('hashchange', onHash);
		return () => window.removeEventListener('hashchange', onHash);
	});

	const sel = $derived(selection.current);
	const href = $derived(
		sel ? linesHref({ billId, version, sectionId: sel.sectionId, from: sel.from, to: sel.to }) : ''
	);

	async function copy() {
		await navigator.clipboard.writeText(location.origin + href);
		copied = true;
		setTimeout(() => (copied = false), 2500);
	}
</script>

{#if mounted && sel}
	<div
		role="region"
		aria-label="Selected lines"
		class="fixed inset-x-0 bottom-0 z-40 border-t border-line-strong bg-card px-4 py-3 shadow-lg"
	>
		<div class="mx-auto flex max-w-[1000px] flex-wrap items-center gap-3">
			<strong class="text-sm">{rangeLabel(sel.from, sel.to)} of {sel.sectionId}</strong>
			<button type="button" class="btn-primary" onclick={copy}>
				{copied ? 'Link copied' : 'Copy link'}
			</button>
			<a {href} class="btn-secondary">Open address</a>
			<button type="button" class="btn-ghost" onclick={() => selection.clear()}>Clear</button>
			<span class="text-xs text-muted">Shift-click a second line number to extend.</span>
		</div>
	</div>
{/if}
