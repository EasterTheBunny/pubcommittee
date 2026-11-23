<script lang="ts">
	import type { Citation } from '#lib/types.ts';

	// Citations found in one bill's text, each linked to the cited line. The citation text is official bill text.
	let {
		citations,
		heading,
		emptyText
	}: { citations: Citation[]; heading: string; emptyText: string } = $props();

	const stateLabels: Record<Citation['state'], string> = {
		resolved: 'Resolved',
		ambiguous: 'Ambiguous',
		unresolved: 'Unresolved',
		'steward-confirmed': 'Confirmed by a steward'
	};

	// Unresolved and ambiguous citations get the orange style; the others are neutral.
	function stateClass(state: Citation['state']): string {
		return state === 'unresolved' || state === 'ambiguous'
			? 'bg-oppose-soft text-oppose-ink'
			: 'bg-line-soft text-ink-2';
	}
</script>

<section aria-label={heading}>
	<h3 class="eyebrow">{heading}</h3>

	{#if citations.length === 0}
		<p class="text-sm text-muted">{emptyText}</p>
	{:else}
		<ul class="mt-2 flex flex-col gap-2">
			{#each citations as c (c.section_id + ':' + (c.line ?? 0) + ':' + c.text)}
				<li data-content-class={c.content_class} class="flex flex-col gap-1">
					<span>{c.text}</span>
					<span class="flex flex-wrap items-center gap-2 text-sm">
						<span class="chip {stateClass(c.state)}">{stateLabels[c.state]}</span>
						<a href={'#' + c.section_id + (c.line ? '-l' + c.line : '')}>
							{c.section_id}{c.line ? ' line ' + c.line : ''}
						</a>
					</span>
				</li>
			{/each}
		</ul>
	{/if}
</section>
