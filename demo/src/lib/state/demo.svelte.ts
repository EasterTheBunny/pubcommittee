import { browser } from '$app/env';
import {
	BRIEF_MINIMUM,
	personaById,
	personas,
	seedSignals,
	seedSuggestions,
	seedThreads,
	SMALL_COUNT,
	catalogEntry,
	REQUEST_THRESHOLD,
	seedFeed
} from '#lib/mock/index.ts';
import type {
	Anchor,
	FeedEvent,
	BillSignal,
	OfficeResponse,
	OpposeReason,
	ResponseStatus,
	SuggestedEdit,
	Thread
} from '#lib/types.ts';

// In-memory demo state, saved per browser so a demo survives a reload.
// "Reset demo" restores the seed (M0 allows reset-to-seed; see design/MILESTONES.md).

const STORAGE_KEY = 'pubcommittee-m0-demo-v1';

interface Snapshot {
	personaId: string;
	threads: Thread[];
	suggestions: SuggestedEdit[];
	supports: Record<string, string[]>;
	signals: Record<string, Record<string, BillSignal>>;
	responses: Record<string, OfficeResponse>;
	flags: string[];
	requests: Record<string, string[]>;
	read: Record<string, string[]>;
	deviceNotify: Record<string, boolean>;
}

const seed = (): Snapshot => ({
	personaId: 'reviewer',
	threads: structuredClone(seedThreads),
	suggestions: structuredClone(seedSuggestions),
	supports: {},
	signals: {},
	responses: {},
	flags: [],
	requests: {},
	read: {},
	deviceNotify: {}
});

let nextId = Date.now();
const newId = (prefix: string) => `${prefix}${(nextId++).toString(36)}`;
const today = () => new Date().toISOString().slice(0, 10);

class Demo {
	personaId = $state('reviewer');
	threads = $state<Thread[]>(structuredClone(seedThreads));
	suggestions = $state<SuggestedEdit[]>(structuredClone(seedSuggestions));
	/** personaId -> supported suggestion ids. Private: only totals are ever shown. */
	supports = $state<Record<string, string[]>>({});
	/** personaId -> billId -> bill-level signal. Private: only totals are ever shown. */
	signals = $state<Record<string, Record<string, BillSignal>>>({});
	/** suggestionId -> public office response. */
	responses = $state<Record<string, OfficeResponse>>({});
	flags = $state<string[]>([]);
	/** personaId -> catalog ids requested for markup. Private: only totals are ever shown. */
	requests = $state<Record<string, string[]>>({});
	/** personaId -> feed event ids read in the inbox. */
	read = $state<Record<string, string[]>>({});
	/** personaId -> device notifications turned on for this browser (M2: a Web Push subscription). */
	deviceNotify = $state<Record<string, boolean>>({});

	persona = $derived(personaById(this.personaId));
	/** Reviewers, stewards and moderators are residents and may signal. Officials and readers may not. */
	canSignal = $derived(['reviewer', 'steward', 'moderator'].includes(this.persona.role));

	#started = false;

	/** Loads saved state and starts autosave. Call once from the root layout in the browser. */
	init() {
		if (!browser || this.#started) return;
		this.#started = true;
		try {
			const raw = localStorage.getItem(STORAGE_KEY);
			if (raw) this.#apply({ ...seed(), ...(JSON.parse(raw) as Partial<Snapshot>) });
		} catch {
			// Storage unavailable or corrupt: keep the seed.
		}
		$effect.root(() => {
			$effect(() => {
				const snap = JSON.stringify(this.#snapshot());
				try {
					localStorage.setItem(STORAGE_KEY, snap);
				} catch {
					// Private mode or blocked storage: the demo still works for this session.
				}
			});
		});
	}

	reset() {
		this.#apply(seed());
	}

	#snapshot(): Snapshot {
		return {
			personaId: this.personaId,
			threads: this.threads,
			suggestions: this.suggestions,
			supports: this.supports,
			signals: this.signals,
			responses: this.responses,
			flags: this.flags,
			requests: this.requests,
			read: this.read,
			deviceNotify: this.deviceNotify
		};
	}

	#apply(s: Snapshot) {
		this.personaId = personas.some((p) => p.id === s.personaId) ? s.personaId : 'reviewer';
		this.threads = s.threads;
		this.suggestions = s.suggestions;
		this.supports = s.supports;
		this.signals = s.signals;
		this.responses = s.responses;
		this.flags = s.flags;
		this.requests = s.requests;
		this.read = s.read;
		this.deviceNotify = s.deviceNotify;
	}

