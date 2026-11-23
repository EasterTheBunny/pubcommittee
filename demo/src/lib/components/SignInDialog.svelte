<script lang="ts">
	import { personas } from '#lib/mock/index.ts';
	import type { PlaceholderId } from '#lib/placeholders.ts';
	import { demo } from '#lib/state/demo.svelte.ts';
	import { ui } from '#lib/state/ui.svelte.ts';
	import Modal from './Modal.svelte';
	import PlaceholderNote from './PlaceholderNote.svelte';

	let provider = $state<PlaceholderId | null>(null);

	const pick = (id: string) => {
		demo.personaId = id;
		ui.signIn = false;
		provider = null;
		ui.notify(`Now viewing as ${demo.persona.name}`);
	};
</script>

<Modal bind:open={ui.signIn} title="Join the review" width="max-w-4xl">
	<div class="grid gap-6 md:grid-cols-2">
		<div class="flex flex-col gap-3.5">
			<p class="m-0 text-sm leading-relaxed text-ink-2">
				Reading is open to everyone. Sign in to comment, suggest edits, and support or oppose bills.
			</p>
			<button
				type="button"
				class="btn-secondary min-h-12"
				onclick={() => (provider = 'signin-google')}
			>
				Continue with Google
			</button>
			<button
				type="button"
				class="btn-secondary min-h-12"
				onclick={() => (provider = 'signin-facebook')}
			>
				Continue with Facebook
			</button>
			{#if provider}
				<PlaceholderNote id={provider} />
			{/if}
			<button
				type="button"
				class="btn-ghost self-start"
				onclick={() => (provider = 'address-confirm')}
			>
				Verification tiers
			</button>
			<p class="mt-auto text-xs text-muted">
				Your support and opposition stay private. Comments are public under a display name.
			</p>
		</div>

		<div class="flex flex-col gap-3 rounded-xl bg-subtle p-4">
			<div class="flex items-center gap-2">
				<span class="chip bg-oppose-soft text-oppose-ink">POC only</span>
				<h3 class="m-0 text-base font-semibold">Pick a mock persona</h3>
			</div>
			{#each personas as p (p.id)}
				<button
					type="button"
					onclick={() => pick(p.id)}
					aria-pressed={demo.personaId === p.id}
					class="flex cursor-pointer flex-col items-start gap-0.5 rounded-xl border bg-card px-3.5 py-3 text-left hover:bg-support-soft {demo.personaId ===
					p.id
						? 'border-2 border-support'
						: 'border-line'}"
				>
					<strong class="text-sm">
						{p.role === 'reader'
							? 'Reader'
							: `${p.role[0].toUpperCase()}${p.role.slice(1)} · ${p.name}`}
					</strong>
					<span class="text-[13px] text-muted">{p.office ? `${p.office}. ` : ''}{p.blurb}</span>
				</button>
			{/each}
		</div>
	</div>
</Modal>
