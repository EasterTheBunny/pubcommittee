<script lang="ts">
	import { logHref } from '#lib/links.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head>
	<title>Selection policy · PubCommittee</title>
</svelte:head>

<div class="mx-auto max-w-[1000px] px-4 py-8 sm:px-6">
	<section aria-labelledby="policy-heading" class="card p-5">
		<h1 id="policy-heading" class="font-display text-2xl font-bold">Selection policy</h1>
		<p class="mt-2 text-[15px] text-ink-2">
			These are the only rules that decide which bills are reviewed here. Every decision is
			published in the public log, with its reason.
		</p>
		<ol
			class="mt-4 list-decimal pl-5 text-[15px] text-ink-2"
			data-content-class={data.policy.content_class}
		>
			{#each data.policy.rules as rule}
				<li class="mt-1">{rule}</li>
			{/each}
		</ol>
		<p class="mt-4 text-sm text-muted">
			Policy version {data.policy.policy_version}, as of {data.policy.as_of.slice(0, 10)}.
		</p>
		<p class="mt-2 text-sm">
			<a href={logHref()}>Read the public log</a>
		</p>
	</section>
</div>
