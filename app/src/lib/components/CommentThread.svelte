<script lang="ts">
	import { rangeLabel } from '#lib/anchors.ts';
	import { demo } from '#lib/state/demo.svelte.ts';
	import { ui } from '#lib/state/ui.svelte.ts';
	import type { Thread } from '#lib/types.ts';
	import Icon from './Icon.svelte';

	// Line comments: discussion only, no counts (design/03).
	let { thread }: { thread: Thread } = $props();

	let replying = $state(false);
	let body = $state('');

	function startReply() {
		if (!ui.requireResident()) return;
		replying = true;
	}

	function send() {
		if (!body.trim()) return;
		demo.reply(thread.id, body.trim());
		body = '';
		replying = false;
	}
</script>

<div
	class="overflow-hidden rounded-xl border border-line font-sans text-sm leading-normal"
	data-content-class="user"
>
	<div
		class="flex items-center gap-2 border-b border-line bg-subtle px-3.5 py-2 text-[13px] text-muted"
	>
		<span>
			Line comments on {rangeLabel(thread.anchor.from, thread.anchor.to)} · discussion only, no counts
		</span>
		<button
			type="button"
			class="ml-auto inline-flex size-9 cursor-pointer items-center justify-center rounded-lg hover:bg-card"
			aria-label="Share this thread"
			onclick={() => (ui.share = { kind: 'thread', threadId: thread.id })}
		>
			<Icon name="share" size={14} />
		</button>
	</div>
	{#each thread.comments as c, i (c.id)}
		<div
			class="flex flex-col gap-1.5 border-b border-line-soft px-3.5 py-3.5 {i > 0
				? 'pl-8'
				: ''} {c.stewardContext ? 'bg-[#f3f7ff]' : ''}"
			data-content-class={c.stewardContext ? 'platform' : 'user'}
		>
			<div class="flex flex-wrap items-center gap-1.5 text-[13px]">
				<strong>{c.author}</strong>
				{#if c.stewardContext}
					<span class="chip bg-support-tint text-support-strong">Steward context</span>
					<span class="text-muted">{c.office}</span>
				{/if}
				<span class="text-faint">· {c.at}</span>
				{#if demo.persona.role !== 'reader'}
					{#if demo.flags.includes(c.id)}
						<span class="ml-auto text-xs font-semibold text-oppose-ink">
							Flagged for moderators
						</span>
						{#if demo.persona.role === 'moderator'}
							<button
								type="button"
								class="btn-future min-h-8 px-2.5 text-xs"
								onclick={() => ui.future('moderation-remove')}
							>
								Remove · M2
							</button>
						{/if}
					{:else}
						<button
							type="button"
							class="ml-auto inline-flex min-h-8 cursor-pointer items-center gap-1 rounded px-2 text-xs text-faint hover:bg-subtle"
							onclick={() => {
								demo.flag(c.id);
								ui.notify('Sent to moderators. Only rule-breaking content is removed.');
							}}
						>
							<Icon name="flag" size={12} /> Flag
						</button>
					{/if}
				{/if}
			</div>
			<div>{c.body}</div>
		</div>
	{/each}
	<div class="px-3.5 py-2.5">
		{#if replying}
			<form
				class="flex flex-col gap-2"
				onsubmit={(e) => {
					e.preventDefault();
					send();
				}}
			>
				<label class="sr-only" for="reply-{thread.id}">Reply</label>
				<textarea id="reply-{thread.id}" rows="2" bind:value={body} class="field"></textarea>
				<div class="flex gap-2">
					<button type="submit" class="btn-primary">Post reply</button>
					<button type="button" class="btn-ghost text-ink-2" onclick={() => (replying = false)}>
						Cancel
					</button>
				</div>
			</form>
		{:else}
			<button type="button" class="btn-ghost px-3" onclick={startReply}>Reply</button>
		{/if}
	</div>
</div>
