<script lang="ts">
	import { billById, latestVersion } from '#lib/mock/index.ts';
	import { demo } from '#lib/state/demo.svelte.ts';
	import { ui } from '#lib/state/ui.svelte.ts';
	import type { OpposeReason } from '#lib/types.ts';
	import Modal from './Modal.svelte';

	let open = $state(false);
	let billId = $state<string | null>(null);
	let reasons = $state<OpposeReason[]>([]);
	let riderSections = $state<string[]>([]);
	let comment = $state('');

	const bill = $derived(billId ? billById(billId) : undefined);
	const sections = $derived(bill ? latestVersion(bill).sections : []);

	$effect(() => {
		if (ui.oppose) {
			billId = ui.oppose;
			const existing = demo.signalFor(ui.oppose);
			const isOppose = existing?.value === 'oppose';
			reasons = isOppose ? [...existing.reasons] : [];
			riderSections = isOppose ? [...existing.riderSections] : [];
			comment = isOppose ? existing.comment : '';
			open = true;
		}
	});
	$effect(() => {
		if (!open) ui.oppose = null;
	});

	const reasonDefs: { id: OpposeReason; label: string; help: string }[] = [
		{
			id: 'complex',
			label: 'Too complex',
			help: 'Hard to follow, or tries to do too much at once.'
		},
		{
			id: 'riders',
			label: 'Unrelated riders',
			help: 'Carries provisions disconnected from its main purpose.'
		},
		{ id: 'policy', label: 'Disagree with the policy', help: 'Opposition to the bill’s substance.' }
	];

	function save() {
		if (!billId) return;
		demo.setSignal(billId, {
			value: 'oppose',
			reasons: $state.snapshot(reasons),
			riderSections: reasons.includes('riders') ? $state.snapshot(riderSections) : [],
			comment
		});
		open = false;
		ui.notify('Opposition recorded. Only totals leave the system.');
	}
</script>

<Modal
	bind:open
	title="Oppose this bill"
	eyebrow={bill ? `${bill.number} · ${bill.title}` : undefined}
>
	<fieldset class="m-0 flex flex-col gap-2.5 border-0 p-0">
		<legend class="mb-2 text-sm font-semibold">Reasons (optional, pick any)</legend>
		{#each reasonDefs as r (r.id)}
			<label
				class="flex cursor-pointer items-start gap-3 rounded-xl border px-3.5 py-3 {reasons.includes(
					r.id
				)
					? 'border-oppose-line bg-oppose-tint'
					: 'border-line'}"
			>
				<input
					type="checkbox"
					value={r.id}
					bind:group={reasons}
					class="mt-0.5 size-5 rounded text-oppose"
				/>
				<span>
					<strong>{r.label}</strong><br /><span class="text-sm text-muted">{r.help}</span>
				</span>
			</label>
			{#if r.id === 'riders' && reasons.includes('riders')}
				<div class="ml-8 flex flex-col gap-1.5 border-l-2 border-oppose-line py-1 pl-3.5">
					<div class="text-[13px] font-semibold text-muted">
						Which sections do you consider riders?
					</div>
					{#each sections as s (s.id)}
						<label class="flex min-h-8 cursor-pointer items-center gap-2.5 text-sm">
							<input
								type="checkbox"
								value={s.id}
								bind:group={riderSections}
								class="size-4.5 rounded text-oppose"
							/>
							{s.label} · {s.heading.toLowerCase().replace(/\.$/, '')}
						</label>
					{/each}
				</div>
			{/if}
		{/each}
	</fieldset>

	<label class="flex flex-col gap-1.5 text-sm font-semibold">
		Comment (optional, private)
		<textarea rows="2" bind:value={comment} class="field font-normal"></textarea>
	</label>

	<div class="flex flex-wrap items-center gap-3">
		<span class="flex-[1_1_260px] text-[13px] text-muted">
			Your position and comment are private. Only totals leave the system; your comment may reach
			the brief only as an anonymous excerpt. You can withdraw while the bill is in review.
		</span>
		<button type="button" class="btn-ghost text-ink-2" onclick={() => (open = false)}>
			Cancel
		</button>
		<button type="button" class="btn-oppose-solid" onclick={save}>Record opposition</button>
	</div>
</Modal>
