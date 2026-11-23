<script lang="ts">
	import { diffWords } from '#lib/diff.ts';

	// Citizen edit against bill: a literal side-by-side diff (design/01, design/03).
	let {
		before,
		after,
		beforeLabel = 'Bill text',
		afterLabel = 'Suggested replacement',
		compact = false
	}: {
		before: string;
		after: string;
		beforeLabel?: string;
		afterLabel?: string;
		compact?: boolean;
	} = $props();

	const parts = $derived(diffWords(before, after));
</script>

<div class="grid md:grid-cols-2">
	<div
		class="border-line {compact ? 'px-4 py-3' : 'px-5 py-4.5'} md:border-r"
		data-content-class="official"
	>
		<div class="mb-2.5 eyebrow">{beforeLabel}</div>
		<p class="m-0 font-serif text-base leading-[1.7]">
			{#each parts as p, i (i)}
				{#if p.kind === 'eq'}{p.text}{:else if p.kind === 'del'}<del
						class="bg-del text-del-ink decoration-2"
					>
						{p.text}
					</del>{/if}
			{/each}
		</p>
	</div>
	<div class="bg-[#fafcff] {compact ? 'px-4 py-3' : 'px-5 py-4.5'}" data-content-class="user">
		<div class="mb-2.5 eyebrow">{afterLabel}</div>
		<p class="m-0 font-serif text-base leading-[1.7]">
			{#if after.trim() === ''}
				<em class="font-sans text-sm text-muted">Delete these lines.</em>
			{/if}
			{#each parts as p, i (i)}
				{#if p.kind === 'eq'}{p.text}{:else if p.kind === 'ins'}<ins
						class="bg-support-tint text-ins-ink decoration-2"
					>
						{p.text}
					</ins>{/if}
			{/each}
		</p>
	</div>
</div>
