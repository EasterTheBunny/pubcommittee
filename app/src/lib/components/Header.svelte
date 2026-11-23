<script lang="ts">
	import { page } from '$app/state';
	import { catalogHref, homeHref, logHref, policyHref } from '#lib/links.ts';
	import Icon from './Icon.svelte';
	import PlaceholderLink from './PlaceholderLink.svelte';

	const path = $derived(page.url.pathname);
	const nav = $derived([
		{
			href: homeHref(),
			label: 'Bills in review',
			active: path === homeHref() || path.startsWith('/bills/')
		},
		{ href: catalogHref(), label: 'Catalog', active: path.startsWith('/catalog') },
		{ href: policyHref(), label: 'Selection policy', active: path.startsWith('/policy') },
		{ href: logHref(), label: 'Public log', active: path.startsWith('/log') }
	]);
</script>

<header class="border-b border-line bg-card">
	<div class="mx-auto flex max-w-[1200px] flex-wrap items-center gap-x-7 gap-y-3 px-4 py-3 sm:px-6">
		<a href={homeHref()} class="flex items-center gap-2.5 text-ink no-underline hover:text-ink">
			<Icon name="doc" size={26} />
			<span class="font-display text-lg font-bold">PubCommittee</span>
		</a>
		<nav aria-label="Primary" class="flex flex-wrap gap-x-5 gap-y-1 text-sm font-medium">
			{#each nav as item (item.label)}
				<a
					href={item.href}
					aria-current={item.active ? 'page' : undefined}
					class="no-underline {item.active
						? 'border-b-2 border-ink pb-0.5 text-ink hover:text-ink'
						: 'text-muted hover:text-ink'}"
				>
					{item.label}
				</a>
			{/each}
		</nav>
		<div class="ml-auto text-sm">
			<PlaceholderLink id="signin-google">Sign in</PlaceholderLink>
		</div>
	</div>
</header>
