<script lang="ts" module>
	const paths = {
		doc: 'M4 4h12l4 4v12H4z M8 10h8 M8 14h8 M8 18h5',
		share: 'M4 12v7a1 1 0 0 0 1 1h14a1 1 0 0 0 1-1v-7 M12 3v12 M7 8l5-5 5 5',
		link: 'M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1 M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1',
		comment: 'M4 5h16v11H9l-5 4z',
		pencil: 'M4 20h4L19 9l-4-4L4 16z',
		close: 'M6 6l12 12 M18 6L6 18',
		chevronDown: 'M6 9l6 6 6-6',
		chevronLeft: 'M15 18l-6-6 6-6',
		list: 'M7 7h10 M7 12h10 M7 17h6 M3 3h18v18H3z',
		flag: 'M5 21V4 M5 4h11l-2 4 2 4H5',
		check: 'M5 12l5 5L20 7',
		arrowRight: 'M4 12h15 M14 6l6 6-6 6',
		user: 'M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8z M4 20a8 8 0 0 1 16 0',
		reset: 'M4 4v6h6 M20 12a8 8 0 1 1-2.3-5.7L4 10'
	};
	export type IconName = keyof typeof paths;
</script>

<script lang="ts">
	let { name, size = 16, label }: { name: IconName; size?: number; label?: string } = $props();
</script>

<svg
	width={size}
	height={size}
	viewBox="0 0 24 24"
	fill="none"
	stroke="currentColor"
	stroke-width="2"
	stroke-linecap="round"
	stroke-linejoin="round"
	class="shrink-0"
	role={label ? 'img' : undefined}
	aria-label={label}
	aria-hidden={label ? undefined : 'true'}
>
	<path d={paths[name]} />
</svg>
