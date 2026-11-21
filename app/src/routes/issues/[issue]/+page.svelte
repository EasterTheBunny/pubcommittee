<script lang="ts">
	import FutureButton from '#lib/components/FutureButton.svelte';
	import { billHref } from '#lib/links.ts';
	import { billById } from '#lib/mock/index.ts';
	import { demo, fmt, pct } from '#lib/state/demo.svelte.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	const issue = $derived(data.issue);
	const members = $derived(issue.billIds.map((id) => billById(id)!));
</script>

<svelte:head><title>{issue.title} · Issues</title></svelte:head>

<div class="mx-auto flex max-w-[1200px] flex-col gap-5 px-4 py-6 sm:px-6">
	<div class="flex flex-col gap-2">
		<div class="text-[13px] text-muted">Issue · tagged {issue.tags.join(', ')}</div>
		<h1 class="m-0 font-display text-[32px] font-semibold">{issue.title}</h1>
		<p class="m-0 max-w-3xl text-base leading-relaxed text-ink-2">{issue.summary}</p>
	</div>

	<section aria-label="Neutral comparison" class="flex flex-col gap-2.5 card p-5">
		<div class="flex flex-wrap items-center gap-2">
			<h2 class="m-0 text-base font-semibold">How they differ</h2>
			{#if issue.twoPartySignOff}
				<span class="chip bg-ok-soft text-ok">Signed off by stewards from 2 parties</span>
			{/if}
		</div>
		<p class="m-0 text-[15px] leading-relaxed">{issue.comparison}</p>
	</section>

	<div class="overflow-x-auto card">
		<table class="w-full min-w-[760px] border-collapse text-sm">
			<thead>
				<tr class="border-b border-line text-left">
					<th scope="col" class="w-[22%] px-4.5 py-3.5"><span class="sr-only">Attribute</span></th>
					{#each members as b (b.id)}
						<th scope="col" class="px-4.5 py-3.5 align-top">
							<a href={billHref(b.id)} class="font-display text-[17px] font-semibold">{b.number}</a>
							<div class="text-[13px] font-normal text-muted">{b.title}</div>
						</th>
					{/each}
				</tr>
			</thead>
			<tbody class="leading-normal">
				{#each issue.rows as row (row.label)}
					<tr class="border-b border-line-soft">
						<th scope="row" class="px-4.5 py-3.5 text-left font-semibold">{row.label}</th>
						{#each members as b (b.id)}
							<td class="px-4.5 py-3.5">
								{#if row.label === 'Sections outside the issue' && row.values[b.id] !== 'None'}
									<span class="chip bg-oppose-soft text-oppose-ink">{row.values[b.id]}</span>
								{:else}
									{row.values[b.id]}
								{/if}
							</td>
						{/each}
					</tr>
				{/each}
				{#each [{ label: 'Support', key: 'support' as const, cls: 'text-support' }, { label: 'Oppose', key: 'oppose' as const, cls: 'text-oppose' }] as r (r.key)}
					<tr class="border-b border-line-soft">
						<th scope="row" class="px-4.5 py-3.5 text-left font-semibold">{r.label}</th>
						{#each members as b (b.id)}
							{@const t = demo.billTotals(b.id)}
							<td class="px-4.5 py-3.5">
								<strong class={r.cls}>{fmt(t[r.key])}</strong>
								<span class="text-muted">· {pct(t[r.key], t.reviewers)}%</span>
							</td>
						{/each}
					</tr>
				{/each}
				<tr>
					<th scope="row" class="px-4.5 py-3.5 text-left font-semibold">Reviewers</th>
					{#each members as b (b.id)}
						<td class="px-4.5 py-3.5 text-muted">{fmt(demo.billTotals(b.id).reviewers)}</td>
					{/each}
				</tr>
			</tbody>
		</table>
	</div>

	<section aria-label="Linked bills" class="flex flex-col gap-2.5 card p-5">
		<h2 class="m-0 text-base font-semibold">Linked bills</h2>
		{#each issue.links as link (link.from + link.to)}
			<div class="flex flex-wrap items-center gap-2.5 text-sm">
				<span class="font-semibold">{link.from}</span>
				<svg
					width="40"
					height="12"
					viewBox="0 0 40 12"
					fill="none"
					stroke="currentColor"
					stroke-width="1.5"
					class="text-faint"
					aria-hidden="true"
				>
					<path d="M0 6h38M32 1l6 5-6 5" />
				</svg>
				<span class="font-semibold">{link.to}</span>
				<span class="text-muted">{link.note}</span>
			</div>
		{/each}
		<FutureButton id="lineage-candidates" class="self-start">Suggested links</FutureButton>
	</section>
</div>
