<script lang="ts">
	import { placeholders, type PlaceholderId } from '#lib/placeholders.ts';

	// Inline "coming later" panel, used inside dialogs (share, sign-in) and by the global pop-up.
	let { id }: { id: PlaceholderId } = $props();
	const p = $derived(placeholders[id]);
</script>

<div
	role="status"
	class="flex flex-col gap-2 rounded-xl border border-dashed border-faint bg-[#f7f8fa] px-4 py-3.5"
>
	<div class="flex flex-wrap items-center gap-2">
		<span class="chip bg-ink text-white">Coming in {p.milestone}</span>
		<strong class="text-[15px]">{p.title}</strong>
	</div>
	<p class="m-0 text-sm leading-relaxed text-ink-2">{p.body}</p>
	<div class="text-xs text-muted">Design doc: <code>{p.doc}</code></div>
</div>
