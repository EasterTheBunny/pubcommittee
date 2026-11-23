<script lang="ts">
	import CatalogEntryCard from '#lib/components/CatalogEntryCard.svelte';
	import { catalogHref } from '#lib/links.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head>
	<title>Catalog · PubCommittee</title>
</svelte:head>

<div class="mx-auto max-w-[1000px] px-4 py-8 sm:px-6">
	<h1 class="font-display text-2xl font-bold">Catalog</h1>
	<p class="mt-2 text-[15px] text-ink-2">
		Every bill, tracked or not. Only bills in review show their text here; every other bill links to
		the legislature's own page.
	</p>

	<form method="GET" action={catalogHref()} class="mt-6 grid gap-4 card p-4 sm:grid-cols-2">
		<div class="flex flex-col gap-1">
			<label for="catalog-q" class="text-sm font-semibold">Search by number or title</label>
			<input id="catalog-q" name="q" type="text" class="field" value={data.filters.q} />
		</div>
		<div class="flex flex-col gap-1">
			<label for="catalog-status" class="text-sm font-semibold">Status</label>
			<input
				id="catalog-status"
				name="status"
				type="text"
				class="field"
				value={data.filters.status}
			/>
		</div>
		<div class="flex flex-col gap-1">
			<label for="catalog-chamber" class="text-sm font-semibold">Chamber</label>
			<select id="catalog-chamber" name="chamber" class="field">
				<option value="" selected={data.filters.chamber === ''}>Any</option>
				<option value="house" selected={data.filters.chamber === 'house'}>House</option>
				<option value="senate" selected={data.filters.chamber === 'senate'}>Senate</option>
				<option value="joint" selected={data.filters.chamber === 'joint'}>Joint</option>
			</select>
		</div>
		<div class="flex flex-col gap-1">
			<label for="catalog-committee" class="text-sm font-semibold">Committee action</label>
			<select id="catalog-committee" name="committee_activity" class="field">
				<option value="" selected={data.filters.committee_activity === ''}>Any</option>
				<option value="true" selected={data.filters.committee_activity === 'true'}>
					With committee action
				</option>
				<option value="false" selected={data.filters.committee_activity === 'false'}>
					Without committee action
				</option>
			</select>
		</div>
		<div class="flex flex-col gap-1">
			<label for="catalog-subject" class="text-sm font-semibold">Subject</label>
			<input
				id="catalog-subject"
				name="subject"
				type="text"
				class="field"
				value={data.filters.subject}
			/>
		</div>
		<div class="flex items-end">
			<button type="submit" class="btn-primary">Search</button>
		</div>
	</form>

	<section aria-labelledby="coverage-heading" class="mt-8">
		<h2 id="coverage-heading" class="font-display text-xl font-bold">Coverage on this page</h2>
		<dl class="mt-3 grid grid-cols-2 gap-3 text-[15px] sm:grid-cols-4">
			<div class="card p-3">
				<dt class="text-sm text-muted">Entries</dt>
				<dd class="font-display text-xl font-bold">{data.breakdown.total}</dd>
			</div>
			<div class="card p-3">
				<dt class="text-sm text-muted">In review</dt>
				<dd class="font-display text-xl font-bold">{data.breakdown.inReview}</dd>
			</div>
			<div class="card p-3">
				<dt class="text-sm text-muted">Eligible for review</dt>
				<dd class="font-display text-xl font-bold">{data.breakdown.eligible}</dd>
			</div>
			<div class="card p-3">
				<dt class="text-sm text-muted">Not a candidate</dt>
				<dd class="font-display text-xl font-bold">{data.breakdown.notCandidate}</dd>
			</div>
		</dl>
	</section>

	<section aria-label="Bills" class="mt-8">
		{#if data.entries.length === 0}
			<p class="text-[15px] text-muted">No bills match these filters.</p>
		{:else}
			<ul class="flex flex-col gap-3">
				{#each data.entries as entry (entry.id)}
					<li>
						<CatalogEntryCard {entry} />
					</li>
				{/each}
			</ul>
		{/if}
		{#if data.nextHref}
			<p class="mt-6">
				<a href={catalogHref() + data.nextHref} rel="next" class="btn-secondary">Next page</a>
			</p>
		{/if}
	</section>
</div>
