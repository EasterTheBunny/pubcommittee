<script lang="ts">
	import { billHref, catalogHref, latestSectionHref, sectionHref } from '#lib/links.ts';
	import type { BillPageData } from '#lib/server/bill.ts';
	import type { Citation, Explanation } from '#lib/types.ts';
	import CitationList from './CitationList.svelte';
	import LineList from './LineList.svelte';
	import LineSelectionBar from './LineSelectionBar.svelte';
	import PlaceholderLink from './PlaceholderLink.svelte';
	import StatementsPanel from './StatementsPanel.svelte';
	import UntrackedBill from './UntrackedBill.svelte';

	let { data }: { data: BillPageData } = $props();

	const byTracked = $derived(data.kind === 'tracked' ? data : null);
	const explanationsBySection = $derived.by(() => {
		const m = new Map<string, Explanation[]>();
		for (const e of byTracked?.explanations ?? [])
			m.set(e.section_id, [...(m.get(e.section_id) ?? []), e]);
		return m;
	});
	const open = $derived(
		byTracked && byTracked.citations.ok
			? byTracked.citations.data.filter(
					(c: Citation) => c.state === 'unresolved' || c.state === 'ambiguous'
				)
			: []
	);
	const date = (s: string) => s.slice(0, 10);
</script>

<svelte:head>
	<title>
		{data.kind === 'untracked' ? data.entry.title : `${data.bill.title} · ${data.version.type}`} · PubCommittee
	</title>
</svelte:head>

{#if data.kind === 'untracked'}
	<UntrackedBill entry={data.entry} />
{:else}
	{@const { bill, version } = data}
	<div class="mx-auto max-w-[1100px] px-4 py-6 sm:px-6">
		<nav aria-label="Breadcrumb" class="text-sm text-muted">
			<a href={catalogHref()}>Catalog</a> / {bill.ref.type.toUpperCase()}
			{bill.ref.number}
		</nav>
		<header class="mt-2">
			<h1 class="font-display text-2xl font-bold sm:text-3xl">{bill.title}</h1>
			<p class="mt-1 text-sm text-muted">
				{bill.ref.type.toUpperCase()}
				{bill.ref.number} · {bill.status} · sponsor {bill.sponsor}
				· <span class="chip bg-line-soft text-ink-2">Review {bill.review_state}</span>
			</p>
			{#if bill.official_links.length}
				<ul class="mt-2 flex flex-wrap gap-x-4 text-sm">
					{#each bill.official_links as link (link.url)}
						<li><a href={link.url} rel="noopener noreferrer">{link.label}</a></li>
					{/each}
				</ul>
			{/if}
			{#if bill.tracked_reasons?.length}
				<p class="mt-1 text-sm text-muted">In review because: {bill.tracked_reasons.join('; ')}</p>
			{/if}
		</header>

		<nav aria-label="Versions" class="mt-5">
			<h2 class="eyebrow">Versions</h2>
			<ul class="mt-1 flex flex-wrap gap-2">
				{#each bill.versions as v (v.code)}
					<li>
						<a
							href={billHref(bill.id, v.code)}
							aria-current={v.code === version.code ? 'page' : undefined}
							class="btn {v.code === version.code
								? 'border-ink bg-ink text-white hover:text-white'
								: 'border-line-strong bg-card text-ink hover:bg-subtle hover:text-ink'}"
						>
							{v.type} ({v.code}) · {date(v.published_at)}
						</a>
						{#each v.referrals ?? [] as r (r.code)}
							<p class="referral-note mt-1 text-xs text-muted" data-referral={r.code}>
								{r.type} ({r.code}) · {date(r.published_at)}: same text as this version.
							</p>
						{/each}
					</li>
				{/each}
			</ul>
			{#if !version.address.is_latest}
				<p
					role="status"
					class="mt-2 rounded-lg border border-oppose-line bg-oppose-soft px-3 py-2 text-sm text-oppose-ink"
				>
					You are reading an older version.
					<a href={billHref(bill.id, version.address.latest_version)}>See the current version</a>.
				</p>
			{/if}
		</nav>

		<div class="mt-5 grid gap-4 md:grid-cols-2">
			<StatementsPanel
				title="Bill summary"
				doc={byTracked?.summary.ok ? byTracked.summary.data : null}
				emptyText="No summary is published for this version yet."
			/>
			{#if byTracked?.changes}
				<StatementsPanel
					title="What changed from the previous version"
					doc={byTracked.changes.ok ? byTracked.changes.data : null}
					emptyText="No summary of changes is published for this version yet."
				/>
			{/if}
		</div>

		<nav aria-label="Sections" class="mt-6">
			<h2 class="eyebrow">Sections</h2>
			<ol class="mt-1 columns-1 gap-6 text-sm sm:columns-2">
				{#each version.sections as s (s.id)}
					<li><a href="#{s.id}">{s.label} {s.heading}</a></li>
				{/each}
			</ol>
		</nav>

		<div id="bill-text" class="mt-6 flex flex-col gap-6" data-bill-text>
			{#each version.sections as s (s.id)}
				{@const ex = explanationsBySection.get(s.id) ?? []}
				<section id={s.id} aria-labelledby="{s.id}-h" class="section-block card p-4">
					<h2 id="{s.id}-h" class="font-display text-lg font-semibold">
						<a href="#{s.id}" class="text-ink no-underline hover:text-support">{s.label}</a>
						{s.heading}
					</h2>
					<p class="mt-0.5 text-xs text-muted">
						Address:
						<a href={sectionHref(bill.id, version.code, s.id)}>section page</a>
						{#if version.address.is_latest}
							· <a href={latestSectionHref(bill.id, s.id)}>latest</a>
						{/if}
					</p>
					<div class="mt-3">
						<h3 class="mb-1 eyebrow">Bill text</h3>
						<LineList
							billId={bill.id}
							version={version.code}
							sectionId={s.id}
							lines={s.lines}
							explanations={ex}
						/>
					</div>
				</section>
			{/each}
		</div>

		<div class="mt-8 grid gap-4 md:grid-cols-2">
			{#if byTracked?.citations.ok}
				<CitationList
					citations={open}
					heading="Citations that could not be matched to existing law"
					emptyText="Every citation in this version was matched, or none is published yet."
				/>
			{:else}
				<section class="card p-4">
					<h3 class="eyebrow">Citations of existing law</h3>
					<p class="text-sm text-muted">Citations are not available right now.</p>
				</section>
			{/if}
			<section class="card p-4" aria-label="Coming later">
				<h3 class="eyebrow">Coming later</h3>
				<div class="mt-2 flex flex-wrap gap-2">
					<PlaceholderLink id="notifications">Follow this bill</PlaceholderLink>
					<PlaceholderLink id="share-social">Share to a network</PlaceholderLink>
					<PlaceholderLink id="as-amended">Preview as amended</PlaceholderLink>
				</div>
			</section>
		</div>
	</div>
	<LineSelectionBar billId={bill.id} version={version.code} />
{/if}
