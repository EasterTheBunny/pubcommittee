<script lang="ts">
	import { count, demo, pct, share } from '#lib/state/demo.svelte.ts';
	import FutureButton from './FutureButton.svelte';
	import { ui } from '#lib/state/ui.svelte.ts';

	// Bill-level signal: support or oppose the whole bill (design/03).
	let { billId }: { billId: string } = $props();

	const totals = $derived(demo.billTotals(billId));
	const mine = $derived(demo.signalFor(billId));

	function support() {
		if (!ui.requireResident()) return;
		if (mine?.value === 'support') demo.setSignal(billId, null);
		else demo.setSignal(billId, { value: 'support', reasons: [], riderSections: [], comment: '' });
	}

	function oppose() {
		if (!ui.requireResident()) return;
		ui.oppose = billId;
	}
</script>

<div
	class="flex flex-col gap-3.5 rounded-xl border border-line bg-subtle p-4.5"
	data-content-class="platform"
>
	<div class="eyebrow">Bill-level signal</div>
	<div class="grid grid-cols-2 gap-3">
		<div>
			<div class="font-display text-[28px] font-semibold text-support">{count(totals.support)}</div>
			<div class="text-[13px] text-muted">
				support{share(totals.support, totals.reviewers)
					? ` · ${share(totals.support, totals.reviewers)} of reviewers`
					: ''}
			</div>
		</div>
		<div>
			<div class="font-display text-[28px] font-semibold text-oppose">{count(totals.oppose)}</div>
			<div class="text-[13px] text-muted">
				oppose{share(totals.oppose, totals.reviewers)
					? ` · ${share(totals.oppose, totals.reviewers)} of reviewers`
					: ''}
			</div>
		</div>
	</div>
	<div class="flex h-2 overflow-hidden rounded bg-line-soft" aria-hidden="true">
		<div class="bg-support" style:width="{pct(totals.support, totals.reviewers)}%"></div>
		<div class="bg-oppose-bar" style:width="{pct(totals.oppose, totals.reviewers)}%"></div>
	</div>
	<div class="flex flex-wrap items-center gap-2 text-[13px] text-muted">
		{count(totals.reviewers)} reviewers · national totals
		<FutureButton id="count-batching" class="min-h-8 px-2.5 text-xs">
			How counts update
		</FutureButton>
	</div>

	{#if mine}
		<div class="text-sm" role="status">
			You {mine.value} this bill{mine.reasons.length
				? ` (${mine.reasons.map((r) => ({ complex: 'too complex', riders: 'unrelated riders', policy: 'policy' })[r]).join(', ')})`
				: ''}. Private to you.
			<button
				type="button"
				class="ml-1 cursor-pointer font-semibold text-support underline"
				onclick={() => demo.setSignal(billId, null)}
			>
				Withdraw
			</button>
		</div>
	{/if}

	<div class="flex gap-2.5">
		<button
			type="button"
			class="flex-1 {mine?.value === 'support' ? 'btn-primary' : 'btn-outline'}"
			aria-pressed={mine?.value === 'support'}
			onclick={support}
		>
			{mine?.value === 'support' ? 'Supported' : 'Support'}
		</button>
		<button
			type="button"
			class="flex-1 {mine?.value === 'oppose' ? 'btn-oppose-solid' : 'btn-oppose'}"
			aria-pressed={mine?.value === 'oppose'}
			onclick={oppose}
		>
			{mine?.value === 'oppose' ? 'Opposed · edit' : 'Oppose'}
		</button>
	</div>
</div>
