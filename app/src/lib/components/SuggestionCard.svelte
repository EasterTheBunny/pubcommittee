<script lang="ts">
	import { anchorText, isStale, rangeLabel } from '#lib/anchors.ts';
	import { linesHref } from '#lib/links.ts';
	import { BRIEF_MINIMUM, sectionOf, versionOf } from '#lib/mock/index.ts';
	import { stewardConflict } from '#lib/rules.ts';
	import { count as countLabel, demo, isSmall, share } from '#lib/state/demo.svelte.ts';
	import { ui } from '#lib/state/ui.svelte.ts';
	import type { Bill, SuggestedEdit } from '#lib/types.ts';
	import FutureButton from './FutureButton.svelte';
	import Icon from './Icon.svelte';
	import SideBySide from './SideBySide.svelte';

	let {
		suggestion: s,
		bill,
		rank
	}: { suggestion: SuggestedEdit; bill: Bill; rank: number } = $props();

	const stale = $derived(isStale(bill, s.anchor));
	const count = $derived(demo.supportCount(s.id));
	const reviewers = $derived(demo.billTotals(bill.id).reviewers);
	const supported = $derived(demo.hasSupported(s.id));
	const response = $derived(demo.responses[s.id]);
	const section = $derived(sectionOf(versionOf(bill, s.anchor.version)!, s.anchor.sectionId)!);
	const conflict = $derived(stewardConflict(demo.persona, bill));

	const statusLabel = { adopted: 'Adopted', consider: 'Under consideration', declined: 'Declined' };

	function toggle() {
		if (!ui.requireResident()) return;
		demo.toggleSupport(s.id);
	}
</script>

<article
	id={s.id}
	class="scroll-mt-6 overflow-hidden card {stale ? 'border-oppose-line' : ''}"
	data-content-class="user"
>
	<div class="flex flex-wrap items-center gap-2.5 border-b border-line px-5 py-3.5">
		<span class="font-display text-[17px] font-semibold">
			<span data-content-class="platform">#{rank} ·</span>
			{s.title}
		</span>
		{#if stale}
			<span class="chip bg-oppose-soft text-oppose-ink">
				Stale · written against v{s.anchor.version}
			</span>
		{:else if s.wellFormed}
			<span class="chip bg-ok-soft text-ok">Well formed</span>
		{:else}
			<span class="chip bg-line-soft text-ink-2">Not reviewed</span>
		{/if}
		{#if !stale}
			{#if count >= BRIEF_MINIMUM}
				<span class="chip bg-support-soft text-support-strong">In the brief</span>
			{:else}
				<!-- "N more to go" would reveal the exact count (design/11). -->
				<span class="chip bg-line-soft text-ink-2">
					Enters the brief at {BRIEF_MINIMUM} supporters
				</span>
			{/if}
		{/if}
		<span class="ml-auto text-[13px] text-muted" data-content-class="platform">
			{section.label} · {rangeLabel(s.anchor.from, s.anchor.to)} · by
			<strong class="text-ink" data-content-class="user">{s.author}</strong>
			{#if s.revisions}
				·
				<button
					type="button"
					class="cursor-pointer text-support underline"
					onclick={() => ui.future('revision-history')}
				>
					revised {s.revisions === 1 ? 'once' : `${s.revisions} times`}
				</button>
			{/if}
		</span>
	</div>

	{#if stale}
		<div
			class="flex flex-col gap-3 bg-oppose-tint px-5 py-4 text-sm leading-relaxed text-ink-2"
			data-content-class="platform"
		>
			<p class="m-0">
				A newer official version changed the lines this targets. The author has been asked to
				re-anchor it. If the meaning still holds, support carries over and supporters are notified.
				If it changed, support resets.
			</p>
			<div class="flex flex-wrap gap-2.5">
				<FutureButton id="reanchor">Re-anchor to the current version</FutureButton>
				<a class="btn-secondary" href={linesHref(s.anchor)}>View in v{s.anchor.version}</a>
			</div>
		</div>
	{/if}

	<SideBySide
		before={anchorText(bill, s.anchor)}
		after={s.replacement}
		beforeLabel="Bill text (v{s.anchor.version})"
	/>

	<div class="flex flex-wrap items-center gap-x-7 gap-y-4 border-t border-line px-5 py-4">
		<div class="min-w-0 flex-[999_1_420px] text-[15px] leading-normal">
			<span class="eyebrow">Why</span><br />{s.reason}
		</div>
		<div class="flex flex-[1_1_280px] items-center gap-3.5" data-content-class="platform">
			<div>
				<div
					class="font-display font-semibold text-support {isSmall(count)
						? 'text-[17px]'
						: 'text-[26px]'}"
				>
					{countLabel(count)}
				</div>
				<div class="text-[13px] text-muted">
					supporters{share(count, reviewers)
						? ` · ${share(count, reviewers)} of reviewers`
						: ''}{stale ? ' · frozen' : ''}
				</div>
			</div>
			<button
				type="button"
				class="ml-auto {supported ? 'btn-primary' : 'btn-outline'}"
				aria-pressed={supported}
				disabled={stale}
				onclick={toggle}
			>
				{supported ? 'Supported' : 'Support'}
			</button>
			<button
				type="button"
				class="btn-secondary w-11 px-0"
				aria-label="Share this suggestion"
				onclick={() => (ui.share = { kind: 'suggestion', suggestionId: s.id })}
			>
				<Icon name="share" />
			</button>
		</div>
	</div>

	{#if response}
		<div class="border-t border-line bg-subtle px-5 py-3 text-sm" data-content-class="user">
			<strong>{response.office}:</strong>
			{statusLabel[response.status]}{response.reason ? `. ${response.reason}` : ''}
		</div>
	{/if}

	{#if demo.persona.role === 'steward' && !stale}
		<div
			class="flex flex-wrap items-center gap-2.5 border-t border-support-tint bg-[#f3f7ff] px-5 py-3.5"
		>
			<span class="mr-1.5 text-[13px] font-semibold text-support-strong">Steward tools</span>
			{#if conflict}
				<span class="text-[13px] text-ink-2">
					Your member sponsors this bill, so you cannot mark, combine, or annotate here. You can
					still answer factual questions in comments.
				</span>
			{:else}
				<button
					type="button"
					class="btn border-ok bg-card text-ok hover:bg-ok-soft"
					aria-pressed={s.wellFormed}
					onclick={() => demo.markWellFormed(s.id, !s.wellFormed)}
				>
					{#if s.wellFormed}<Icon name="check" /> Marked well formed{:else}Mark well formed{/if}
				</button>
				<FutureButton id="wording-fix">Offer wording fix</FutureButton>
				<FutureButton id="combine-duplicates">Combine duplicates</FutureButton>
			{/if}
		</div>
	{/if}
</article>
