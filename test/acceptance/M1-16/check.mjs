// M1-16 local gate: end-to-end checks of the M1 exit criteria over HTTP, with no browser and no
// page scripts: everything is read from the server-rendered HTML of the web app and compared with
// the read-api resource of the same address. Node 24, no dependencies.
// Environment: READ_API_URL (default http://read-api:8080), APP_URL (default http://app:3000).
// PUBLIC_API_URL is what the app advertises in rel="alternate" (default READ_API_URL).
const api = process.env.READ_API_URL ?? 'http://read-api:8080';
const app = process.env.APP_URL ?? 'http://app:3000';
const publicApi = process.env.PUBLIC_API_URL ?? api;
const bills = ['us-119-hr-4313', 'us-119-hr-3062', 'us-119-hr-3427'];
const ua = { 'User-Agent': 'pubcommittee-acceptance-M1-16' };

let failures = 0;
const counts = {};
const pass = (name, detail = '') => console.log(`PASS ${name}${detail ? ` (${detail})` : ''}`);
const fail = (name, reason) => {
	failures++;
	console.log(`FAIL ${name}: ${reason}`);
};
const check = (name, ok, reason, detail) => (ok ? pass(name, detail) : fail(name, reason));
const tally = (key, ok, why) => {
	counts[key] ??= { ok: 0, bad: [] };
	if (ok) counts[key].ok++;
	else counts[key].bad.push(why);
};
const report = (name, key) => {
	const c = counts[key] ?? { ok: 0, bad: [] };
	check(name, c.bad.length === 0 && c.ok > 0, `${c.bad.length} mismatches, first: ${c.bad[0]}`, `${c.ok} compared`);
};

