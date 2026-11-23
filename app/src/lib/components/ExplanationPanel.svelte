<script lang="ts">
	import type { Explanation, ExplanationFlag } from '#lib/types.ts';

	// Platform-written explanation of one section. Blue-tinted and always kept apart from the literal bill text.
	let { explanations }: { explanations: Explanation[] } = $props();

	const flagLabels: Record<ExplanationFlag, string> = {
		auto: 'Auto-generated',
		'steward-checked': 'Checked by a steward',
		'bill-only': 'Bill-only (drawn from the bill text alone)'
	};

	// Provenance label per flag; an explanation with no flags is auto-generated.
	function provenance(flags: ExplanationFlag[]): string[] {
		return flags.length > 0 ? flags.map((f) => flagLabels[f]) : ['Auto-generated'];
	}
</script>

{#if explanations.length > 0}
	<aside
		aria-label="Explanation of this section"
		class="rounded-xl border border-support-line bg-support-soft p-4"
	>
		{#each explanations as e (e.provision_id ?? e.what_changes)}
			<article class="flex flex-col gap-3">
				<div class="flex flex-wrap gap-2">
					{#each provenance(e.flags) as label, i (i)}
						<span class="chip bg-card text-ins-ink">{label}</span>
					{/each}
				</div>

				<div>
					<h4 class="eyebrow">What changes</h4>
					<p data-content-class={e.content_class}>{e.what_changes}</p>
				</div>

				<div>
					<h4 class="eyebrow">Why it matters</h4>
					<p data-content-class={e.content_class}>{e.why_it_matters}</p>
				</div>

				{#if e.current_text}
					<div>
						<h4 class="eyebrow">Existing law cited</h4>
						{#if e.provision_id}
							<p class="text-sm text-muted">Provision {e.provision_id}</p>
						{/if}
						<blockquote
							data-content-class={e.current_text.content_class}
							class="mt-1 border-l-4 border-line-strong bg-card px-3 py-2 text-ink-2"
						>
							{e.current_text.text}
						</blockquote>
					</div>
				{/if}

				{#if e.as_amended}
					<div>
						<div class="flex flex-wrap items-center gap-2">
							<h4 class="eyebrow">Preview as amended</h4>
							<span class="chip bg-oppose-soft text-oppose-ink">Unofficial</span>
						</div>
						<p class="mt-1" data-content-class={e.as_amended.content_class}>{e.as_amended.text}</p>
					</div>
				{/if}

				<p class="text-xs text-muted">Explanation as of {e.as_of.slice(0, 10)}</p>
			</article>
		{/each}
	</aside>
{/if}
