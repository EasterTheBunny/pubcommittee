<script lang="ts">
	import { onMount } from 'svelte';
	import { anchorText, isStale, overlaps, parseHash, rangeLabel } from '#lib/anchors.ts';
	import CommentThread from '#lib/components/CommentThread.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import SideBySide from '#lib/components/SideBySide.svelte';
	import SuggestionCard from '#lib/components/SuggestionCard.svelte';
	import { billHref, linesHref } from '#lib/links.ts';
	import { BRIEF_MINIMUM, latestVersion } from '#lib/mock/index.ts';
	import { demo } from '#lib/state/demo.svelte.ts';
	import { ui } from '#lib/state/ui.svelte.ts';
	import type { Anchor } from '#lib/types.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	const bill = $derived(data.bill);
	const version = $derived(data.version);
	const section = $derived(data.section);
	const isLatest = $derived(version.n === latestVersion(bill).n);

	// Line range from the URL hash (e.g. #s4-l12-13), or the whole section.
	let from = $state(0);
	let to = $state(0);
	let filtered = $state(false);

	function readHash() {
		const h = parseHash(location.hash);
		if (h?.sectionId === section.id && h.from !== undefined) {
			from = h.from;
			to = h.to ?? h.from;
			filtered = true;
		}
	}

	$effect.pre(() => {
		from = section.lines[0].n;
		to = section.lines[0].n;
	});

	onMount(() => {
		readHash();
		window.addEventListener('hashchange', readHash);
		return () => window.removeEventListener('hashchange', readHash);
	});

	const range = $derived<Anchor>({
		billId: bill.id,
		version: version.n,
		sectionId: section.id,
		from,
		to
	});

	const all = $derived(
		demo.suggestions
			.filter(
				(s) =>
					s.anchor.billId === bill.id &&
					s.anchor.sectionId === section.id &&
					(isLatest ? s.anchor.version <= version.n : s.anchor.version === version.n)
			)
			.map((s) => ({ s, stale: isStale(bill, s.anchor), count: demo.supportCount(s.id) }))
			.sort((a, b) => Number(a.stale) - Number(b.stale) || b.count - a.count)
	);
	const shown = $derived(
		filtered ? all.filter((x) => overlaps({ ...x.s.anchor, version: version.n }, range)) : all
	);
	const threads = $derived(
		demo.threadsFor(bill.id, version.n).filter((t) => t.anchor.sectionId === section.id)
	);

	// --- Composer ---
	let title = $state('');
	let replacement = $state('');
	let reason = $state('');
	let lastRange = '';

	$effect(() => {
		const key = `${from}-${to}`;
		if (key !== lastRange) {
			lastRange = key;
			replacement = anchorText(bill, range);
		}
	});

	function post(e: SubmitEvent) {
		e.preventDefault();
		if (!ui.requireResident()) return;
		if (!reason.trim()) {
			ui.notify('Add a short plain-language reason.');
			return;
		}
		const id = demo.addSuggestion(
			$state.snapshot(range),
			title.trim() || 'Suggested edit',
			replacement.trim(),
			reason.trim()
		);
		title = '';
		reason = '';
		replacement = anchorText(bill, range);
		filtered = true;
		ui.notify(`Posted. It enters the brief at ${BRIEF_MINIMUM} supporters.`);
		requestAnimationFrame(() => document.getElementById(id)?.scrollIntoView({ block: 'center' }));
	}
</script>

<svelte:head><title>Suggestions · {bill.number} {section.label}</title></svelte:head>

