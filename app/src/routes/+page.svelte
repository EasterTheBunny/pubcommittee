<script lang="ts">
	import {
		billHref,
		briefHref,
		catalogHref,
		issueHref,
		linesHref,
		sectionSuggestionsHref
	} from '#lib/links.ts';
	import { bills, issues, latestVersion } from '#lib/mock/index.ts';
	import { demo, fmt, pct } from '#lib/state/demo.svelte.ts';
	import { ui } from '#lib/state/ui.svelte.ts';

	const tour = [
		{ text: 'Read a bill and its change explanation', href: billHref('hr-4127') },
		{
			text: 'Open a shared line (deep link)',
			href: linesHref({ billId: 'hr-4127', version: 2, sectionId: 's4', from: 12, to: 13 })
		},
		{
			text: 'Compare suggested edits side by side',
			href: sectionSuggestionsHref('hr-4127', 2, 's4', 12, 13)
		},
		{
			text: 'Open an old shared link after the text changed',
			href: linesHref({ billId: 'hr-4127', version: 1, sectionId: 's4', from: 12, to: 13 })
		},
		{
			text: 'Compare competing bills on the issue page',
			href: issueHref('rural-clinic-inspections')
		},
		{ text: 'Browse the catalog and request markup of a bill', href: catalogHref() },
		{ text: 'Read the brief and respond as an office', href: briefHref('hr-4127') }
	];
</script>

<div class="mx-auto flex max-w-[1200px] flex-col gap-8 px-4 py-8 sm:px-6">
	<section class="flex flex-col gap-3">
		<h1 class="m-0 font-display text-4xl font-semibold">Review active bills, line by line</h1>
		<p class="m-0 max-w-3xl text-lg leading-relaxed text-ink-2">
			Comment on any line, suggest exact replacement text, and support or oppose the bill as a
			whole. Offices receive a short brief they can act on. It is not a vote.
		</p>
	</section>

	<div class="flex flex-wrap items-start gap-6">
		<section aria-label="Active bills" class="flex min-w-0 flex-[999_1_560px] flex-col gap-4">
			<div class="flex flex-wrap items-baseline gap-3">
				<h2 class="m-0 font-display text-xl font-semibold">Bills in review</h2>
				<a class="text-sm font-semibold" href={catalogHref()}>Browse every bill in the catalog</a>
			</div>
			{#each bills as bill (bill.id)}
				{@const t = demo.billTotals(bill.id)}
				{@const v = latestVersion(bill)}
				<a
					href={billHref(bill.id)}
					class="flex flex-col gap-2 card p-5 text-ink no-underline hover:border-support-line hover:text-ink"
				>
					<div class="flex flex-wrap gap-2 text-[13px] text-muted">
						<span class="font-semibold text-ink">{bill.number}</span><span>·</span><span>
							{bill.status}
						</span><span>·</span><span>v{v.n} {v.type}</span>
					</div>
					<div class="font-display text-xl font-semibold">{bill.title}</div>
					<p class="m-0 text-[15px] leading-normal text-ink-2">{bill.summary}</p>
					<div class="flex flex-wrap gap-4 text-sm">
						<span>
							<strong class="text-support">{fmt(t.support)}</strong>
							<span class="text-muted">support · {pct(t.support, t.reviewers)}%</span>
						</span>
						<span>
							<strong class="text-oppose">{fmt(t.oppose)}</strong>
							<span class="text-muted">oppose · {pct(t.oppose, t.reviewers)}%</span>
						</span>
						<span class="text-muted">{fmt(t.reviewers)} reviewers</span>
					</div>
				</a>
			{/each}
		</section>

		<aside class="flex min-w-0 flex-[1_1_320px] flex-col gap-4">
			<div class="flex flex-col gap-3 card p-5">
				<h2 class="m-0 text-base font-semibold">Demo tour</h2>
				<ol class="m-0 flex list-decimal flex-col gap-2 pl-5 text-sm">
					{#each tour as step (step.text)}
						<li><a href={step.href}>{step.text}</a></li>
					{/each}
				</ol>
				<p class="m-0 text-[13px] text-muted">
					Switch personas from the header to see steward, moderator and office views.
				</p>
			</div>
			<div class="flex flex-col gap-3 card p-5">
				<h2 class="m-0 text-base font-semibold">Issues</h2>
				{#each issues as issue (issue.id)}
					<a href={issueHref(issue.id)} class="font-semibold">{issue.title}</a>
					<p class="m-0 text-sm text-ink-2">{issue.billIds.length} competing bills</p>
				{/each}
			</div>
			<div class="flex flex-col gap-3 card p-5">
				<h2 class="m-0 text-base font-semibold">Demo data</h2>
				<p class="m-0 text-sm text-ink-2">
					Your actions are kept in this browser only. Reset to start a fresh demo.
				</p>
				<button
					type="button"
					class="btn-secondary self-start"
					onclick={() => {
						demo.reset();
						ui.notify('Demo reset to seed data.');
					}}
				>
					Reset demo
				</button>
			</div>
		</aside>
	</div>
</div>
