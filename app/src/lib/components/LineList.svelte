<script lang="ts">
	import { rangeParam } from '#lib/anchors.ts';
	import { linesHref } from '#lib/links.ts';
	import { selection } from '#lib/selection.svelte.ts';
	import type { Line } from '#lib/types.ts';

	// The literal text of one section: numbered lines, each with its anchor id and its canonical
	// address. Complete without scripts; scripts only add click selection (selection.svelte.ts).
	let {
		billId,
		version,
		sectionId,
		lines,
		highlight = null
	}: {
		billId: string;
		version: number;
		sectionId: string;
		lines: Line[];
		highlight?: { from: number; to: number } | null;
	} = $props();

	const marked = (n: number) => {
		const s = selection.current;
		if (s && s.sectionId === sectionId) return n >= s.from && n <= s.to;
		return !!highlight && !s && n >= highlight.from && n <= highlight.to;
	};
</script>

<ol class="m-0 list-none p-0" data-content-class="official">
	{#each lines as line (line.anchor)}
		<li
			id={line.anchor}
			data-line={line.n}
			data-depth={line.depth}
			class="ln"
			class:ln-on={marked(line.n)}
			style="--d:{line.depth}"
		>
			<a
				href={linesHref({ billId, version, sectionId, from: line.n, to: line.n })}
				class="ln-no"
				aria-label="Address of line {line.n}"
				data-line-number={line.n}
				onclick={(e) => {
					if (e.button !== 0 || e.metaKey || e.ctrlKey) return;
					e.preventDefault();
					selection.pick(sectionId, line.n, e.shiftKey);
				}}
			>
				{line.n}
			</a>
			<span class="ln-text">{line.text}</span>
			{#if line.block_to > line.n}
				<a
					href={linesHref({
						billId,
						version,
						sectionId,
						from: line.n,
						to: line.block_to
					})}
					class="ln-block"
					title="Lines {rangeParam(
						line.n,
						line.block_to
					)}: this line with everything nested under it"
				>
					block
				</a>
			{/if}
		</li>
	{/each}
</ol>
