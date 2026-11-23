// A tiny read-api stand-in for UI tests that need data the fixture read-api does not serve yet:
// a comparison between versions (M1-08) and a very long bill. Invented data only.
// Usage: node tests/stub-api.mjs [port]   (default 4010)
import http from 'node:http';

const sec = (id, label, heading, lines) => ({ id, path: [id], label, heading, lines });
const mkLines = (sid, texts) =>
	texts.map((text, i) => ({
		n: i + 1,
		depth: 0,
		text,
		anchor: `${sid}-l${i + 1}`,
		block_to: i + 1
	}));
const meta = {
	version: 1,
	as_of: '2026-03-01T12:00:00Z',
	provenance: 'source',
	content_class: 'official'
};

const v1 = [
	sec('s1', 'Section 1.', 'Short title', mkLines('s1', ['This Act is the Stub Act.'])),
	sec(
		's2',
		'Sec. 2.',
		'Hours',
		mkLines('s2', [
			'Offices open at eight.',
			'Offices close at five.',
			'Offices close on holidays.'
		])
	)
];
const v2 = [
	v1[0],
	sec(
		's2',
		'Sec. 2.',
		'Hours',
		mkLines('s2', ['Offices open at eight.', 'Offices close at six.', 'Offices close on holidays.'])
	)
];
const big = Array.from({ length: 400 }, (_, i) =>
	sec(
		`s${i + 1}`,
		`Sec. ${i + 1}.`,
		`Provision ${i + 1}`,
		mkLines(
			`s${i + 1}`,
			Array.from(
				{ length: 25 },
				(_, j) =>
					`Line ${j + 1} of provision ${i + 1}: the Secretary shall publish a report on the matters described in this section.`
			)
		)
	)
);

const versions = { stub: [v1, v2], big: [big] };
const bills = {
	stub: 'Stub Hours Act',
	big: 'Very Long Appropriations Act'
};
const id = (k) => `us-119-hr-${k === 'stub' ? '1' : '2'}`;
const key = (billId) =>
	billId === 'us-119-hr-1' ? 'stub' : billId === 'us-119-hr-2' ? 'big' : null;

function billDoc(k) {
	return {
		...meta,
		id: id(k),
		ref: { jurisdiction: 'us', session: '119', type: 'hr', number: k === 'stub' ? 1 : 2 },
		title: bills[k],
		status: 'reported',
		sponsor: 'Rep. Stub',
		review_state: 'open',
		latest_version: versions[k].length,
		address: `/v1/bills/${id(k)}`,
		official_links: [],
		versions: versions[k].map((_, i) => ({
			n: i + 1,
			code: i ? 'rh' : 'ih',
			type: i ? 'reported' : 'introduced',
			published_at: `2026-03-0${i + 1}T12:00:00Z`,
			address: `/v1/bills/${id(k)}/v/${i + 1}`
		}))
	};
}
const addr = (k, n, tail, page) => ({
	canonical: `/v1/bills/${id(k)}/v/${n}${tail}`,
	page,
	pinned_version: n,
	latest_version: versions[k].length,
	is_latest: n === versions[k].length,
	resolved_from_latest: false
});

const problem = (res, status) => {
	res.writeHead(status, { 'content-type': 'application/problem+json' });
	res.end(JSON.stringify({ type: '/problems/not-found', title: 'Not Found', status }));
};
const ok = (res, body) => {
	res.writeHead(200, { 'content-type': 'application/json' });
	res.end(JSON.stringify(body));
};

http
	.createServer((req, res) => {
		const u = new URL(req.url, 'http://x');
		let m;
		if (u.pathname === '/v1/bills' && u.searchParams.get('scope') === 'catalog')
			return ok(res, { items: [] });
		if ((m = /^\/v1\/bills\/([^/]+)$/.exec(u.pathname))) {
			const k = key(m[1]);
			return k ? ok(res, billDoc(k)) : problem(res, 404);
		}
		if ((m = /^\/v1\/bills\/([^/]+)\/v\/(\d+|latest)(\/.*)?$/.exec(u.pathname))) {
			const k = key(m[1]);
			if (!k) return problem(res, 404);
			const n = m[2] === 'latest' ? versions[k].length : Number(m[2]);
			const secs = versions[k][n - 1];
			if (!secs) return problem(res, 404);
			const rest = m[3] ?? '';
			if (rest === '') {
				return ok(res, {
					...meta,
					bill: id(k),
					n,
					code: n > 1 ? 'rh' : 'ih',
					type: 'x',
					published_at: '2026-03-01T12:00:00Z',
					source_sha256: 'x',
					licence: 'public-domain',
					address: addr(k, n, '', `/bills/${id(k)}/v/${n}`),
					sections: secs
				});
			}
			if (rest === '/explanations' || rest === '/citations') return ok(res, { items: [] });
			if ((m = /^\/s\/([^/]+)(?:\/l\/(\d+)(?:-(\d+))?)?$/.exec(rest))) {
				const s = secs.find((x) => x.id === m[1]);
				if (!s) return problem(res, 404);
				if (m[2] === undefined)
					return ok(res, {
						...meta,
						bill: id(k),
						n,
						licence: 'public-domain',
						address: addr(k, n, rest, `/bills/${id(k)}/v/${n}${rest}`),
						section: s
					});
				const from = Number(m[2]);
				const to = Number(m[3] ?? m[2]);
				return ok(res, {
					...meta,
					bill: id(k),
					n,
					section_id: s.id,
					section_path: s.path,
					from,
					to,
					licence: 'public-domain',
					address: addr(k, n, rest, `/bills/${id(k)}/v/${n}${rest}`),
					lines: s.lines.filter((l) => l.n >= from && l.n <= to)
				});
			}
			if ((m = /^\/compare\/(\d+)$/.exec(rest)) && k === 'stub' && n === 1 && m[1] === '2') {
				return ok(res, {
					...meta,
					bill: id(k),
					from_n: 1,
					to_n: 2,
					sections: [
						{ section_id: 's1', status: 'unchanged', changes: [] },
						{
							section_id: 's2',
							status: 'changed',
							changes: [
								{
									kind: 'modified',
									before_span: { section_id: 's2', from: 2, to: 2 },
									after_span: { section_id: 's2', from: 2, to: 2 },
									before: { text: 'Offices close at five.', content_class: 'official' },
									after: { text: 'Offices close at six.', content_class: 'official' }
								}
							]
						}
					]
				});
			}
		}
		return problem(res, 404);
	})
	.listen(Number(process.argv[2] ?? 4010), '0.0.0.0');