<div class="mx-auto flex max-w-[1200px] flex-col gap-5 px-4 py-6 sm:px-6">
	<a
		href={billHref(bill.id, version.n) + `#${section.id}`}
		class="inline-flex min-h-11 items-center gap-1.5 self-start text-sm font-semibold no-underline"
	>
		<Icon name="chevronLeft" />
		{bill.number} · {bill.title}
	</a>

	<div class="flex flex-col gap-1.5">
		<div class="text-[13px] text-muted">
			{section.label} · {filtered ? rangeLabel(from, to) : 'whole section'} · against v{version.n}
			{version.type}
		</div>
		<h1 class="m-0 font-display text-3xl font-semibold">
			{shown.length} suggested edit{shown.length === 1 ? '' : 's'} on {filtered
				? `${rangeLabel(from, to)}`
				: section.label}
		</h1>
		<div class="text-sm text-muted">
			Ranked by support. Suggestions can only be supported. To push back, write a better one or
			comment on the line.
			{#if filtered}
				<button
					type="button"
					class="ml-1 cursor-pointer font-semibold text-support underline"
					onclick={() => (filtered = false)}
				>
					Show the whole section
				</button>
			{/if}
		</div>
	</div>

	{#each shown as item, i (item.s.id)}
		<SuggestionCard suggestion={item.s} {bill} rank={i + 1} />
	{:else}
		<div class="card p-5 text-sm text-muted">
			No suggestions here yet. Write the first one below.
		</div>
	{/each}

	<form class="flex flex-col gap-3.5 card p-5" onsubmit={post}>
		<h2 class="m-0 font-display text-xl font-semibold">Write a suggested edit</h2>
		<div class="flex flex-wrap items-end gap-3">
			<label class="flex flex-col gap-1 text-sm font-semibold">
				From line
				<select
					class="min-h-11 field font-normal"
					bind:value={from}
					onchange={() => {
						if (to < from) to = from;
						filtered = true;
					}}
				>
					{#each section.lines as l (l.n)}<option value={l.n}>{l.n}</option>{/each}
				</select>
			</label>
			<label class="flex flex-col gap-1 text-sm font-semibold">
				To line
				<select
					class="min-h-11 field font-normal"
					bind:value={to}
					onchange={() => {
						if (from > to) from = to;
						filtered = true;
					}}
				>
					{#each section.lines as l (l.n)}<option value={l.n}>{l.n}</option>{/each}
				</select>
			</label>
			<label class="flex min-w-60 flex-1 flex-col gap-1 text-sm font-semibold">
				Short title
				<input
					type="text"
					bind:value={title}
					placeholder="e.g. Replace “shall” with “may”"
					class="min-h-11 field font-normal"
				/>
			</label>
		</div>
		<div class="grid gap-4 md:grid-cols-2">
			<label class="flex flex-col gap-1.5 text-sm font-semibold">
				Replacement text (leave empty to delete)
				<textarea
					rows="4"
					bind:value={replacement}
					class="field font-serif leading-relaxed font-normal"></textarea>
			</label>
			<label class="flex flex-col gap-1.5 text-sm font-semibold">
				Reason, in plain language
				<textarea
					rows="4"
					bind:value={reason}
					placeholder="Why should the legislature make this change?"
					class="field font-normal"></textarea>
			</label>
		</div>
		<div class="overflow-hidden rounded-xl border border-line">
			<SideBySide
				before={anchorText(bill, range)}
				after={replacement}
				afterLabel="Preview"
				compact
			/>
		</div>
		<div class="flex flex-wrap items-center gap-3">
			<span class="text-[13px] text-muted">
				Items need {BRIEF_MINIMUM} supporters to enter the brief. A steward may review it as well formed.
			</span>
			<button type="submit" class="ml-auto btn-primary">Post suggestion</button>
		</div>
	</form>

	{#if threads.length}
		<section class="flex flex-col gap-3">
			<h2 class="m-0 font-display text-xl font-semibold">Line comments on {section.label}</h2>
			{#each threads as t (t.id)}
				<div class="flex flex-col gap-1.5">
					<a class="text-sm font-semibold" href={linesHref(t.anchor)}>
						View {rangeLabel(t.anchor.from, t.anchor.to)} in the bill
					</a>
					<CommentThread thread={t} />
				</div>
			{/each}
		</section>
	{/if}
</div>
