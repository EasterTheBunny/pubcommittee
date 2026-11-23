<script lang="ts">
	import { anchorText, rangeLabel } from '#lib/anchors.ts';
	import { billById, latestVersion, sectionOf, versionOf } from '#lib/mock/index.ts';
	import { billHref, linesHref, sectionHref, sectionSuggestionsHref } from '#lib/links.ts';
	import type { PlaceholderId } from '#lib/placeholders.ts';
	import { count, demo, pct } from '#lib/state/demo.svelte.ts';
	import { ui } from '#lib/state/ui.svelte.ts';
	import type { ShareTarget } from '#lib/types.ts';
	import Modal from './Modal.svelte';
	import PlaceholderNote from './PlaceholderNote.svelte';

	let open = $state(false);
	let target = $state<ShareTarget | null>(null);
	let picked = $state<PlaceholderId | null>(null);
	let copied = $state(false);

	$effect(() => {
		if (ui.share) {
			target = ui.share;
			picked = null;
			copied = false;
			open = true;
		}
	});
	$effect(() => {
		if (!open) ui.share = null;
	});

	interface Card {
		heading: string;
		pinned: string;
		quote?: string;
		before?: string;
		after?: string;
		citation: string;
		/** Summary field: official or platform text only, never user content (design/10). */
		title: string;
		/** User content, kept in its own labelled field (design/10). */
		userQuote?: { label: string; text: string };
		meta?: string;
		href: string;
	}

	const card = $derived.by((): Card | null => {
		const t = target;
		if (!t) return null;
		if (t.kind === 'bill' || t.kind === 'section') {
			const bill = billById(t.billId)!;
			const version = versionOf(bill, t.version) ?? latestVersion(bill);
			const totals = demo.billTotals(bill.id);
			if (t.kind === 'bill') {
				return {
					heading: 'Share this bill',
					pinned: `Pinned to v${version.n}`,
					quote: bill.summary,
					citation: `${bill.number} · ${bill.sponsor} · ${bill.status}`,
					title: bill.title,
					meta: `${count(totals.support)} support · ${count(totals.oppose)} oppose`,
					href: billHref(bill.id, version.n)
				};
			}
			const section = sectionOf(version, t.sectionId)!;
			const riders = totals.riderTags[section.id] ?? 0;
			const riderShare = pct(riders, totals.oppose);
			return {
				heading: `Share ${section.label}`,
				pinned: `Pinned to v${version.n}`,
				quote: `${section.label}. ${section.heading} ${section.lines
					.slice(0, 2)
					.map((l) => l.text)
					.join(' ')}…`,
				citation: `${bill.number}, ${section.label}`,
				title: bill.title,
				meta:
					riderShare >= 10
						? `Tagged as an unrelated rider by ${riderShare}% of opposers`
						: undefined,
				href: sectionHref(bill.id, version.n, section.id)
			};
		}
		if (t.kind === 'lines') {
			const bill = billById(t.anchor.billId)!;
			const section = sectionOf(versionOf(bill, t.anchor.version)!, t.anchor.sectionId)!;
			return {
				heading: 'Share these lines',
				pinned: `${rangeLabel(t.anchor.from, t.anchor.to)} · pinned to v${t.anchor.version}`,
				quote: `“${anchorText(bill, t.anchor).replace(/^[“"]|[”"]$/g, '')}”`,
				citation: `${bill.number}, ${section.label}`,
				title: bill.title,
				href: linesHref(t.anchor)
			};
		}
		if (t.kind === 'suggestion') {
			const s = demo.suggestion(t.suggestionId)!;
			const bill = billById(s.anchor.billId)!;
			const section = sectionOf(versionOf(bill, s.anchor.version)!, s.anchor.sectionId)!;
			return {
				heading: 'Share this suggested edit',
				pinned: `Against v${s.anchor.version}`,
				before: anchorText(bill, s.anchor),
				after: s.replacement,
				citation: `${bill.number}, ${section.label}`,
				title: bill.title,
				userQuote: { label: s.title, text: s.reason },
				meta: `${count(demo.supportCount(s.id))} supporters`,
				href: sectionSuggestionsHref(
					bill.id,
					s.anchor.version,
					s.anchor.sectionId,
					s.anchor.from,
					s.anchor.to
				)
			};
		}
		const thread = demo.threads.find((x) => x.id === t.threadId)!;
		const bill = billById(thread.anchor.billId)!;
		const section = sectionOf(versionOf(bill, thread.anchor.version)!, thread.anchor.sectionId)!;
		return {
			heading: 'Share this comment thread',
			pinned: `Pinned to v${thread.anchor.version}`,
			quote: `“${anchorText(bill, thread.anchor).replace(/^[“"]|[”"]$/g, '')}”`,
			citation: `${bill.number}, ${section.label}`,
			title: bill.title,
			userQuote: { label: thread.comments[0].author, text: thread.comments[0].body },
			href: linesHref(thread.anchor)
		};
	});

	const absolute = $derived(
		card
			? typeof location === 'undefined'
				? card.href
				: new URL(card.href, location.origin).href
			: ''
	);

	async function copy() {
		try {
			await navigator.clipboard.writeText(absolute);
			copied = true;
		} catch {
			ui.notify('Copy failed. Select the link and copy it manually.');
		}
	}

	const targets: { id: PlaceholderId; short: string; label: string }[] = [
		{ id: 'share-social', short: 'S', label: 'Social post' },
		{ id: 'share-message', short: 'M', label: 'Message' },
		{ id: 'share-image', short: 'I', label: 'Quote image' },
		{ id: 'share-embed', short: 'E', label: 'Embed' }
	];
