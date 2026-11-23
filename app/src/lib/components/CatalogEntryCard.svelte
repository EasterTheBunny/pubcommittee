<script lang="ts">
	import { billHref } from '#lib/links.ts';
	import type { CatalogEntry } from '#lib/types.ts';
	import PlaceholderLink from './PlaceholderLink.svelte';

	// One catalog entry. Only tracked bills link into the app; no bill text is shown here.
	let { entry }: { entry: CatalogEntry } = $props();

	const markupLabel = {
		'in-review': 'In review',
		eligible: 'Eligible for review',
		'not-a-candidate': 'Not a candidate'
	} as const;
</script>

<article class="card p-4" data-content-class={entry.content_class}>
	<h2 class="font-display text-lg font-semibold">
		{#if entry.tracked}
			<a href={billHref(entry.id)}>{entry.title}</a>
		{:else}
			{entry.title}
		{/if}
	</h2>

	<p class="mt-1 text-sm text-muted">
		{entry.ref.type.toUpperCase()}
		{entry.ref.number}{#if entry.status}
			· {entry.status}{/if}{#if entry.chamber}
			· {entry.chamber}{/if}
	</p>

	<p class="mt-2 flex flex-wrap items-center gap-2">
		<span class="chip">{markupLabel[entry.markup_status]}</span>
	</p>

	{#if entry.subjects?.length}
		<ul class="mt-2 flex flex-wrap gap-2" aria-label="Subjects">
			{#each entry.subjects as subject (subject)}
				<li class="chip bg-line-soft text-ink-2">{subject}</li>
			{/each}
		</ul>
	{/if}

	{#if entry.reasons?.length}
		<h3 id="reasons-{entry.id}" class="mt-3 eyebrow">Why it is in review</h3>
		<ul aria-labelledby="reasons-{entry.id}" class="mt-1 list-disc pl-5 text-[15px] text-ink-2">
			{#each entry.reasons as reason (reason)}
				<li>{reason}</li>
			{/each}
		</ul>
	{/if}

	{#if entry.exclusion_reason}
		<p class="mt-3 text-[15px] text-ink-2">Why it is not a candidate: {entry.exclusion_reason}</p>
	{/if}

	<h3 id="official-{entry.id}" class="mt-3 eyebrow">Official pages</h3>
	{#if entry.official_links.length}
		<ul aria-labelledby="official-{entry.id}" class="mt-1 flex flex-col gap-1 text-[15px]">
			{#each entry.official_links as link (link.url)}
				<li>
					<a href={link.url} rel="noopener noreferrer">{link.label}</a>
				</li>
			{/each}
		</ul>
	{:else}
		<p class="mt-1 text-sm text-muted">No official page listed.</p>
	{/if}

	{#if !entry.tracked && entry.markup_status === 'eligible'}
		<p class="mt-3">
			<PlaceholderLink id="markup-decision">Ask for markup</PlaceholderLink>
		</p>
	{/if}
</article>
