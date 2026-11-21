<script lang="ts">
	import type { Explanation, Section } from '#lib/types.ts';
	import FutureButton from './FutureButton.svelte';

	// Bill against law: an explanation, never a literal diff (design/01).
	let { explanation: x, section }: { explanation: Explanation | undefined; section: Section } =
		$props();

	const provenanceLabel = {
		auto: 'Auto-generated',
		'steward-checked': 'Steward checked',
		'bill-only': 'Written from the bill alone'
	};
</script>

<div class="flex flex-col gap-3.5 card p-5" data-content-class="platform">
	<div class="flex flex-wrap items-center gap-2">
		<span class="eyebrow">Bill against law</span>
		<span class="text-xs text-muted">· explanation, not a diff</span>
	</div>
	<h3 class="m-0 font-display text-[19px] font-semibold">
		What {section.label} changes in current law
	</h3>

	{#if x}
		{#if x.currentText}
			<div
				class="rounded-lg border border-line bg-subtle px-3.5 py-3"
				data-content-class="official"
			>
				<div class="mb-1.5 text-xs font-semibold text-muted">Current law · {x.citation}</div>
				<div class="font-serif text-[15px] leading-relaxed">{x.currentText}</div>
			</div>
		{:else if x.citation}
			<div class="text-xs font-semibold text-muted">Adds to {x.citation}</div>
		{/if}
		<div class="flex flex-col gap-2 text-[15px] leading-relaxed">
			<p class="m-0"><strong>What the bill does.</strong> {x.whatChanges}</p>
			<p class="m-0"><strong>Why it matters.</strong> {x.whyItMatters}</p>
		</div>
		<div class="flex flex-wrap gap-2">
			{#each x.provenance as p (p)}
				<span
					class="chip {p === 'steward-checked' ? 'bg-ok-soft text-ok' : 'bg-line-soft text-ink-2'}"
				>
					{provenanceLabel[p]}
				</span>
			{/each}
		</div>
		{#if x.asAmended}
			<FutureButton id="as-amended">Preview as amended (unofficial)</FutureButton>
		{/if}
	{:else}
		<p class="m-0 text-sm text-muted">
			No change to existing law is linked to this section. In M1, citations are parsed automatically
			and unresolved ones go to a steward.
		</p>
	{/if}
</div>
