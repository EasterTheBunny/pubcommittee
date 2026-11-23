<script lang="ts">
	import { issueHref } from '#lib/links.ts';
	import { billById, issues } from '#lib/mock/index.ts';
</script>

<svelte:head><title>Issues · PubCommittee</title></svelte:head>

<div class="mx-auto flex max-w-[1200px] flex-col gap-5 px-4 py-8 sm:px-6">
	<h1 class="m-0 font-display text-3xl font-semibold">Issues</h1>
	<p class="m-0 max-w-3xl text-ink-2">
		Bills that tackle the same issue differently are grouped here and compared in neutral terms.
		Each keeps its own review and signals.
	</p>
	{#each issues as issue (issue.id)}
		<a
			href={issueHref(issue.id)}
			class="flex flex-col gap-2 card p-5 text-ink no-underline hover:border-support-line hover:text-ink"
		>
			<div class="text-[13px] text-muted">{issue.tags.join(' · ')}</div>
			<div class="font-display text-xl font-semibold">{issue.title}</div>
			<div class="text-sm text-ink-2">
				{issue.billIds.map((id) => billById(id)?.number).join(' · ')}
			</div>
		</a>
	{/each}
</div>