	// --- Suggested edits ----------------------------------------------------

	suggestion(id: string) {
		return this.suggestions.find((s) => s.id === id);
	}

	supportCount(id: string): number {
		const s = this.suggestion(id);
		if (!s) return 0;
		const extra = Object.values(this.supports).filter((ids) => ids.includes(id)).length;
		return s.seedSupport + extra;
	}

	hasSupported(id: string): boolean {
		return (this.supports[this.personaId] ?? []).includes(id);
	}

	toggleSupport(id: string) {
		const mine = this.supports[this.personaId] ?? [];
		this.supports[this.personaId] = mine.includes(id)
			? mine.filter((x) => x !== id)
			: [...mine, id];
	}

	briefEligible(id: string): boolean {
		return this.supportCount(id) >= BRIEF_MINIMUM;
	}

	addSuggestion(anchor: Anchor, title: string, replacement: string, reason: string): string {
		const id = newId('e');
		// Authors do not automatically support their own suggestion (design/11): a count of 1
		// would otherwise identify the author. They may still support it themselves.
		this.suggestions.push({
			id,
			anchor,
			title,
			replacement,
			reason,
			author: this.persona.name,
			seedSupport: 0,
			wellFormed: false,
			revisions: 0
		});
		return id;
	}

	markWellFormed(id: string, value = true) {
		const s = this.suggestion(id);
		if (s) s.wellFormed = value;
	}

	// --- Line comments ------------------------------------------------------

	threadsFor(billId: string, version: number) {
		return this.threads.filter((t) => t.anchor.billId === billId && t.anchor.version === version);
	}

