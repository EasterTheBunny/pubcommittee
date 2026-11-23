<script lang="ts">
	import { anchorText, isStale, overlaps, rangeLabel } from '#lib/anchors.ts';
	import FutureButton from '#lib/components/FutureButton.svelte';
	import SideBySide from '#lib/components/SideBySide.svelte';
	import { issueHref, sectionSuggestionsHref } from '#lib/links.ts';
	import { billById, issueById, latestVersion, sectionOf } from '#lib/mock/index.ts';
	import { count as countLabel, demo, fmt, pct } from '#lib/state/demo.svelte.ts';
	import type { ResponseStatus } from '#lib/types.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	const bill = $derived(data.bill);
	const version = $derived(latestVersion(bill));
	const totals = $derived(demo.billTotals(bill.id));
	const isOffice = $derived(demo.persona.role === 'office');

	const reasons = $derived(
		(
			[
				['policy', 'Disagree with policy'],
				['riders', 'Unrelated riders'],
				['complex', 'Too complex']
			] as const
		).map(([k, label]) => ({ label, share: pct(totals.reasons[k], totals.oppose) }))
	);

	const topRider = $derived.by(() => {
		const [id, n] = Object.entries(totals.riderTags).sort((a, b) => b[1] - a[1])[0] ?? [];
		if (!id) return null;
		const section = sectionOf(version, id);
		return section
			? { section, n, share: pct(n, totals.oppose), note: bill.riderNotes?.[id] }
			: null;
	});

	const competitors = $derived(
		bill.issueId
			? issueById(bill.issueId)!
					.billIds.filter((id) => id !== bill.id)
					.map((id) => ({ bill: billById(id)!, t: demo.billTotals(id) }))
			: []
	);

	const live = $derived(
		demo.suggestions.filter((s) => s.anchor.billId === bill.id && !isStale(bill, s.anchor))
	);

	/** Brief-eligible items, grouped by section, ranked by support (design/07). */
	const groups = $derived(
		version.sections
			.map((section) => ({
				section,
				items: live
					.filter((s) => s.anchor.sectionId === section.id && demo.briefEligible(s.id))
					.map((s) => ({ s, count: demo.supportCount(s.id) }))
					.sort((a, b) => b.count - a.count)
			}))
			.filter((g) => g.items.length)
	);

	const topConcern = (sectionId: string) =>
		[...demo.threadsFor(bill.id, version.n).filter((t) => t.anchor.sectionId === sectionId)].sort(
			(a, b) => b.comments.length - a.comments.length
		)[0]?.comments[0];

	const statusDefs: { id: ResponseStatus; label: string }[] = [
		{ id: 'adopted', label: 'Adopted' },
		{ id: 'consider', label: 'Under consideration' },
		{ id: 'declined', label: 'Declined, with reason' }
	];
	const statusLabel = { adopted: 'Adopted', consider: 'Under consideration', declined: 'Declined' };

	let reasonDrafts = $state<Record<string, string>>({});
</script>

<svelte:head><title>Brief · {bill.number}</title></svelte:head>

<div class="border-b border-ink bg-ink text-white">
	<div
		class="mx-auto flex max-w-[1100px] flex-wrap items-center gap-x-6 gap-y-2 px-4 py-3 text-sm sm:px-6"
	>
		<span class="font-display font-semibold">Office brief</span>
		<span class="text-[#d6d9de]">
			{isOffice
				? `Signed in as staff · ${demo.persona.office}`
				: 'Public view of office responses. Switch to the office persona to respond.'}
		</span>
	</div>
</div>

