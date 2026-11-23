<script lang="ts">
	import FutureButton from '#lib/components/FutureButton.svelte';
	import { billHref } from '#lib/links.ts';
	import {
		catalog,
		REQUEST_THRESHOLD,
		seedRequestReasons,
		selectionPolicy,
		topicLabel
	} from '#lib/mock/index.ts';
	import { count, demo } from '#lib/state/demo.svelte.ts';
	import { ui } from '#lib/state/ui.svelte.ts';
	import type { CatalogEntry, MarkupStatus } from '#lib/types.ts';

	// Public bill catalog and markup requests (design/01). Every bill is listed; only
	// tracked bills have review pages. Untracked text lives on the legislature's site.

	const statusLabel: Record<MarkupStatus, string> = {
		in_review: 'In review',
		eligible: 'Eligible',
		not_candidate: 'Not a markup candidate'
	};
	const statusClass: Record<MarkupStatus, string> = {
		in_review: 'bg-support-soft text-support-strong',
		eligible: 'bg-line-soft text-ink-2',
		not_candidate: 'bg-oppose-soft text-oppose-ink'
	};

	let status = $state<MarkupStatus | 'all'>('all');
	let chamber = $state<'all' | 'House' | 'Senate'>('all');
	let q = $state('');

	const shown = $derived(
		catalog.filter((c) => {
			if (status !== 'all' && c.markupStatus !== status) return false;
			if (chamber !== 'all' && c.chamber !== chamber) return false;
			const needle = q.trim().toLowerCase();
			if (!needle) return true;
			return [c.number, c.title, ...c.subjects].join(' ').toLowerCase().includes(needle);
		})
	);

	// Coverage disclosure (design/09 Coverage neutrality).
	const tracked = catalog.filter((c) => c.tracked);
	const byTopic = selectionPolicy.topics.map((t) => ({
		label: topicLabel[t],
		n: tracked.filter((c) => c.topic === t).length
	}));
	const byParty = (['A', 'B'] as const).map((p) => ({
		label: `Party ${p}`,
		n: tracked.filter((c) => c.sponsorParty === p).length
	}));
	const byAdmin = tracked.filter((c) => c.tracked?.byAdministrator).length;

	const queue = $derived(catalog.filter((c) => demo.inStewardQueue(c.id)));

	function request(c: CatalogEntry) {
		if (!ui.requireResident()) return;
		const had = demo.hasRequested(c.id);
		demo.toggleRequest(c.id);
		ui.notify(
			had ? 'Request withdrawn.' : 'Requested. Your request is private; only the total is shown.'
		);
	}
</script>

<svelte:head><title>Bill catalog · PubCommittee</title></svelte:head>

