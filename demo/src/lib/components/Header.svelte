<script lang="ts">
	import { page } from '$app/state';
	import { briefsHref, catalogHref, homeHref, inboxHref, issuesHref } from '#lib/links.ts';
	import { demo } from '#lib/state/demo.svelte.ts';
	import { ui } from '#lib/state/ui.svelte.ts';
	import Icon from './Icon.svelte';

	const nav = $derived([
		{
			href: homeHref(),
			label: 'Bills',
			active: page.url.pathname === homeHref() || page.url.pathname.includes('/bills/')
		},
		{ href: catalogHref(), label: 'Catalog', active: page.url.pathname.includes('/catalog') },
		{ href: issuesHref(), label: 'Issues', active: page.url.pathname.includes('/issues') },
		{ href: briefsHref(), label: 'Briefs', active: page.url.pathname.includes('/briefs') }
	]);

	const personaLabel = $derived(
		demo.persona.role === 'reader'
			? 'Sign in'
			: `${demo.persona.role[0].toUpperCase()}${demo.persona.role.slice(1)} · ${demo.persona.name}`
	);
</script>

<header class="border-b border-line bg-card">
	<div class="mx-auto flex max-w-[1200px] flex-wrap items-center gap-x-7 gap-y-3 px-4 py-3 sm:px-6">
		<a href={homeHref()} class="flex items-center gap-2.5 text-ink no-underline hover:text-ink">
			<Icon name="doc" size={26} />
			<span class="font-display text-lg font-bold">PubCommittee</span>
			<span class="chip bg-oppose-soft text-oppose-ink">POC · mock data</span>
		</a>
		<nav aria-label="Primary" class="flex gap-5 text-sm font-medium">
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
		<div class="ml-auto flex items-center gap-2 text-sm">
			{#if ui.installPrompt}
				<button
					type="button"
					class="btn-ghost px-3"
					onclick={async () => {
						await ui.installPrompt?.prompt();
						ui.installPrompt = null;
					}}
				>
					Install app
				</button>
			{/if}
			{#if demo.persona.role !== 'reader'}
				<a
					href={inboxHref()}
					class="relative btn-ghost px-3 no-underline"
					aria-label="Inbox, {demo.unread} new"
				>
					Inbox
					{#if demo.unread}
						<span
							class="ml-1 inline-flex min-w-5 items-center justify-center rounded-full bg-support px-1.5 text-xs font-semibold text-white"
						>
							{demo.unread}
						</span>
					{/if}
				</a>
			{/if}
			<span class="hidden text-muted sm:inline">
				{demo.persona.role === 'reader' ? 'Not signed in' : 'Viewing as'}
			</span>
			<button type="button" class="btn-secondary" onclick={() => (ui.signIn = true)}>
				<Icon name="user" />
				{personaLabel}
				<Icon name="chevronDown" size={14} />
			</button>
			<button
				type="button"
				class="btn-ghost px-3 text-muted"
				title="Reset demo data"
				aria-label="Reset demo data"
				onclick={() => {
					demo.reset();
					ui.notify('Demo reset to seed data.');
				}}
			>
				<Icon name="reset" />
			</button>
		</div>
	</div>
</header>
