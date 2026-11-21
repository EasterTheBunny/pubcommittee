<script lang="ts">
	import { onMount, tick, untrack } from 'svelte';
	import { goto, replaceState } from '$app/navigation';
	import {
		anchorHash,
		anchorText,
		isStale,
		parseHash,
		rangeLabel,
		type HashSelection
	} from '#lib/anchors.ts';
	import { billHref, catalogHref, issueHref, sectionSuggestionsHref } from '#lib/links.ts';
	import { billById, catalogForBill, issueById, latestVersion } from '#lib/mock/index.ts';
	import { demo, pct } from '#lib/state/demo.svelte.ts';
	import { ui } from '#lib/state/ui.svelte.ts';
	import type { Anchor, Bill, BillVersion } from '#lib/types.ts';
	import CommentThread from './CommentThread.svelte';
	import ExplanationPanel from './ExplanationPanel.svelte';
	import FutureButton from './FutureButton.svelte';
	import Icon from './Icon.svelte';
	import SignalCard from './SignalCard.svelte';

	let {
		bill,
		version,
		initial,
		basePath
	}: {
		bill: Bill;
		version: BillVersion;
		/** Selection from a canonical path address (design/10), rendered at build time. */
		initial?: HashSelection;
		/** Page to keep in the URL when the selection changes; defaults to the current path. */
		basePath?: string;
	} = $props();

	const latest = $derived(latestVersion(bill));
	const isLatest = $derived(version.n === latest.n);
	const issue = $derived(bill.issueId ? issueById(bill.issueId) : undefined);
	const competitors = $derived(
		issue ? issue.billIds.filter((id) => id !== bill.id).map((id) => billById(id)!) : []
	);
	const totals = $derived(demo.billTotals(bill.id));
	const selection = $derived(catalogForBill(bill.id)?.tracked);
	const threads = $derived(demo.threadsFor(bill.id, version.n));

	// --- Line selection, mirrored in the URL hash so any selection is shareable ---

	interface Selection {
		sectionId: string;
		from: number;
		to: number;
	}
	let sel = $state<Selection | null>(
		untrack(() =>
			initial?.from !== undefined
				? { sectionId: initial.sectionId, from: initial.from, to: initial.to ?? initial.from }
				: null
		)
	);
	let composing = $state(false);
	let draft = $state('');

	const selAnchor = $derived<Anchor | null>(
		sel ? { billId: bill.id, version: version.n, ...sel } : null
	);
	/** A shared link opened on an older version whose words have since changed (design/06). */
	const sharedChanged = $derived(selAnchor && !isLatest ? isStale(bill, selAnchor) : false);

	function readHash() {
		const h = parseHash(location.hash);
		if (!h) return;
		sel =
			h.from !== undefined ? { sectionId: h.sectionId, from: h.from, to: h.to ?? h.from } : null;
		const target = document.getElementById(h.from !== undefined ? `l${h.from}` : h.sectionId);
		target?.scrollIntoView({ block: 'center' });
	}

	onMount(() => {
		if (location.hash) readHash();
		else if (initial) {
			const id = initial.from !== undefined ? `l${initial.from}` : initial.sectionId;
			document.getElementById(id)?.scrollIntoView({ block: 'center' });
		}
		window.addEventListener('hashchange', readHash);
		return () => window.removeEventListener('hashchange', readHash);
	});

	function select(sectionId: string, n: number, extend: boolean) {
		composing = false;
		if (extend && sel && sel.sectionId === sectionId) {
			sel = { sectionId, from: Math.min(sel.from, n), to: Math.max(sel.to, n) };
		} else if (sel && sel.sectionId === sectionId && sel.from === n && sel.to === n) {
			sel = null;
		} else {
			sel = { sectionId, from: n, to: n };
		}
		const hash = sel ? anchorHash(sel.sectionId, sel.from, sel.to) : '';
		replaceState(new URL((basePath ?? location.pathname) + hash, location.href), {});
	}

	const inSel = (sectionId: string, n: number) =>
		!!sel && sel.sectionId === sectionId && n >= sel.from && n <= sel.to;

	function startComment() {
		if (!ui.requireResident()) return;
		composing = true;
		tick().then(() => document.getElementById('new-comment')?.focus());
	}

	function postComment() {
		if (!selAnchor || !draft.trim()) return;
		demo.addThread($state.snapshot(selAnchor), draft.trim());
		draft = '';
		composing = false;
	}

	// --- Per-section summaries ---

	const sectionSuggestions = (sectionId: string) =>
		demo.suggestions.filter(
			(s) =>
				s.anchor.billId === bill.id &&
				s.anchor.sectionId === sectionId &&
				(isLatest ? s.anchor.version <= version.n : s.anchor.version === version.n)
		);

	const sectionComments = (sectionId: string) =>
		threads
			.filter((t) => t.anchor.sectionId === sectionId)
			.reduce((n, t) => n + t.comments.length, 0);

	const staleCount = $derived(
		demo.suggestions.filter((s) => s.anchor.billId === bill.id && isStale(bill, s.anchor)).length
	);
	const firstStale = $derived(
		demo.suggestions.find((s) => s.anchor.billId === bill.id && isStale(bill, s.anchor))
	);

	const focusedSection = $derived(
		version.sections.find((s) => s.id === sel?.sectionId) ??
			version.sections.find((s) => version.explanations.some((x) => x.sectionId === s.id)) ??
			version.sections[0]
	);
	const focusedExplanation = $derived(
		version.explanations.find((x) => x.sectionId === focusedSection.id)
	);