<div class="mx-auto flex max-w-[1200px] flex-col gap-5 px-4 py-8 sm:px-6">
	<div class="flex flex-col gap-2">
		<h1 class="m-0 font-display text-3xl font-semibold">Bill catalog</h1>
		<p class="m-0 max-w-3xl text-ink-2" data-content-class="platform">
			Every bill in this session. Bills in review open here for line-by-line markup; for the rest,
			the official text is on the legislature's own site. You can ask for an eligible bill to be
			brought into review.
		</p>
	</div>

	<section
		aria-label="Current scope"
		class="flex flex-wrap items-start gap-x-8 gap-y-3 card p-5 text-sm"
		data-content-class="platform"
	>
		<div class="flex min-w-0 flex-[2_1_360px] flex-col gap-1.5">
			<div class="eyebrow">Current scope · selection policy v{selectionPolicy.version}</div>
			<p class="m-0 leading-relaxed">
				Pilot: bills with committee action beyond referral on
				<strong>
					{selectionPolicy.topics.map((t) => topicLabel[t].toLowerCase()).join(', ')}
				</strong>, plus companions and approved requests. Selection never uses sponsor party or
				position.
			</p>
			<div class="flex flex-wrap gap-2">
				<FutureButton id="moderation-log" class="min-h-8 px-2.5 text-xs">
					Policy changes in the public log
				</FutureButton>
			</div>
		</div>
		<dl class="m-0 grid flex-[1_1_260px] grid-cols-[auto_1fr] gap-x-4 gap-y-1">
			<dt class="text-muted">In review</dt>
			<dd class="m-0 font-semibold">{tracked.length} bills</dd>
			<dt class="text-muted">By topic</dt>
			<dd class="m-0">{byTopic.map((t) => `${t.label} ${t.n}`).join(' · ')}</dd>
			<dt class="text-muted">By sponsor party</dt>
			<dd class="m-0">{byParty.map((p) => `${p.label} ${p.n}`).join(' · ')}</dd>
			<dt class="text-muted">Selected by an administrator</dt>
			<dd class="m-0">{byAdmin} (early pilot; each in the public log)</dd>
		</dl>
	</section>

	{#if demo.persona.role === 'steward'}
		<section
			aria-label="Markup request queue"
			class="flex flex-col gap-3 rounded-xl border border-support-tint bg-[#f3f7ff] p-5"
		>
			<h2 class="m-0 text-sm font-semibold text-support-strong">
				Steward queue · requests at or above {REQUEST_THRESHOLD}
			</h2>
			{#each queue as c (c.id)}
				<div class="flex flex-col gap-2 rounded-lg bg-card p-3.5 text-sm">
					<div>
						<strong>{c.number}</strong> · {c.title} · {count(demo.requestCount(c.id))} requests
					</div>
					{#if seedRequestReasons[c.id]?.length}
						<ul class="m-0 flex flex-col gap-1 pl-5 text-ink-2" data-content-class="user">
							{#each seedRequestReasons[c.id] as r (r)}<li>{r}</li>{/each}
						</ul>
						<div class="text-xs text-muted">Reasons shown without names. Never published.</div>
					{/if}
					<div class="flex flex-wrap gap-2">
						<FutureButton id="markup-decision">
							Accept · needs a steward from another party
						</FutureButton>
						<FutureButton id="markup-decision">Decline with reason</FutureButton>
					</div>
				</div>
			{:else}
				<p class="m-0 text-sm text-ink-2">No bills waiting for a decision.</p>
			{/each}
		</section>
	{/if}

	<div class="flex flex-wrap items-end gap-3">
		<label class="flex min-w-60 flex-1 flex-col gap-1 text-sm font-semibold">
			Search number, title, or subject
			<input type="search" bind:value={q} class="min-h-11 field font-normal" />
		</label>
		<label class="flex flex-col gap-1 text-sm font-semibold">
			Status
			<select bind:value={status} class="min-h-11 field font-normal">
				<option value="all">All</option>
				<option value="in_review">In review</option>
				<option value="eligible">Eligible</option>
				<option value="not_candidate">Not a markup candidate</option>
			</select>
		</label>
		<label class="flex flex-col gap-1 text-sm font-semibold">
			Chamber
			<select bind:value={chamber} class="min-h-11 field font-normal">
				<option value="all">Both</option>
				<option value="House">House</option>
				<option value="Senate">Senate</option>
			</select>
		</label>
	</div>

	<p class="m-0 text-[13px] text-muted">{shown.length} of {catalog.length} bills</p>

	{#each shown as c (c.id)}
		{@const n = demo.requestCount(c.id)}
		{@const mine = demo.hasRequested(c.id)}
		<article id={c.id} class="flex flex-col gap-3 card p-5" data-content-class="platform">
			<div class="flex flex-wrap items-center gap-2 text-[13px] text-muted">
				<span class="font-semibold text-ink">{c.number}</span><span>·</span><span>{c.chamber}</span>
				<span>·</span><span>{c.sponsor}</span><span>·</span><span>{c.latestAction}</span>
				<span class="ml-auto chip {statusClass[c.markupStatus]}">
					{statusLabel[c.markupStatus]}
				</span>
			</div>
			<h2 class="m-0 font-display text-xl font-semibold" data-content-class="official">
				{c.title}
			</h2>
			<div class="flex flex-wrap gap-1.5 text-xs">
				{#each c.subjects as s (s)}<span class="chip bg-line-soft text-ink-2">{s}</span>{/each}
				<span class="chip bg-subtle text-muted">
					{c.committeeActivity.length
						? `Committee: ${c.committeeActivity.join(', ').toLowerCase()}`
						: 'Referred only'}
				</span>
			</div>

			<div class="flex flex-wrap items-center gap-2">
				{#if c.tracked}
					<a class="btn-primary" href={billHref(c.tracked.billId)}>Open review</a>
				{/if}
				<FutureButton id="official-links">Official page</FutureButton>
				{#each c.officialVersions as v (v)}
					<FutureButton id="official-links" class="min-h-8 px-2.5 text-xs">Text: {v}</FutureButton>
				{/each}
			</div>

			{#if c.tracked}
				<p class="m-0 text-sm text-ink-2">
					<span class="font-semibold text-ink">Why it is in review:</span>
					{c.tracked.reasons.join(' · ')} · policy v{c.tracked.policy}
				</p>
			{:else if c.markupStatus === 'not_candidate'}
				<p class="m-0 text-sm text-ink-2">{c.exclusionReason}</p>
			{:else}
				{#if c.decision}
					<p class="m-0 rounded-lg border border-line bg-subtle px-3 py-2 text-sm">
						<strong>Request {c.decision.outcome}</strong> · {c.decision.date} · {c.decision.reason}
					</p>
				{/if}
				<div class="flex flex-wrap items-center gap-3">
					<button
						type="button"
						class={mine ? 'btn-primary' : 'btn-outline'}
						aria-pressed={mine}
						disabled={!!c.decision}
						onclick={() => request(c)}
					>
						{mine ? 'Requested' : 'Request markup'}
					</button>
					<span class="text-sm text-muted">
						{count(n)} request{n === 1 ? '' : 's'} · steward review at {REQUEST_THRESHOLD}
					</span>
					{#if mine}
						<FutureButton id="request-notify" class="min-h-8 px-2.5 text-xs">
							Tell me the outcome
						</FutureButton>
					{/if}
				</div>
				<p class="m-0 text-xs text-muted">
					Who requested is private, even from stewards. Requests are not a signal about the bill and
					are not sent to legislators.
				</p>
			{/if}
		</article>
	{:else}
		<div class="card p-5 text-sm text-muted">No bills match.</div>
	{/each}
</div>
