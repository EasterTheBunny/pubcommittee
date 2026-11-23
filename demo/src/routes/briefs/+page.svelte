<script lang="ts">
	import { briefHref } from '#lib/links.ts';
	import { bills } from '#lib/mock/index.ts';
	import { demo } from '#lib/state/demo.svelte.ts';
</script>

<svelte:head><title>Briefs · PubCommittee</title></svelte:head>

<div class="mx-auto flex max-w-[1100px] flex-col gap-5 px-4 py-8 sm:px-6">
	<h1 class="m-0 font-display text-3xl font-semibold">Office briefs</h1>
	<p class="m-0 max-w-3xl text-ink-2">
		Each office receives a short brief per bill, written like an amendment memo. At launch every
		member receives the same national figures.
		{#if demo.persona.role !== 'office'}Switch to the office persona to respond to items.{/if}
	</p>
	{#each bills as bill (bill.id)}
		<a
			href={briefHref(bill.id)}
			class="flex flex-wrap items-center gap-3 card p-5 text-ink no-underline hover:border-support-line hover:text-ink"
		>
			<span class="font-semibold">{bill.number}</span>
			<span>{bill.title}</span>
			<span class="ml-auto text-sm font-semibold text-support">Open brief</span>
		</a>
	{/each}
</div>
