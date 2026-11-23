// The line range a reader selected on the page. Enhancement only: the same range is always
// reachable without scripts through its canonical path address (design/06 Link behavior).
import { anchorHash, parseHash } from '#lib/anchors.ts';

export interface Selected {
	sectionId: string;
	from: number;
	to: number;
}

class Selection {
	current = $state<Selected | null>(null);

	/** Read the "#s4-l12-13" fragment. */
	fromHash(hash: string) {
		const h = parseHash(hash);
		this.current =
			h && h.from !== undefined
				? { sectionId: h.sectionId, from: h.from, to: h.to ?? h.from }
				: null;
	}

	/** Click selects one line; shift-click extends within the same section. */
	pick(sectionId: string, n: number, extend: boolean) {
		const c = this.current;
		this.current =
			extend && c && c.sectionId === sectionId
				? { sectionId, from: Math.min(c.from, n), to: Math.max(c.to, n) }
				: { sectionId, from: n, to: n };
		history.replaceState(null, '', anchorHash(sectionId, this.current.from, this.current.to));
	}

	clear() {
		this.current = null;
		history.replaceState(null, '', location.pathname + location.search);
	}
}

export const selection = new Selection();