	addThread(anchor: Anchor, body: string): string {
		const id = newId('t');
		this.threads.push({ id, anchor, comments: [this.#comment(body)] });
		return id;
	}

	reply(threadId: string, body: string) {
		this.threads.find((t) => t.id === threadId)?.comments.push(this.#comment(body));
	}

	#comment(body: string) {
		const p = this.persona;
		return {
			id: newId('c'),
			author: p.name,
			body,
			at: today(),
			stewardContext: p.role === 'steward' || undefined,
			office: p.office
		};
	}

	flag(commentId: string) {
		if (!this.flags.includes(commentId)) this.flags.push(commentId);
	}

	// --- Bill-level signal --------------------------------------------------

	signalFor(billId: string): BillSignal | undefined {
		return this.signals[this.personaId]?.[billId];
	}

	setSignal(billId: string, signal: BillSignal | null) {
		const mine = { ...(this.signals[this.personaId] ?? {}) };
		if (signal) mine[billId] = signal;
		else delete mine[billId];
		this.signals[this.personaId] = mine;
	}

	/** Seeded totals plus every demo persona's signals. */
	billTotals(billId: string) {
		const base = seedSignals[billId] ?? {
			support: 0,
			oppose: 0,
			reviewers: 0,
			reasons: { policy: 0, riders: 0, complex: 0 },
			riderTags: {}
		};
		const reasons: Record<OpposeReason, number> = { ...base.reasons };
		const riderTags: Record<string, number> = { ...base.riderTags };
		let support = base.support;
		let oppose = base.oppose;
		let reviewers = base.reviewers;

		for (const [pid, bySignal] of Object.entries(this.signals)) {
			const sig = bySignal[billId];
			const supportedHere = (this.supports[pid] ?? []).some(
				(sid) => this.suggestion(sid)?.anchor.billId === billId
			);
			if (sig || supportedHere) reviewers += 1;
			if (!sig) continue;
			if (sig.value === 'support') support += 1;
			else {
				oppose += 1;
				for (const r of sig.reasons) reasons[r] += 1;
				for (const s of sig.riderSections) riderTags[s] = (riderTags[s] ?? 0) + 1;
			}
		}
		return { support, oppose, reviewers, reasons, riderTags };
	}

	// --- Markup requests (design/01) ---------------------------------------

	requestCount(entryId: string): number {
		const base = catalogEntry(entryId)?.seedRequests ?? 0;
		return base + Object.values(this.requests).filter((ids) => ids.includes(entryId)).length;
	}

	hasRequested(entryId: string): boolean {
		return (this.requests[this.personaId] ?? []).includes(entryId);
	}

	toggleRequest(entryId: string) {
		const mine = this.requests[this.personaId] ?? [];
		this.requests[this.personaId] = mine.includes(entryId)
			? mine.filter((x) => x !== entryId)
			: [...mine, entryId];
	}

	/** In the steward queue once requests reach the threshold and no decision exists. */
	inStewardQueue(entryId: string): boolean {
		const e = catalogEntry(entryId);
		return (
			!!e &&
			e.markupStatus === 'eligible' &&
			!e.decision &&
			this.requestCount(entryId) >= REQUEST_THRESHOLD
		);
	}

	// --- Notifications (design/08) -----------------------------------------

	/**
	 * Public events joined with this persona's own items, the same join job-notify does in
	 * memory in M2. The demo shows matching events regardless of when the persona acted.
	 */
	inbox = $derived.by((): FeedEvent[] => {
		const supported = new Set(this.supports[this.personaId] ?? []);
		const signalled = new Set(Object.keys(this.signals[this.personaId] ?? {}));
		const requested = new Set(this.requests[this.personaId] ?? []);
		const supportedBills = new Set(
			[...supported].map((id) => this.suggestion(id)?.anchor.billId).filter(Boolean) as string[]
		);
		const responses: FeedEvent[] = Object.entries(this.responses).map(([sid, r]) => {
			const s = this.suggestion(sid);
			return {
				id: `resp-${sid}-${r.status}`,
				date: today(),
				kind: 'response',
				target: { kind: 'suggestion', id: sid, billId: s?.anchor.billId ?? '' },
				text: `An office responded to a suggestion you support: ${
					{ adopted: 'adopted', consider: 'under consideration', declined: 'declined' }[r.status]
				}.`,
				href: `/briefs/${s?.anchor.billId ?? ''}`
			};
		});
		return [...seedFeed, ...responses]
			.filter((e) => {
				const t = e.target;
				if (t.kind === 'suggestion') return supported.has(t.id);
				if (t.kind === 'request') return requested.has(t.id);
				return signalled.has(t.billId) || supportedBills.has(t.billId);
			})
			.sort((a, b) => b.date.localeCompare(a.date));
	});

	unread = $derived(
		this.inbox.filter((e) => !(this.read[this.personaId] ?? []).includes(e.id)).length
	);

	isRead(id: string) {
		return (this.read[this.personaId] ?? []).includes(id);
	}

	markAllRead() {
		this.read[this.personaId] = this.inbox.map((e) => e.id);
	}

	setDeviceNotify(on: boolean) {
		this.deviceNotify[this.personaId] = on;
	}

	// --- Office responses ---------------------------------------------------

	respond(suggestionId: string, status: ResponseStatus, reason = '') {
		this.responses[suggestionId] = { status, reason, office: this.persona.office ?? 'Office' };
	}

	clearResponse(suggestionId: string) {
		delete this.responses[suggestionId];
	}
}

export const demo = new Demo();

export const pct = (n: number, d: number) => (d > 0 ? Math.round((n / d) * 100) : 0);
export const fmt = (n: number) => n.toLocaleString('en-US');

/** True when a count is too small to publish exactly (design/11). */
export const isSmall = (n: number) => n < SMALL_COUNT;

/** Public form of a count: exact, or "fewer than N" when small (design/03, design/11). */
export const count = (n: number) => (isSmall(n) ? `fewer than ${SMALL_COUNT}` : fmt(n));

/** Public share of reviewers, omitted when the count is small. */
export const share = (n: number, d: number) => (isSmall(n) ? '' : `${pct(n, d)}%`);