</script>

<Modal bind:open title={card?.heading ?? 'Share'} eyebrow={card?.pinned} width="max-w-2xl">
	{#if card}
		<div class="overflow-hidden rounded-xl border border-line">
			{#if card.before !== undefined}
				<div class="grid sm:grid-cols-2">
					<div
						class="border-line bg-subtle px-4 py-3 font-serif text-[15px] leading-relaxed sm:border-r"
						data-content-class="official"
					>
						<div class="mb-1 font-sans eyebrow">Bill text</div>
						{card.before}
					</div>
					<div
						class="bg-[#fafcff] px-4 py-3 font-serif text-[15px] leading-relaxed"
						data-content-class="user"
					>
						<div class="mb-1 font-sans eyebrow">Suggested</div>
						{card.after}
					</div>
				</div>
			{:else}
				<div
					class="bg-subtle px-5 py-4 font-serif text-lg leading-relaxed"
					data-content-class={target?.kind === 'bill' ? 'platform' : 'official'}
				>
					{card.quote}
				</div>
			{/if}
			{#if card.userQuote}
				<div
					class="border-t border-line px-5 py-3 text-sm leading-normal"
					data-content-class="user"
				>
					<strong>{card.userQuote.label}:</strong>
					{card.userQuote.text}
				</div>
			{/if}
			<div
				class="flex flex-wrap gap-x-3 gap-y-1 border-t border-line px-5 py-3 text-[13px] text-muted"
				data-content-class="platform"
			>
				<strong class="text-ink">{card.citation}</strong>
				<span>{card.title}</span>
				{#if card.meta}<span class="font-semibold text-oppose-ink">{card.meta}</span>{/if}
			</div>
		</div>
		<p class="-mt-2 text-[13px] text-muted">
			Preview card. It never shows your own support or opposition.
		</p>

		<div class="grid grid-cols-2 gap-2.5 sm:grid-cols-4">
			{#each targets as t (t.id)}
				<button
					type="button"
					onclick={() => (picked = t.id)}
					aria-pressed={picked === t.id}
					class="flex min-h-18 cursor-pointer flex-col items-center justify-center gap-1.5 rounded-xl border text-[13px] font-semibold {picked ===
					t.id
						? 'border-support bg-support-soft'
						: 'border-line bg-card hover:bg-subtle'}"
				>
					<span
						class="flex size-7 items-center justify-center rounded-full bg-line-soft text-xs text-ink-2"
					>
						{t.short}
					</span>
					{t.label}
				</button>
			{/each}
		</div>

		{#if picked}<PlaceholderNote id={picked} />{/if}

		<div class="flex items-end gap-2.5">
			<label class="flex flex-1 flex-col gap-1 text-[13px] font-semibold">
				Link (works in M0)
				<input
					type="text"
					readonly
					value={absolute}
					class="min-h-11 field font-normal text-ink-2"
				/>
			</label>
			<button type="button" class="btn-primary" onclick={copy}>
				{copied ? 'Copied' : 'Copy link'}
			</button>
		</div>
	{/if}
</Modal>
