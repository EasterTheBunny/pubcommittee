<script lang="ts">
	import { onMount } from 'svelte';
	import FutureButton from '#lib/components/FutureButton.svelte';
	import { demo } from '#lib/state/demo.svelte.ts';
	import { ui } from '#lib/state/ui.svelte.ts';

	// In-app inbox and device notification settings (design/08 Notifications).

	let support = $state<'unknown' | 'unsupported' | 'default' | 'granted' | 'denied'>('unknown');
	let standalone = $state(false);

	onMount(() => {
		support = 'Notification' in window ? Notification.permission : 'unsupported';
		standalone = window.matchMedia('(display-mode: standalone)').matches;
	});

	const on = $derived(!!demo.deviceNotify[demo.personaId]);
	const signedIn = $derived(demo.persona.role !== 'reader');

	async function turnOn() {
		if (!('Notification' in window)) return;
		const result = await Notification.requestPermission();
		support = result;
		if (result === 'granted') {
			demo.setDeviceNotify(true);
			ui.notify('Device notifications on for this browser.');
		} else {
			ui.notify('Notifications were not allowed. You can still use the inbox.');
		}
	}

	function turnOff() {
		demo.setDeviceNotify(false);
		ui.notify('Device notifications off. In M2 this deletes the subscription.');
	}

	/** What job-notify would do after a count batch: one content-free message, or nothing. */
	async function simulateBatch() {
		if (demo.unread === 0) {
			ui.notify('Nothing new, so no notification is sent.');
			return;
		}
		const reg = await navigator.serviceWorker?.ready;
		if (!reg?.active) {
			ui.notify('The service worker is not running yet. Reload and try again.');
			return;
		}
		reg.active.postMessage({ kind: 'demo-updates' });
	}
</script>

<svelte:head><title>Inbox · PubCommittee</title></svelte:head>

<div class="mx-auto flex max-w-[900px] flex-col gap-5 px-4 py-8 sm:px-6">
	<div class="flex flex-wrap items-end gap-3">
		<h1 class="m-0 font-display text-3xl font-semibold">Inbox</h1>
		{#if demo.inbox.length}
			<button type="button" class="ml-auto btn-ghost px-3" onclick={() => demo.markAllRead()}>
				Mark all read
			</button>
		{/if}
	</div>

	{#if !signedIn}
		<div class="card p-5 text-sm">
			<p class="m-0">Sign in to see updates on things you took part in.</p>
			<button type="button" class="mt-3 btn-primary" onclick={() => (ui.signIn = true)}>
				Sign in
			</button>
		</div>
	{:else}
		<section aria-label="Updates" class="flex flex-col gap-2.5">
			{#each demo.inbox as e (e.id)}
				{@const read = demo.isRead(e.id)}
				<a
					href={e.href}
					class="flex flex-col gap-1 card px-4 py-3 text-ink no-underline hover:border-support-line hover:text-ink {read
						? ''
						: 'border-support-line'}"
					data-content-class="platform"
				>
					<span class="text-xs text-muted">
						{e.date}{read ? '' : ' · new'}
					</span>
					<span class="text-sm">{e.text}</span>
				</a>
			{:else}
				<div class="card p-5 text-sm text-muted">
					No updates. Support a suggestion, signal on a bill, or request markup, and changes to
					those appear here.
				</div>
			{/each}
		</section>
		<p class="m-0 text-xs text-muted">
			Built from public updates and your own private items when you open this page. No list of who
			supported what is kept.
		</p>

		<section aria-label="Device notifications" class="flex flex-col gap-3 card p-5">
			<h2 class="m-0 font-display text-xl font-semibold">Device notifications</h2>
			<p class="m-0 text-sm leading-relaxed text-ink-2">
				Off by default and set per device. A notification only says “You have updates”, never which
				bill or what you supported, so a lock screen shows nothing about your positions. No email or
				phone number is needed.
			</p>

			{#if support === 'unsupported'}
				<p class="m-0 text-sm text-oppose-ink">
					This browser does not support notifications here. On iPhone and iPad, install the app
					first (Share, then Add to Home Screen) and open it from the home screen.
				</p>
			{:else if support === 'denied'}
				<p class="m-0 text-sm text-oppose-ink">
					Notifications are blocked in this browser's site settings. The inbox still works.
				</p>
			{:else}
				<div class="flex flex-wrap items-center gap-2.5">
					{#if on && support === 'granted'}
						<span class="chip bg-ok-soft text-ok">On for this browser</span>
						<button type="button" class="btn-secondary" onclick={turnOff}>Turn off</button>
						<button type="button" class="btn-outline" onclick={simulateBatch}>
							Simulate a notification batch
						</button>
					{:else}
						<button type="button" class="btn-primary" onclick={turnOn}>
							Turn on for this browser
						</button>
					{/if}
					<FutureButton id="push-delivery">How delivery works</FutureButton>
				</div>
			{/if}

			{#if !standalone}
				<p class="m-0 text-xs text-muted">
					Tip: install PubCommittee from your browser's menu to open it like an app and read bills
					you have opened while offline. Some phones only deliver notifications to installed apps.
				</p>
			{/if}
		</section>
	{/if}
</div>
