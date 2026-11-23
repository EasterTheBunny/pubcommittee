<script lang="ts">
	import { latestLinesHref, billHref, latestSectionHref } from '#lib/links.ts';
	import type { ChangedSince } from '#lib/server/bill.ts';

	// "Changed since" notice on old-version addresses (design/06 Link behavior). It states only what
	// the comparison says; with no comparison published it says so rather than guessing.
	let {
		notice,
		billId,
		sectionId,
		range
	}: {
		notice: ChangedSince;
		billId: string;
		sectionId: string;
		range: { from: number; to: number } | null;
	} = $props();

	const currentHref = $derived.by(() => {
		if (notice.kind === 'current') return '';
		if (notice.kind === 'changed' && notice.now)
			return latestLinesHref(billId, notice.now.sectionId, notice.now.from, notice.now.to);
		if (range && notice.kind === 'unchanged')
			return latestLinesHref(billId, sectionId, range.from, range.to);
		return latestSectionHref(billId, sectionId);
	});
</script>

{#if notice.kind !== 'current'}
	<div
		role="status"
		data-notice={notice.kind}
		class="rounded-xl border px-4 py-3 text-[15px] {notice.kind === 'changed'
			? 'border-oppose-line bg-oppose-soft text-oppose-ink'
			: 'border-line-strong bg-subtle text-ink-2'}"
	>
		{#if notice.kind === 'changed'}
			<strong>These words changed in a newer version.</strong> Version {notice.currentVersion} is the
			current text.
		{:else if notice.kind === 'unchanged'}
			This is an older version. These words are unchanged in version {notice.currentVersion}, the
			current text.
		{:else}
			This is an older version. Version {notice.currentVersion} is the current text; whether these words
			changed is not published yet.
		{/if}
		<a class="mt-2 ml-2 btn-secondary" href={currentHref}>See the current version</a>
		<a class="ml-2 text-sm" href={billHref(billId)}>Whole bill</a>
	</div>
{/if}
