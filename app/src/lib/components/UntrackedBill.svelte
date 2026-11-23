<script lang="ts">
	import { catalogHref } from '#lib/links.ts';
	import type { CatalogEntry } from '#lib/types.ts';

	// A catalog entry that is not in review: links to the legislature's pages and never shows text.
	let { entry }: { entry: CatalogEntry } = $props();
</script>

<div class="mx-auto max-w-[800px] px-4 py-8 sm:px-6">
	<article class="card p-5" data-content-class={entry.content_class}>
		<h1 class="font-display text-2xl font-bold">{entry.title}</h1>
		<p class="mt-1 text-sm text-muted">
			{entry.ref.type.toUpperCase()}
			{entry.ref.number}{#if entry.status}
				· {entry.status}{/if}
		</p>
		<p class="mt-3 text-[15px] text-ink-2">
			This bill is not in review, so PubCommittee does not show its text. Read it on the
			legislature's own pages.
		</p>
		{#if entry.exclusion_reason}
			<p class="mt-2 text-sm text-muted">Why it is not a candidate: {entry.exclusion_reason}</p>
		{/if}
		<h2 class="mt-4 eyebrow">Official pages</h2>
		{#if entry.official_links.length}
			<ul class="mt-1 list-disc pl-5">
				{#each entry.official_links as link (link.url)}
					<li><a href={link.url} rel="noopener noreferrer">{link.label}</a></li>
				{/each}
			</ul>
		{:else}
			<p class="text-sm text-muted">No official page listed.</p>
		{/if}
		<p class="mt-4 text-sm"><a href={catalogHref()}>Back to the catalog</a></p>
	</article>
</div>
