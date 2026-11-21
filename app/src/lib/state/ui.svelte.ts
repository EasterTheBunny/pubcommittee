import type { PlaceholderId } from '#lib/placeholders.ts';
import type { ShareTarget } from '#lib/types.ts';
import { demo } from './demo.svelte';

// Global dialog state. Dialogs are rendered once, in the root layout.
class UI {
	placeholder = $state<PlaceholderId | null>(null);
	signIn = $state(false);
	share = $state<ShareTarget | null>(null);
	/** Bill id whose oppose dialog is open. */
	oppose = $state<string | null>(null);
	toast = $state<string | null>(null);

	#toastTimer: ReturnType<typeof setTimeout> | undefined;

	future(id: PlaceholderId) {
		this.placeholder = id;
	}

	notify(message: string) {
		this.toast = message;
		clearTimeout(this.#toastTimer);
		this.#toastTimer = setTimeout(() => (this.toast = null), 3500);
	}

	/**
	 * Gate for commenting, suggesting and signalling. Readers are invited to sign in at
	 * the point they try to act (design/06); officials cannot signal (design/05).
	 */
	requireResident(): boolean {
		if (demo.canSignal) return true;
		if (demo.persona.role === 'reader') this.signIn = true;
		else this.notify('Legislator and office accounts cannot comment, suggest, or signal.');
		return false;
	}
}

export const ui = new UI();
