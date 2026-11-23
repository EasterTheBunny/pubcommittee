#!/bin/sh
# M1-04 acceptance: bill XML parser, anchors, and example data.
# Run inside the dev container with the repository mounted at /src (cwd = repository root
# or any directory below it). Needs Go; installs xmllint (libxml2-utils) when missing.
# Environment: BILLXML_BULK_DIR (optional) a directory of extra govinfo XML to check for dropped text.
set -eu
cd "$(dirname "$0")/../../.."
[ -f go.mod ] || {
	echo "skip: Go module not present (go.mod); this script runs in the dev container with the repository mounted"
	exit 0
}
fail=0
check() { # name, command...
	name=$1; shift
	if "$@" >/tmp/m1-04-check.log 2>&1; then echo "pass: $name"; else echo "FAIL: $name"; sed 's/^/    /' /tmp/m1-04-check.log | head -20; fail=1; fi
}

# Every fixture file listed in fixtures/README.md exists with the recorded SHA-256, and nothing else is there.
hashes() {
	grep -o '^| `bills/[^`]*` | [0-9]* | .* | `[0-9a-f]\{64\}` |$' fixtures/README.md >/tmp/m1-04-listed.txt
	while IFS='|' read -r _ f _ _ h _; do
		f=$(echo "$f" | tr -d ' `'); h=$(echo "$h" | tr -d ' `')
		[ "$(sha256sum "fixtures/$f" | cut -d' ' -f1)" = "$h" ] || { echo "hash mismatch: $f"; return 1; }
	done </tmp/m1-04-listed.txt
	listed=$(grep -c '^| `bills/' fixtures/README.md)
	present=$(find fixtures/bills -type f | wc -l)
	[ "$listed" -eq "$present" ] || { echo "README lists $listed files, found $present"; return 1; }
}
check "fixtures/README.md hashes match every file" hashes

# Size caps: exit-bill versions at most 30 KB each; at most 2 MB per bill and 20 MB in all.
caps() {
	for b in us-119-hr-4313 us-119-hr-3062 us-119-hr-3427; do
		for v in ih rh eh rfs; do
			f=fixtures/bills/$b/v/$v/source.xml
			[ -f "$f" ] || { echo "missing exit-bill version $f"; return 1; }
			[ "$(wc -c <"$f")" -le 30720 ] || { echo "$f over 30 KB"; return 1; }
		done
	done
	for d in fixtures/bills/*/; do
		[ "$(du -sk "$d" | cut -f1)" -le 2048 ] || { echo "$d over 2 MB"; return 1; }
	done
	[ "$(du -sk fixtures | cut -f1)" -le 20480 ] || { echo "fixtures over 20 MB"; return 1; }
}
check "exit bills present, size caps hold" caps

# Parser tests: no dropped text, deterministic output, golden files, structure cases, fuzz seeds.
check "go test ./internal/billxml/..." go test -count=1 ./internal/billxml/...

# DTD validation of every original with the official bill.dtd (usgpo/bill-dtd).
dtd() {
	command -v xmllint >/dev/null 2>&1 || { apt-get update -qq && apt-get install -y -qq --no-install-recommends libxml2-utils; } >/dev/null 2>&1
	bad=0
	for f in fixtures/bills/*/v/*/source.xml; do
		if xmllint --noout --nonet --dtdvalid internal/billxml/testdata/dtd/bill.dtd "$f" 2>&1 | grep -q 'validity error'; then
			echo "DTD invalid: $f"; bad=1
		fi
	done
	return $bad
}
check "all fixture XML validates against bill.dtd (xmllint)" dtd

# Optional: larger local pull.
if [ -n "${BILLXML_BULK_DIR:-}" ]; then
	check "no dropped text across BILLXML_BULK_DIR" go test -count=1 -run TestBulkNoDroppedText ./internal/billxml/
fi
exit $fail
