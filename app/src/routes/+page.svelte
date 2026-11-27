<script lang="ts">
	import { billHref, catalogHref, policyHref } from '#lib/links.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head>
	<title>PubCommittee: bills in review</title>
</svelte:head>

<div class="mx-auto max-w-[1000px] px-4 py-8 sm:px-6">
	<section aria-labelledby="scope-heading" class="card p-5">
		<h1 id="scope-heading" class="font-display text-2xl font-bold">Bills in review</h1>
		<p class="mt-2 text-[15px] text-ink-2">
			PubCommittee shows the literal text of active bills, line by line, beside plain explanations
			of what each change does. The two are kept apart and every explanation says how it was made.
		</p>
		<h2 class="mt-4 text-base font-semibold">Which bills are in review</h2>
		{#if data.policy}
			<ul
				class="mt-1 list-disc pl-5 text-[15px] text-ink-2"
				data-content-class={data.policy.content_class}
			>
				{#each data.policy.rules as rule (rule)}
					<li>{rule}</li>
				{/each}
			</ul>
			<p class="mt-2 text-sm text-muted">
				Selection policy version {data.policy.policy_version}.
				<a href={policyHref()}>Read the full policy</a>.
			</p>
		{:else}
			<p class="mt-1 text-sm text-muted">
				The selection policy is not available right now. <a href={policyHref()}>
					Try the policy page
				</a>.
			</p>
		{/if}
		<p class="mt-3 text-sm">
			Looking for another bill? <a href={catalogHref()}>Browse the full catalog</a>, which links to
			the legislature's own page for bills that are not in review.
		</p>
	</section>

	<section aria-labelledby="tracked-heading" class="mt-8">
		<h2 id="tracked-heading" class="font-display text-xl font-bold">Tracked bills</h2>
		{#if data.bills.length === 0}
			<p class="mt-3 text-[15px] text-muted">No bills are in review yet.</p>
		{:else}
			<ul class="mt-3 flex flex-col gap-3">
				{#each data.bills as bill (bill.id)}
					<li class="card p-4">
						<a href={billHref(bill.id)} class="font-display text-lg font-semibold">{bill.title}</a>
						<div class="mt-1 text-sm text-muted">
							{bill.ref.type.toUpperCase()}
							{bill.ref.number} · {bill.ref.jurisdiction.toUpperCase()}
							session {bill.ref.session} · {bill.status}{#if bill.chamber}
								· {bill.chamber}{/if}{#if bill.latest_version}
								· latest version {bill.latest_version}{/if}
						</div>
					</li>
				{/each}
			</ul>
			{#if data.next}
				<p class="mt-4 text-sm text-muted">
					More bills are in the <a href={catalogHref()}>catalog</a>.
				</p>
			{/if}
		{/if}
	</section>
</div>
