<script lang="ts">
	import type { ExplanationFlag, Statement } from '#lib/types.ts';
	import AccuracyNote from './AccuracyNote.svelte';

	// Platform-written summary of a bill or a version change. Blue-tinted, never shown as bill text.
	let {
		title,
		doc,
		emptyText,
		textHref = '#bill-text'
	}: {
		title: string;
		doc: {
			headline: string;
			statements: Statement[];
			flags?: ExplanationFlag[];
			as_of: string;
			content_class: 'official' | 'platform';
		} | null;
		emptyText: string;
		/** The bill text the summary describes. */
		textHref?: string;
	} = $props();

	const flagLabels: Record<ExplanationFlag, string> = {
		auto: 'Auto-generated',
		'steward-checked': 'Checked by a steward',
		'bill-only': 'Bill-only (drawn from the bill text alone)'
	};

	// Provenance label per flag; a summary with no flags is auto-generated.
	function provenance(flags: ExplanationFlag[] | undefined): string[] {
		return flags && flags.length > 0 ? flags.map((f) => flagLabels[f]) : ['Auto-generated'];
	}
</script>

{#if doc}
	<section
		class="rounded-xl border border-support-line bg-support-soft p-4"
		data-content-class={doc.content_class}
	>
		<h3 class="eyebrow">{title}</h3>

		<div class="mt-2 flex flex-wrap gap-2">
			{#each provenance(doc.flags) as label, i (i)}
				<span class="chip bg-card text-ins-ink">{label}</span>
			{/each}
		</div>

		<p class="mt-3 font-semibold" data-content-class={doc.content_class}>{doc.headline}</p>

		<ul class="mt-2 flex flex-col gap-2">
			{#each doc.statements as s, i (i)}
				<li data-content-class={s.content_class}>
					{s.text}
					{#if s.anchors.length}
						<span class="text-sm text-muted">
							{s.anchors.length === 1 ? 'line' : 'lines'}:
							{#each s.anchors as a, j (j)}
								{#if j > 0},
								{/if}<a href={'#' + a}>{a}</a>
							{/each}
						</span>
					{/if}
				</li>
			{/each}
		</ul>

		<p class="mt-3 text-xs text-muted">As of {doc.as_of.slice(0, 10)}</p>
		<AccuracyNote href={textHref} />
	</section>
{:else}
	<section class="card p-4">
		<h3 class="eyebrow">{title}</h3>
		<p class="text-sm text-muted">{emptyText}</p>
	</section>
{/if}