</script>

<div class="mx-auto flex max-w-[1200px] flex-col gap-5 px-4 py-6 sm:px-6">
	{#if issue && competitors.length}
		<a
			href={issueHref(issue.id)}
			class="flex flex-wrap items-center gap-x-3.5 gap-y-2 rounded-xl border border-support-line bg-support-soft px-4 py-3 text-sm text-ink no-underline hover:text-ink"
		>
			<span class="text-support"><Icon name="list" size={18} /></span>
			<span>
				<strong>{competitors.length} competing bills</strong> address {issue.title.toLowerCase()} differently:
				{competitors.map((b) => b.number).join(' and ')}.
			</span>
			<span class="ml-auto font-semibold text-support">Compare on the issue page</span>
		</a>
	{/if}

	{#if !isLatest}
		<div
			role="status"
			class="rounded-xl border border-oppose-line bg-oppose-tint px-4 py-3 text-sm"
		>
			{#if sharedChanged && sel}
				<strong>These words changed in v{latest.n}.</strong> You are viewing the shared text from v{version.n}
				({version.type}).
			{:else}
				You are viewing v{version.n} ({version.type}). The current version is v{latest.n}.
			{/if}
			<a
				class="ml-1 font-semibold"
				href={billHref(bill.id, latest.n) +
					(sel ? anchorHash(sel.sectionId, sel.from, sel.to) : '')}
			>
				View current version
			</a>
		</div>
	{/if}

	<section aria-label="Bill header" class="flex flex-wrap items-start gap-x-8 gap-y-5 card p-6">
		<div class="flex min-w-0 flex-[999_1_520px] flex-col gap-2.5">
			<div class="flex flex-wrap gap-2 text-[13px] text-muted">
				<span class="font-semibold text-ink">{bill.number}</span><span>·</span><span>
					{bill.chamber}
				</span><span>·</span><span>Sponsor: {bill.sponsor}</span><span>·</span><span>
					Status: {bill.status}
				</span>
			</div>
			<h1
				class="m-0 font-display text-[32px] leading-tight font-semibold"
				data-content-class="official"
			>
				{bill.title}
			</h1>
			<p class="m-0 text-base leading-normal text-ink-2" data-content-class="platform">
				{bill.summary}
			</p>
			{#if selection}
				<p class="m-0 text-sm text-muted" data-content-class="platform">
					In review because: {selection.reasons.join(' · ')} · selection policy v{selection.policy} ·
					<a href={catalogHref() + '#' + bill.id}>catalog entry</a>
				</p>
			{/if}
			{#if version.sourceBill}
				<p class="m-0 text-sm text-muted">
					This version is the text of <strong>{version.sourceBill}</strong>, the other chamber’s
					companion, joined into one review by a steward.
				</p>
			{/if}
			<div class="mt-1.5 flex flex-wrap gap-2.5">
				<label class="flex max-w-full min-w-0 flex-wrap items-center gap-2 text-sm text-muted">
					Version
					<select
						class="min-h-11 max-w-full min-w-0 field text-sm text-ink"
						value={version.n}
						onchange={(e) => goto(billHref(bill.id, Number(e.currentTarget.value)))}
					>
						{#each [...bill.versions].reverse() as v (v.n)}
							<option value={v.n}>v{v.n} · {v.type} · {v.date}</option>
						{/each}
					</select>
				</label>
				<button
					type="button"
					class="btn-secondary"
					onclick={() => (ui.share = { kind: 'bill', billId: bill.id, version: version.n })}
				>
					<Icon name="share" /> Share bill
				</button>
				<FutureButton id="ingestion">Source feed</FutureButton>
				<FutureButton id="official-links">Official page</FutureButton>
				<FutureButton id="machine-access">Data for machines</FutureButton>
			</div>
		</div>
		<div class="min-w-0 flex-[1_1_340px]">
			<SignalCard billId={bill.id} />
		</div>
	</section>

	<div class="flex flex-wrap items-start gap-5">
		<section aria-label="Bill text" class="min-w-0 flex-[999_1_600px] overflow-hidden card">
			<div
				class="flex flex-wrap items-center gap-2.5 border-b border-line px-5 py-3.5 text-[13px] text-muted"
			>
				<span class="font-semibold text-ink">Bill text</span><span>·</span><span>
					v{version.n}
					{version.type}
				</span>
				<span class="ml-auto">Click a line number to select. Shift-click to extend.</span>
			</div>

			{#each version.sections as section (section.id)}
				{@const riderShare = pct(totals.riderTags[section.id] ?? 0, totals.oppose)}
				{@const riderNote = bill.riderNotes?.[section.id]}
				{@const nSugg = sectionSuggestions(section.id).length}
				{@const nComm = sectionComments(section.id)}
				<div
					id={section.id}
					class="scroll-mt-6 border-b border-dashed border-line-strong pb-3 last:border-b-0"
					data-anchor={section.id}
				>
					<div class="flex flex-wrap items-center gap-x-3 gap-y-1 px-5 pt-5 pb-2">
						<h2 class="m-0 font-serif text-lg font-semibold" data-content-class="official">
							{section.label.toUpperCase()}. {section.heading}
						</h2>
						<button
							type="button"
							class="inline-flex size-11 cursor-pointer items-center justify-center rounded-lg text-muted hover:bg-subtle"
							aria-label="Share {section.label}"
							onclick={() =>
								(ui.share = {
									kind: 'section',
									billId: bill.id,
									version: version.n,
									sectionId: section.id
								})}
						>
							<Icon name="link" />
						</button>
						{#if riderShare >= 10}
							<span class="chip border border-oppose-line bg-oppose-soft text-oppose-ink">
								Tagged as unrelated rider by {riderShare}% of opposers
							</span>
						{/if}
						{#if nSugg || nComm}
							<a
								class="ml-auto text-sm font-semibold"
								href={sectionSuggestionsHref(bill.id, version.n, section.id)}
							>
								{nSugg} suggestion{nSugg === 1 ? '' : 's'} · {nComm} comment{nComm === 1 ? '' : 's'}
							</a>
						{/if}
					</div>

					{#if riderNote && riderShare >= 10}
						<div
							class="mx-5 mb-3 rounded-xl border border-line bg-subtle px-3.5 py-3 text-sm leading-normal"
							data-content-class="platform"
						>
							<div class="mb-1 text-xs font-semibold text-muted">
								Steward note{riderNote.twoPartySignOff
									? ' · signed off by stewards from 2 parties'
									: ''}
							</div>
							{riderNote.text}
						</div>
					{/if}

					<div class="font-serif text-base leading-[1.7]" data-content-class="official">
						{#each section.lines as line (line.n)}
							{@const selected = inSel(section.id, line.n)}
							{@const lineThreads = threads.filter(
								(t) => t.anchor.sectionId === section.id && t.anchor.to === line.n
							)}
							<div
								id="l{line.n}"
								class="flex scroll-mt-24 gap-4 px-5 py-0.5 {selected
									? 'bg-hl shadow-[inset_3px_0_0_var(--color-hl-strong)]'
									: 'hover:bg-subtle'}"
							>
								<button
									type="button"
									class="w-8 flex-none cursor-pointer pt-1 text-right font-sans text-xs {selected
										? 'font-semibold text-hl-ink'
										: 'text-faint hover:text-support'}"
									aria-label="Select line {line.n}"
									aria-pressed={selected}
									onclick={(e) => select(section.id, line.n, e.shiftKey)}
								>
									{line.n}
								</button>
								<span
									style:padding-left="{(line.indent ?? 0) * 24}px"
									data-anchor="{section.id}-l{line.n}"
								>
									{line.text}
								</span>
							</div>

							{#if sel && sel.sectionId === section.id && sel.to === line.n && selAnchor}
								<div
									class="my-2 mr-5 ml-16 flex flex-col gap-3 font-sans"
									data-content-class="user"
								>
									<div class="flex flex-wrap gap-2">
										<button type="button" class="btn-secondary" onclick={startComment}>
											<Icon name="comment" /> Comment
										</button>
										<a
											class="btn-primary"
											href={sectionSuggestionsHref(
												bill.id,
												version.n,
												section.id,
												sel.from,
												sel.to
											)}
										>
											<Icon name="pencil" /> Suggest edit
										</a>
										<button
											type="button"
											class="btn-secondary"
											onclick={() =>
												(ui.share = { kind: 'lines', anchor: $state.snapshot(selAnchor) })}
										>
											<Icon name="share" /> Share {rangeLabel(sel.from, sel.to)}
										</button>
										<button
											type="button"
											class="btn-ghost text-ink-2"
											onclick={() => select(section.id, sel!.from, false)}
										>
											Clear
										</button>
									</div>
									{#if composing}
										<form
											class="flex flex-col gap-2 rounded-xl border border-line p-3.5"
											onsubmit={(e) => {
												e.preventDefault();
												postComment();
											}}
										>
											<label for="new-comment" class="text-sm font-semibold">
												Comment on {rangeLabel(sel.from, sel.to)}
											</label>
											<blockquote
												class="m-0 border-l-2 border-hl-strong pl-3 font-serif text-sm text-ink-2"
												data-content-class="official"
											>
												{anchorText(bill, selAnchor)}
											</blockquote>
											<textarea
												id="new-comment"
												rows="3"
												bind:value={draft}
												placeholder="Questions, concerns, context, local impact"
												class="field"></textarea>
											<div class="flex gap-2">
												<button type="submit" class="btn-primary">Post comment</button>
												<button
													type="button"
													class="btn-ghost text-ink-2"
													onclick={() => (composing = false)}
												>
													Cancel
												</button>
											</div>
										</form>
									{/if}
								</div>
							{/if}

							{#each lineThreads as t (t.id)}
								<div class="my-2 mr-5 ml-16" data-content-class="user">
									<CommentThread thread={t} />
								</div>
							{/each}
						{/each}
					</div>
				</div>
			{/each}
		</section>

		<aside
			aria-label="What this changes in current law"
			class="flex min-w-0 flex-[1_1_360px] flex-col gap-4 lg:sticky lg:top-4"
		>
			<ExplanationPanel explanation={focusedExplanation} section={focusedSection} />
			<div class="flex flex-wrap gap-2 text-sm">
				<span class="text-muted">Explanations:</span>
				{#each version.explanations as x (x.sectionId)}
					{@const s = version.sections.find((s) => s.id === x.sectionId)}
					{#if s}<a href="#{s.id}" class="font-semibold">{s.label}</a>{/if}
				{/each}
			</div>

			{#if bill.versions.length > 1}
				<div class="flex flex-col gap-2.5 card p-5">
					<div class="eyebrow">Version change</div>
					{#if isLatest && staleCount && firstStale}
						<div class="text-sm leading-normal">
							v{latest.n} changed lines that earlier suggestions targeted.
							<strong>{staleCount} suggestion{staleCount === 1 ? ' is' : 's are'} stale</strong> and waiting
							for the author to re-anchor.
						</div>
						<a
							class="text-sm font-semibold"
							href={sectionSuggestionsHref(bill.id, latest.n, firstStale.anchor.sectionId)}
						>
							See stale suggestions
						</a>
					{:else}
						<div class="text-sm leading-normal">
							{bill.versions.length} official versions. Older versions stay readable, and every comment
							keeps the version it was written against.
						</div>
					{/if}
					<FutureButton id="notifications" class="self-start">
						Notify me of new versions
					</FutureButton>
				</div>
			{/if}
		</aside>
	</div>
</div>