<div class="mx-auto flex max-w-[1100px] flex-col gap-5 px-4 py-6 sm:px-6">
	<div class="flex flex-wrap items-end gap-x-6 gap-y-3">
		<div class="flex flex-[999_1_480px] flex-col gap-1.5">
			<div class="text-[13px] text-muted">Weekly brief · based on v{version.n} {version.type}</div>
			<h1 class="m-0 font-display text-3xl font-semibold">{bill.number} · {bill.title}</h1>
			<div class="text-sm text-muted">
				National totals · every member receives the same figures · {fmt(totals.reviewers)} reviewers
			</div>
		</div>
		<div class="flex flex-wrap gap-2.5">
			<FutureButton id="email-delivery">Email delivery</FutureButton>
			<FutureButton id="export">Export</FutureButton>
		</div>
	</div>

	<section aria-label="Bill-level signal" class="flex flex-wrap gap-6 card p-5">
		<div class="flex flex-[1_1_260px] flex-col gap-1.5">
			<h2 class="m-0 mb-1.5 eyebrow">Bill-level signal</h2>
			<div>
				<span class="font-display text-[26px] font-semibold text-support">
					{fmt(totals.support)}
				</span>
				<span class="text-sm text-muted">support · {pct(totals.support, totals.reviewers)}%</span>
			</div>
			<div>
				<span class="font-display text-[26px] font-semibold text-oppose">{fmt(totals.oppose)}</span>
				<span class="text-sm text-muted">oppose · {pct(totals.oppose, totals.reviewers)}%</span>
			</div>
		</div>
		<div class="flex flex-[2_1_380px] flex-col gap-2.5">
			<div class="text-[13px] font-semibold text-muted">
				Why opposers oppose (reasons may overlap)
			</div>
			<div class="grid grid-cols-[160px_1fr_48px] items-center gap-x-3 gap-y-2 text-sm">
				{#each reasons as r (r.label)}
					<span>{r.label}</span>
					<div class="h-2.5 rounded bg-line-soft">
						<div class="h-2.5 rounded bg-oppose-bar" style:width="{r.share}%"></div>
					</div>
					<span class="text-right">{r.share}%</span>
				{/each}
			</div>
			{#if topRider && topRider.share >= 10}
				<div
					class="rounded-lg border border-oppose-line bg-oppose-tint px-3 py-2.5 text-sm leading-normal"
				>
					<strong>Most-tagged rider:</strong>
					{topRider.section.label}, {topRider.section.heading.toLowerCase().replace(/\.$/, '')},
					tagged by {countLabel(topRider.n)}
					opposers ({topRider.share}%).{topRider.note ? ' Steward note attached.' : ''}
				</div>
			{/if}
		</div>
	</section>

	{#if competitors.length}
		<section
			aria-label="Competing bills"
			class="flex flex-wrap items-center gap-x-6 gap-y-2.5 card px-5 py-4 text-sm"
		>
			<strong>Competing bills</strong>
			{#each competitors as c (c.bill.id)}
				<span>
					{c.bill.number} · <span class="font-semibold text-support">{fmt(c.t.support)}</span> /
					<span class="font-semibold text-oppose">{fmt(c.t.oppose)}</span>
				</span>
			{/each}
			<a href={issueHref(bill.issueId!)} class="ml-auto font-semibold">Neutral comparison</a>
		</section>
	{/if}

	{#each groups as g (g.section.id)}
		{@const concern = topConcern(g.section.id)}
		<section aria-label="Suggested edits on {g.section.label}" class="flex flex-col gap-3.5">
			<h2 class="m-0 font-display text-xl font-semibold">Suggested edits · {g.section.label}</h2>
			{#each g.items as { s, count } (s.id)}
				{@const response = demo.responses[s.id]}
				{@const others = live.filter((o) => o.id !== s.id && overlaps(o.anchor, s.anchor))}
				<article class="overflow-hidden card">
					<div class="flex flex-wrap items-center gap-2.5 border-b border-line px-5 py-3.5">
						<strong class="text-base">
							{g.section.label}, {rangeLabel(s.anchor.from, s.anchor.to)}: {s.title}
						</strong>
						<span class="ml-auto text-[13px] text-muted">
							Status: <strong class="text-ink">
								{response ? statusLabel[response.status] : 'No response'}
							</strong>
						</span>
					</div>
					<div class="border-b border-line">
						<SideBySide
							before={anchorText(bill, s.anchor)}
							after={s.replacement}
							beforeLabel="Bill text (v{s.anchor.version})"
							compact
						/>
					</div>
					<dl class="m-0 grid text-sm leading-normal sm:grid-cols-[220px_1fr]">
						<dt class="border-b border-line-soft px-5 pt-2.5 text-muted sm:py-2.5">
							Why, in the author’s words
						</dt>
						<dd class="m-0 border-b border-line-soft px-5 py-2.5" data-content-class="user">
							{s.reason}
						</dd>
						<dt class="border-b border-line-soft px-5 pt-2.5 text-muted sm:py-2.5">
							Support, whole jurisdiction
						</dt>
						<dd class="m-0 border-b border-line-soft px-5 py-2.5">
							<strong>{fmt(count)}</strong> residents · {pct(count, totals.reviewers)}% of reviewers
						</dd>
						<dt class="border-b border-line-soft px-5 pt-2.5 text-muted sm:py-2.5">
							Support, your district
						</dt>
						<dd
							class="m-0 flex flex-wrap items-center gap-2 border-b border-line-soft px-5 py-2.5 text-muted"
						>
							Available after address confirmation.
							<FutureButton id="district-breakdown" class="min-h-8 px-2.5 text-xs">
								District breakdowns
							</FutureButton>
						</dd>
						<dt class="border-b border-line-soft px-5 pt-2.5 text-muted sm:py-2.5">
							Competing suggestions
						</dt>
						<dd class="m-0 border-b border-line-soft px-5 py-2.5">
							{#if others.length}
								{others.length} other{others.length === 1 ? '' : 's'} on these lines ·
								{others.map((o) => countLabel(demo.supportCount(o.id))).join(' and ')} supporters
							{:else}None{/if}
						</dd>
						<dt class="border-b border-line-soft px-5 pt-2.5 text-muted sm:py-2.5">
							Top concern on this section
						</dt>
						<dd
							class="m-0 border-b border-line-soft px-5 py-2.5"
							data-content-class={concern ? 'user' : 'platform'}
						>
							{concern ? `“${concern.body}”` : 'No line comments yet'}
						</dd>
						<dt class="border-b border-line-soft px-5 pt-2.5 text-muted sm:py-2.5">
							Steward review
						</dt>
						<dd class="m-0 border-b border-line-soft px-5 py-2.5">
							{s.wellFormed ? 'Well formed · no cross-reference conflicts' : 'Not reviewed'}
						</dd>
						{#if s.integrityNote}
							<dt class="px-5 pt-2.5 text-muted sm:py-2.5">Integrity note</dt>
							<dd class="m-0 px-5 py-2.5">{s.integrityNote}</dd>
						{/if}
					</dl>
					<div
						class="flex flex-wrap items-center gap-2.5 border-t border-line bg-subtle px-5 py-3.5"
					>
						{#if isOffice}
							<span class="mr-1 text-sm font-semibold">Your public response</span>
							{#each statusDefs as d (d.id)}
								<button
									type="button"
									aria-pressed={response?.status === d.id}
									class={response?.status === d.id
										? 'btn border-ink bg-ink text-white'
										: 'btn-secondary'}
									onclick={() =>
										response?.status === d.id
											? demo.clearResponse(s.id)
											: demo.respond(s.id, d.id, reasonDrafts[s.id] ?? '')}
								>
									{d.label}
								</button>
							{/each}
							{#if response?.status === 'declined'}
								<label class="flex w-full flex-col gap-1 text-sm font-semibold">
									Reason (public)
									<input
										type="text"
										class="min-h-11 field font-normal"
										value={response.reason}
										oninput={(e) => demo.respond(s.id, 'declined', e.currentTarget.value)}
									/>
								</label>
							{/if}
							<span class="flex-[1_1_220px] text-[13px] text-muted">
								Optional, but silence shows as “no response” beside the member’s name.
							</span>
						{:else}
							<span class="text-sm text-muted">
								{response
									? `${response.office}: ${statusLabel[response.status]}${response.reason ? `. ${response.reason}` : ''}`
									: 'No office has responded yet.'}
							</span>
							<a
								class="ml-auto text-sm font-semibold"
								href={sectionSuggestionsHref(
									bill.id,
									version.n,
									s.anchor.sectionId,
									s.anchor.from,
									s.anchor.to
								)}
							>
								View in review
							</a>
						{/if}
					</div>
				</article>
			{/each}
		</section>
	{:else}
		<div class="card p-5 text-sm text-muted">
			No suggestions have reached the brief minimum yet.
		</div>
	{/each}

	<div class="flex flex-wrap gap-2.5">
		<FutureButton id="responsiveness-record">Responsiveness record</FutureButton>
		<FutureButton id="amendment-matching">Closing the loop</FutureButton>
		<FutureButton id="audit">Count audit</FutureButton>
		<FutureButton id="automation-disclosure">Automation disclosure</FutureButton>
	</div>
</div>
