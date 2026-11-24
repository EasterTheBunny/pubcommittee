<script lang="ts">
	import type { Explanation, ExplanationFlag } from '#lib/types.ts';
	import AccuracyNote from './AccuracyNote.svelte';

	// Platform-written explanation pinned above the line it describes: a row of controls styled like the
	// line's "block" control, and one block below it that holds the open part. The parts are native
	// <details> elements sharing a `name`, so the browser keeps one open per explanation and every part is
	// reachable without scripts. Always kept apart from the literal bill text (platform, not official).
	let {
		explanation: e,
		key,
		whole = false
	}: {
		explanation: Explanation;
		/** Unique within the page; names the group of parts. */
		key: string;
		/** True when it explains the whole section rather than particular lines. */
		whole?: boolean;
	} = $props();

	const flagLabels: Record<ExplanationFlag, string> = {
		auto: 'Auto-generated',
		'steward-checked': 'Checked by a steward',
		'bill-only': 'Bill-only (drawn from the bill text alone)'
	};

	// Provenance label per flag; an explanation with no flags is auto-generated.
	const labels = $derived(
		e.flags.length > 0 ? e.flags.map((f) => flagLabels[f]) : ['Auto-generated']
	);

	const from = $derived(e.line_from);
	const to = $derived(e.line_to ?? e.line_from);
	const where = $derived(
		whole || from === undefined
			? 'this section'
			: from === to
				? `line ${from}`
				: `lines ${from}-${to}`
	);
	// What the explanation refers to, in words: the provision of existing law it changes, when it has one.
	const target = $derived(e.provision_citation ?? e.provision_id);
	// The lines it explains, for the accuracy note's link; the section when it explains all of it.
	const textHref = $derived(
		whole || from === undefined ? `#${e.section_id}` : `#${e.section_id}-l${from}`
	);

	// Each part shows only when the explanation has it (design/tasks/m1/FEEDBACK.md item 1).
	const parts = $derived(
		[
			{ id: 'what', label: 'What changes', text: e.what_changes?.trim() },
			{ id: 'why', label: 'Why it matters', text: e.why_it_matters?.trim() },
			{ id: 'law', label: 'Existing law cited', text: e.current_text?.text },
			{ id: 'amended', label: 'Preview as amended', text: e.as_amended?.text }
		].filter((p) => !!p.text)
	);
</script>

<aside
	class="ex"
	aria-label="Explanation of {where}{target ? `, ${target}` : ''}"
	data-content-class="platform"
	data-explanation
	data-lines={from === undefined ? undefined : `${from}-${to}`}
>
	<p class="ex-head">
		{#each labels as label, i (i)}
			<span class="chip bg-support-soft text-ins-ink">{label}</span>
		{/each}
		<span>Explains {where}</span>
		{#if target}
			<span>
				changes <strong class="font-semibold">{target}</strong>
			</span>
		{/if}
		<span class="ex-date">as of {e.as_of.slice(0, 10)}</span>
	</p>
	{#if parts.length}
		<div class="ex-parts">
			{#each parts as p (p.id)}
				<details class="ex-part" name="ex-{key}" data-part={p.id}>
					<summary class="ex-ctl">
						{p.label}
						{#if p.id === 'amended'}<span class="chip bg-oppose-soft text-oppose-ink">
								Unofficial
							</span>{/if}
					</summary>
					<div class="ex-body">
						{#if p.id === 'law'}
							<blockquote data-content-class={e.current_text?.content_class}>{p.text}</blockquote>
						{:else if p.id === 'amended'}
							<p data-content-class={e.as_amended?.content_class}>{p.text}</p>
						{:else}
							<p data-content-class={e.content_class}>{p.text}</p>
						{/if}
					</div>
				</details>
			{/each}
		</div>
	{/if}
	<AccuracyNote href={textHref} />
</aside>
