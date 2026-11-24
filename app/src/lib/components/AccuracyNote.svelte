<script lang="ts">
	// Small-print disclosure on every piece of generated content: its tested accuracy and a link to the
	// bill text it rests on (design/01 § Rules and invariants; measured in test/accuracy/report.md).
	let {
		href
	}: {
		/** The bill text the content describes: its lines, or the bill text of the page. */
		href: string;
	} = $props();
</script>

<p class="text-xs text-muted" data-accuracy-note>
	Auto-generated; tested accuracy 75–85%. Verify with the <a {href}>bill text</a>.
</p>
