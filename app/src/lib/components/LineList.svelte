<script lang="ts">
	import { rangeParam } from '#lib/anchors.ts';
	import { linesHref } from '#lib/links.ts';
	import { selection } from '#lib/selection.svelte.ts';
	import type { Explanation, Line } from '#lib/types.ts';
	import ExplanationBlock from './ExplanationBlock.svelte';

	// The literal text of one section: numbered lines, each with its anchor id and its canonical
	// address. Complete without scripts; scripts only add click selection (selection.svelte.ts).
	let {
		billId,
		version,
		sectionId,
		lines,
		highlight = null,
		explanations = []
	}: {
		billId: string;
		version: string;
		sectionId: string;
		lines: Line[];
		highlight?: { from: number; to: number } | null;
		/** Explanations of this section; each is shown above the first line it describes. */
		explanations?: Explanation[];
	} = $props();

	// The lines in runs, with the explanations pinned above the first line of a run between runs. The text
	// stays in official lists (data-content-class="official"); an explanation is its own platform block.
	// An explanation whose lines are all out of the shown range (a range address) is left out.
	const runs = $derived.by(() => {
		const first = lines[0]?.n ?? 0;
		const last = lines[lines.length - 1]?.n ?? 0;
		const at = new Map<number, { e: Explanation; i: number }[]>();
		explanations.forEach((e, i) => {
			const from = e.line_from ?? first;
			const to = e.line_to ?? from;
			if (to < first || from > last) return;
			const n = lines.find((l) => l.n >= from)?.n ?? first;
			at.set(n, [...(at.get(n) ?? []), { e, i }]);
		});
		const out: { pinned: { e: Explanation; i: number }[]; lines: Line[] }[] = [];
		for (const l of lines) {
			if (out.length === 0 || at.has(l.n)) out.push({ pinned: at.get(l.n) ?? [], lines: [] });
			out[out.length - 1].lines.push(l);
		}
		return out;
	});
	const whole = (e: Explanation) =>
		!e.provision_id && e.line_from === lines[0]?.n && e.line_to === lines[lines.length - 1]?.n;

	const marked = (n: number) => {
		const s = selection.current;
		if (s && s.sectionId === sectionId) return n >= s.from && n <= s.to;
		return !!highlight && !s && n >= highlight.from && n <= highlight.to;
	};
</script>

{#each runs as run, r (r)}
	{#each run.pinned as p (p.i)}
		<ExplanationBlock explanation={p.e} key="{sectionId}-{p.i}" whole={whole(p.e)} />
	{/each}
	<ol class="m-0 list-none p-0" start={run.lines[0].n} data-content-class="official">
		{#each run.lines as line (line.anchor)}
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
{/each}