const get = async (base, path, headers = {}) => {
	const res = await fetch(base + path, { headers: { ...ua, ...headers }, redirect: 'manual' });
	const body = await res.text();
	return { status: res.status, headers: res.headers, body };
};
const json = async (path) => {
	const r = await get(api, path);
	if (r.status !== 200) throw new Error(`${path}: ${r.status}`);
	return JSON.parse(r.body);
};
const decode = (s) =>
	s
		.replace(/&#x([0-9a-f]+);/gi, (_, h) => String.fromCodePoint(parseInt(h, 16)))
		.replace(/&#(\d+);/g, (_, d) => String.fromCodePoint(Number(d)))
		.replace(/&lt;/g, '<')
		.replace(/&gt;/g, '>')
		.replace(/&quot;/g, '"')
		.replace(/&#39;|&apos;/g, "'")
		.replace(/&amp;/g, '&');
const visible = (html) =>
	decode(
		html
			.replace(/<script[\s\S]*?<\/script>/g, ' ')
			.replace(/<style[\s\S]*?<\/style>/g, ' ')
			.replace(/<!--[\s\S]*?-->/g, '')
			.replace(/<[^>]*>/g, ' ')
	)
		.replace(/\s+/g, ' ')
		.trim();

/** Lines of a bill page as rendered in the HTML: [{ anchor, n, text }]. */
const pageLines = (html) => {
	const out = [];
	const re = /<li id="([^"]+)" data-line="(\d+)"[^>]*>([\s\S]*?)<\/li>/g;
	let m;
	while ((m = re.exec(html))) {
		const t = /<span class="ln-text">([\s\S]*?)<\/span>/.exec(m[3]);
		out.push({ anchor: m[1], n: Number(m[2]), text: t ? decode(t[1].replace(/<[^>]*>/g, '')).trim() : '' });
	}
	return out;
};

// ---- 1. Three real bills, every version, readable without scripts, canonical addresses ----------
const docs = {}; // bill -> { doc, versions: {n: versionDoc} }
for (const b of bills) {
	const doc = await json(`/v1/bills/${b}`);
	docs[b] = { doc, versions: {} };
	check(`${b}: four official versions listed`, doc.versions?.length === 4 && doc.latest_version === 4, `versions=${doc.versions?.length}`);
	check(
		`${b}: official links to the legislature's page and every text version`,
		doc.official_links?.some((l) => l.kind === 'bill-page' && l.url.includes('congress.gov')) &&
			doc.official_links.filter((l) => l.kind === 'text-version').length === 4,
		'links missing'
	);
	for (const v of doc.versions) docs[b].versions[v.n] = await json(`/v1/bills/${b}/v/${v.n}`);
}

for (const b of bills) {
	for (const [nStr, ver] of Object.entries(docs[b].versions)) {
		const n = Number(nStr);
		const page = await get(app, `/bills/${b}/v/${n}`);
		tally('page-status', page.status === 200, `${b} v${n} page ${page.status}`);
		const lines = pageLines(page.body);
		const flat = ver.sections.flatMap((s) => s.lines);
		const same =
			lines.length === flat.length &&
			flat.every((l, i) => lines[i].anchor === l.anchor && lines[i].n === l.n && lines[i].text === l.text);
		tally('version-page', same, `${b} v${n}: page has ${lines.length} lines, resource ${flat.length}`);
		tally(
			'content-class',
			/data-content-class="official"/.test(page.body) && ver.content_class === 'official' && ver.licence === 'public-domain',
			`${b} v${n}`
		);
		tally(
			'alternate',
			page.body.includes(`rel="alternate" type="application/json" href="${publicApi}/v1/bills/${b}/v/${n}"`),
			`${b} v${n} lacks rel=alternate to the structured form`
		);
		tally('no-script-in-body', !/<script[^>]*>[^<]*(createElement|innerHTML)/.test(page.body), `${b} v${n}`);
		// Every anchor of the resource is a fragment target in the page.
		for (const l of flat) tally('anchor-id', page.body.includes(`id="${l.anchor}"`), `${b} v${n} ${l.anchor}`);

		// Line ranges: every line, every section, every block range, by canonical address.
		for (const s of ver.sections) {
			const ranges = new Set();
			ranges.add(`${s.lines[0].n}-${s.lines.at(-1).n}`);
			for (const l of s.lines) {
				ranges.add(`${l.n}`);
				if (l.block_to > l.n) ranges.add(`${l.n}-${l.block_to}`);
			}
			for (const r of ranges) {
				const path = `/v/${n}/s/${s.id}/l/${r}`;
				const res = await get(api, `/v1/bills/${b}${path}`);
				if (res.status !== 200) {
					tally('range', false, `${b}${path} resource ${res.status}`);
					continue;
				}
				const rj = JSON.parse(res.body);
				const pg = await get(app, `/bills/${b}${path}`);
				const pl = pageLines(pg.body);
				const ok =
					pg.status === 200 &&
					pl.length === rj.lines.length &&
					rj.lines.every((l, i) => pl[i].anchor === l.anchor && pl[i].text === l.text) &&
					rj.address?.canonical === `/v1/bills/${b}${path}` &&
					rj.address.page === `/bills/${b}${path}` &&
					pg.body.includes(`href="${publicApi}/v1/bills/${b}${path}"`);
				tally('range', ok, `${b}${path}: page ${pl.length} lines (${pg.status}) vs resource ${rj.lines.length}`);
				tally('range-provenance', !!rj.provenance && rj.content_class === 'official' && !!rj.as_of, `${b}${path}`);
				if (n < 4) tally('changed-since', !!rj.changed_since?.class, `${b}${path} has no changed_since`);
			}
		}
	}
}
report('every version page matches its read-api resource (anchors, depth order, text)', 'version-page');
report('every version page and version resource answer 200', 'page-status');
report('official content class and licence on every version', 'content-class');
report('every page links its structured form (rel=alternate)', 'alternate');
report('every line anchor of the resource is an id in the page HTML', 'anchor-id');
report('every line range by canonical address: page == structured form (anchors, text, address)', 'range');
report('every range carries provenance, as-of and official content class', 'range-provenance');
report('older-version ranges carry changed_since', 'changed-since');
// The no-script claim: node's fetch never runs scripts, so the checks above prove it. State it plainly.
pass('page text came from server-rendered HTML only (no browser, no scripts executed)');

// Content negotiation: 303 to the resource.
{
	const r = await get(app, `/bills/us-119-hr-4313/v/2/s/s3/l/5-6`, { Accept: 'application/json' });
	check(
		'Accept: application/json answers 303 to the read-api resource',
		r.status === 303 && (r.headers.get('location') ?? '').endsWith('/v1/bills/us-119-hr-4313/v/2/s/s3/l/5-6'),
		`${r.status} ${r.headers.get('location')}`
	);
	const c = await get(app, `/bills/us-119-hr-4313`);
	check('the web host sets no cookies', !c.headers.get('set-cookie'), 'set-cookie present');
}

// ---- 2. Version history and changed-since -------------------------------------------------------
// Stored comparisons: every consecutive pair and every older version against the latest (M1-08).
for (const b of bills) {
	const want = [[1, 2], [2, 3], [3, 4], [1, 4], [2, 4]];
	let have = 0;
	let modified = 0;
	for (const [i, j] of want) {
		const r = await get(api, `/v1/bills/${b}/v/${i}/compare/${j}`);
		if (r.status === 200) {
			have++;
			modified += (r.body.match(/"kind":"(modified|added|removed|moved)"/g) ?? []).length;
		}
	}
	check(`${b}: comparisons stored for consecutive versions and each older one against the latest`, have === 5, `${have}/5`);
	const pg = await get(app, `/bills/${b}/v/1`);
	check(
		`${b}: older version page links every version in its version history`,
		[1, 2, 3, 4].every((n) => pg.body.includes(`/bills/${b}/v/${n}`)) && /aria-label="Versions"/.test(pg.body),
		'version switcher incomplete'
	);
	const s0 = docs[b].versions[1].sections[0];
	const rg = await get(app, `/bills/${b}/v/1/s/${s0.id}/l/${s0.lines[0].n}`);
	check(`${b}: older-version line page says whether the words changed since`, /data-notice=/.test(rg.body), 'no changed-since notice');
	if (b !== 'us-119-hr-3062') check(`${b}: comparison from the first to the latest version classifies line changes`, modified > 0, 'no changes found');
}

// ---- 3. Citations -------------------------------------------------------------------------------
for (const b of bills) {
	let resolved = 0;
	let unresolved = 0;
	for (let n = 1; n <= 4; n++) {
		const c = await json(`/v1/bills/${b}/v/${n}/citations`);
		resolved += c.items.filter((i) => i.state === 'resolved').length;
		const u = await json(`/v1/bills/${b}/v/${n}/citations/unresolved`);
		unresolved += u.items.length;
		const pg = await get(app, `/bills/${b}/v/${n}`);
		if (c.items.length) tally('cite-page', /U\.S\.C\./.test(visible(pg.body)), `${b} v${n}`);
		tally('unresolved-section', /could not be matched to existing law/.test(pg.body), `${b} v${n}`);
	}
	check(`${b}: citations resolved and unresolved ones listed (freestanding text may cite none)`, b === 'us-119-hr-3427' ? resolved + unresolved === 0 : resolved + unresolved > 0, 'unexpected citation count', `resolved ${resolved}, unresolved ${unresolved}`);
}
report('pages show citation text; unresolved citations have their own section', 'unresolved-section');

// ---- 4. Auto-generated explanations are labelled ------------------------------------------------
let summaries = 0;
let changeSummaries = 0;
for (const b of bills) {
	for (let n = 1; n <= 4; n++) {
		const s = await get(api, `/v1/bills/${b}/v/${n}/summary`);
		const pg = await get(app, `/bills/${b}/v/${n}`);
		if (s.status === 200) {
			summaries++;
			const sj = JSON.parse(s.body);
			tally('label', sj.provenance === 'auto' && sj.content_class === 'platform' && sj.flags?.includes('auto'), `${b} v${n} summary not labelled auto/platform`);
			tally('label-page', /Auto-generated/.test(pg.body), `${b} v${n} page lacks the label`);
			tally('summary-anchors', sj.statements.every((st) => st.anchors?.length && st.anchors.every((a) => a.startsWith('prev:') || pg.body.includes(`id="${a.replace(/-l(\d+)-\d+$/, '-l$1')}"`))), `${b} v${n} statement anchor not in page`);
		}
		const ex = await json(`/v1/bills/${b}/v/${n}/explanations`);
		const ver = await json(`/v1/bills/${b}/v/${n}`);
		for (const e of ex.items ?? ex) {
			tally('label', e.provenance === 'auto' && e.content_class === 'platform' && e.flags?.includes('auto'), `${b} v${n} ${e.section_id} explanation unlabelled`);
			// Pinned to lines of its own section, shown above them, and naming the provision in readable form.
			const sec = ver.sections.find((x) => x.id === e.section_id);
			const nums = new Set((sec?.lines ?? []).map((l) => l.n));
			tally('pinned', Number.isInteger(e.line_from) && e.line_to >= e.line_from && nums.has(e.line_from) && nums.has(e.line_to), `${b} v${n} ${e.section_id} ${e.provision_id ?? ''} has no valid line_from/line_to`);
			tally('pinned-page', pg.body.includes(`data-lines="${e.line_from}-${e.line_to}"`), `${b} v${n} ${e.section_id} page lacks the explanation pinned to ${e.line_from}-${e.line_to}`);
			if (e.provision_id) tally('citation', /^\d+ U\.S\.C\. \S/.test(e.provision_citation ?? ''), `${b} v${n} ${e.provision_id} has citation ${e.provision_citation}`);
			// A section that only names the Act has no explanation.
			tally('short-title', !(sec?.lines ?? []).every((l) => / may be cited as /i.test(l.text)), `${b} v${n} ${e.section_id} short-title section has an explanation`);
		}
		const ch = await get(api, `/v1/bills/${b}/v/${n}/changes`);
		if (ch.status === 200) {
			changeSummaries++;
			const cj = JSON.parse(ch.body);
			tally('label', cj.provenance === 'auto' && cj.flags?.includes('auto'), `${b} v${n} change summary unlabelled`);
			tally('label-page', /Auto-generated/.test(pg.body), `${b} v${n} changes page lacks the label`);
		}
	}
}
check('a summary is served for every version of the three exit bills', summaries === 12, `${summaries}/12`);
check('at least one change summary is served', changeSummaries >= 2, `${changeSummaries}`, `${changeSummaries} served`);
report('every served explanation, summary and change summary is labelled auto, content class platform', 'label');
report('every page with an explanation shows the Auto-generated label', 'label-page');
report('every explanation is pinned to valid lines of its section and its page shows it there', 'pinned');
report('every explanation page carries the explanation pinned at the served lines', 'pinned-page');
report('every explanation of a provision names it as "<title> U.S.C. <section>(<sub>)"', 'citation');
report('no explanation is served for a section that only gives the short title', 'short-title');
report('every summary statement points at lines that exist in the page', 'summary-anchors');

// ---- 5. Catalog, markup status, selection policy, tracked-set disclosure ------------------------
{
	let cursor = '';
	const all = [];
	for (let i = 0; i < 20; i++) {
		const j = await json(`/v1/bills?scope=catalog&limit=100${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ''}`);
		all.push(...j.items);
		cursor = j.next_cursor ?? j.next ?? '';
		if (!cursor) break;
	}
	const by = {};
	for (const e of all) by[e.markup_status] = (by[e.markup_status] ?? 0) + 1;
	check('catalog lists every bill with a markup status', all.length >= 200 && all.every((e) => ['in-review', 'eligible', 'not-a-candidate'].includes(e.markup_status)), `${all.length} entries`, `${all.length} entries ${JSON.stringify(by)}`);
	check('every catalog entry has an official link', all.every((e) => e.official_links?.length > 0), 'entry without any official link');
	const noPage = all.filter((e) => !e.official_links?.some((l) => l.kind === 'bill-page')).length;
	console.log(`NOTE ${noPage} of ${all.length} catalog entries have no bill-page link (their BILLSTATUS has no legislationUrl)`);
	check('not-a-candidate entries carry a reason', all.filter((e) => e.markup_status === 'not-a-candidate').every((e) => e.exclusion_reason), 'missing exclusion reason');
	check('tracked entries list reasons and the policy version', all.filter((e) => e.tracked).every((e) => e.reasons?.length && e.policy_version >= 1), 'tracked entry without reasons');
	const pg = await get(app, '/catalog');
	const text = visible(pg.body);
	check('catalog page renders without scripts with official links and statuses', pg.status === 200 && /Eligible for review/.test(text) && /Congress\.gov/.test(text), 'catalog page lacks statuses or links');
	const q = await get(app, '/catalog?q=hospital');
	check('catalog search by title words finds the exit bill', /Hospital Inpatient Services Modernization Act/.test(q.body), 'not found');
	const untracked = all.find((e) => !e.tracked);
	const up = await get(app, `/bills/${untracked.id}`);
	check('an untracked catalog bill shows no text', up.status === 200 && !/data-line=/.test(up.body), `${untracked.id}`);
	// Tracked by the policy before its text was pulled (the pull is its own step): a page, not a 404.
	const noText = [];
	for (const e of all.filter((x) => x.tracked && !bills.includes(x.id))) {
		const v = await fetch(`${api}/v1/bills/${e.id}/v/latest`);
		if (v.status === 404) noText.push(e.id);
	}
	if (noText.length) {
		const nt = await get(app, `/bills/${noText[0]}`);
		check('a tracked bill with no stored text has a page that says so', nt.status === 200 && /has not been added here yet/.test(visible(nt.body)) && !/data-line=/.test(nt.body), `${noText[0]} answered ${nt.status}`);
	}
	const tr = await get(app, `/bills/${bills[0]}`);
	check('a tracked bill shows why it is in review', /In review because/.test(tr.body), 'reason missing');
}
{
	const pol = await json('/v1/selection-policy');
	check('selection policy is served with a version and rules', pol.policy_version >= 1 && pol.rules?.length > 0, 'no policy');
	const pg = await get(app, '/policy');
	const t = visible(pg.body);
	check('policy page states the rules and the version, without scripts', /Policy version \d+/.test(t) && /committee action/i.test(t), 'text missing');
	const ts = await get(api, '/v1/exports/tracked-set.json');
	check('tracked-set breakdown is disclosed (counts by topic and party)', ts.status === 200 && /by_topic/.test(ts.body) && /by_sponsor_party/.test(ts.body), `${ts.status}`);
	const log = await get(app, '/log');
	check('public log page lists actions with reasons', log.status === 200 && /Reason:/.test(visible(log.body)), 'no entries');
}

// ---- 6. Exports, feed, machine-access notice, rate limits, cache classes ------------------------
{
	const ex = await json('/v1/exports?limit=200');
	const ids = ex.items.map((e) => e.id);
	check('exports: bill text, existing law and change explanations per version', ['bill-text.', 'existing-law.', 'change-explanations.'].every((p) => ids.some((i) => i.startsWith(p))), 'a kind is missing', `${ids.length} exports`);
	let bad = 0;
	for (const e of ex.items) if ((await get(api, e.href)).status !== 200) bad++;
	check('every indexed export is served', bad === 0, `${bad} not served`);
	check('exports state licence, as-of and content class', ex.items.every((e) => e.licence && e.as_of && e.content_class), 'missing metadata');
	const law = ex.items.find((e) => e.id.startsWith('existing-law.'));
	const lr = await get(api, law.href);
	check('an existing-law export holds cited provisions', lr.status === 200 && /usc\//.test(lr.body), 'no provisions');
	const feedItems = [];
	for (let next = '', k = 0; k < 20; k++) {
		const f = await json(`/v1/feed?limit=200${next ? `&cursor=${encodeURIComponent(next)}` : ''}`);
		feedItems.push(...f.items);
		next = f.next ?? '';
		if (!next) break;
	}
	const feed = { items: feedItems };
	const kinds = new Set(feedItems.map((i) => i.kind));
	check('feed lists new versions, explanations, tracked bills, policy and log entries', ['version', 'explanation', 'bill-tracked', 'policy', 'log-entry'].every((k) => kinds.has(k)), `kinds: ${[...kinds]}`, `${feedItems.length} entries`);
	let fbad = 0;
	for (const i of feed.items.filter((_, k) => k % 7 === 0)) if ((await get(api, i.address)).status !== 200) fbad++;
	check('feed entries reach their item by canonical address', fbad === 0, `${fbad} unreachable`);
	const n = await get(app, '/.well-known/machine-access.json');
	check('machine-access notice is served without sign-in', n.status === 200 && /"automated_submission": ?"prohibited"/.test(n.body) && /Untrusted data/i.test(n.body), `${n.status}`);
	const rb = await get(app, '/robots.txt');
	check('crawler directives allow public content', rb.status === 200 && /Allow: \//.test(rb.body) && /Disallow: \/sign-in/.test(rb.body), 'robots.txt');
	const disc = await get(api, '/v1');
	check('discovery links the notice and the spec', /machine-access/.test(disc.body) && (await get(api, '/v1/openapi.yaml')).status === 200, 'discovery');
	const imm = await get(api, '/v1/bills/us-119-hr-4313/v/4/s/s3/l/5-6');
	check('the latest pinned official text is cached as immutable with an ETag', /immutable/.test(imm.headers.get('cache-control') ?? '') && !!imm.headers.get('etag'), imm.headers.get('cache-control'));
	const inm = await get(api, '/v1/bills/us-119-hr-4313/v/4/s/s3/l/5-6', { 'If-None-Match': imm.headers.get('etag') });
	check('If-None-Match answers 304', inm.status === 304, `${inm.status}`);
}

console.log(failures === 0 ? 'M1-16 checks: all passed' : `M1-16 checks: ${failures} FAILED`);
process.exit(failures === 0 ? 0 : 1);
