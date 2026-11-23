<script lang="ts">
	import { logHref } from '#lib/links.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head>
	<title>Public log · PubCommittee</title>
</svelte:head>

<div class="mx-auto max-w-[1000px] px-4 py-8 sm:px-6">
	<section aria-labelledby="log-heading" class="card p-5">
		<h1 id="log-heading" class="font-display text-2xl font-bold">Public log</h1>
		<p class="mt-2 text-[15px] text-ink-2">
			Every administrator action is listed here with the reason given. Removed content is never
			republished.
		</p>
	</section>

	<section aria-label="Logged actions" class="mt-6">
		{#if data.entries.length === 0}
			<p class="text-[15px] text-muted">No actions have been logged yet.</p>
		{:else}
			<ol class="flex flex-col gap-3">
				{#each data.entries as entry (entry.id)}
					<li class="card p-4" data-content-class={entry.content_class}>
						<div class="flex flex-wrap items-center gap-2 text-sm text-muted">
							<time datetime={entry.at}>{entry.at}</time>
							<span class="chip bg-line-soft text-ink-2">{entry.kind}</span>
						</div>
						<p class="mt-2 font-semibold">{entry.subject}</p>
						<p class="mt-1 text-[15px] text-ink-2">Reason: {entry.reason}</p>
					</li>
				{/each}
			</ol>
			{#if data.nextHref}
				<p class="mt-4">
					<a href={logHref() + data.nextHref} rel="next" class="btn-secondary">Older entries</a>
				</p>
			{/if}
		{/if}
	</section>
</div>
