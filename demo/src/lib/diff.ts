export type DiffPart = { kind: 'eq' | 'del' | 'ins'; text: string };

/**
 * Word-level diff (longest common subsequence) for the citizen-edit-against-bill
 * side-by-side view (design/03). Whitespace is kept attached to each token.
 */
export function diffWords(before: string, after: string): DiffPart[] {
	const a = before.match(/\S+\s*/g) ?? [];
	const b = after.match(/\S+\s*/g) ?? [];
	const norm = (t: string) => t.trimEnd();

	const dp: number[][] = Array.from({ length: a.length + 1 }, () =>
		new Array(b.length + 1).fill(0)
	);
	for (let i = a.length - 1; i >= 0; i--) {
		for (let j = b.length - 1; j >= 0; j--) {
			dp[i][j] =
				norm(a[i]) === norm(b[j]) ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
		}
	}

	const parts: DiffPart[] = [];
	const push = (kind: DiffPart['kind'], text: string) => {
		const last = parts[parts.length - 1];
		if (last && last.kind === kind) last.text += text;
		else parts.push({ kind, text });
	};

	let i = 0;
	let j = 0;
	while (i < a.length && j < b.length) {
		if (norm(a[i]) === norm(b[j])) {
			push('eq', b[j]);
			i++;
			j++;
		} else if (dp[i + 1][j] >= dp[i][j + 1]) {
			push('del', a[i++]);
		} else {
			push('ins', b[j++]);
		}
	}
	while (i < a.length) push('del', a[i++]);
	while (j < b.length) push('ins', b[j++]);
	return parts;
}
