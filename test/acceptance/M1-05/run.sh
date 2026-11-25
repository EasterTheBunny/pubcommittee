#!/bin/sh
# M1-05 acceptance: manual bill pull and storage (job-ingest).
# Run inside the dev container with the repository mounted at /src (cwd = repository root
# or any directory below it). Needs Go and sha256sum.
# Environment: DATA_DIR (optional) a data root to pull into; a fresh temporary one is used when unset.
#              LIVE=1 also pulls the three exit bills from govinfo (15 requests, 2 s apart) into a
#              second temporary root and compares every original with fixtures/ (stage L2).
set -eu
cd "$(dirname "$0")/../../.."
[ -f go.mod ] || {
	echo "skip: Go module not present (go.mod); this script runs in the dev container with the repository mounted"
	exit 0
}
fail=0
check() { # name, command...
	name=$1; shift
	if "$@" >/tmp/m1-05-check.log 2>&1; then echo "pass: $name"; else echo "FAIL: $name"; sed 's/^/    /' /tmp/m1-05-check.log | head -30; fail=1; fi
}
bills="us-119-hr-4313 us-119-hr-3062 us-119-hr-3427"
bin=$(mktemp -d)/job-ingest
go build -tags local -o "$bin" ./cmd/job-ingest
root=${DATA_DIR:-$(mktemp -d)}
export DATA_DIR=$root

path() { echo "$1" | sed 's#^\([a-z]*\)-\([0-9]*\)-\([a-z]*\)-\([0-9]*\)$#\1/\2/\3/\4#'; }
tree_sum() { (cd "$root" && find . -type f ! -path './.locks/*' | sort | xargs sha256sum | sha256sum); }

# Unit, end-to-end (memory + fixturesrc), conformance, adapter tests.
check "go tests: ingest, billstatus, adapters (memory)" go test -tags memory -count=1 ./internal/ingest/... ./internal/billstatus/... ./internal/adapters/fixturesrc/... ./internal/adapters/govinfo/... ./internal/adapters/memory/... ./internal/data/...
check "go tests: fsobject provenance (local)" go test -tags local -count=1 ./internal/adapters/fsobject/...

# Pull with fixturesrc on the local target: every original and parsed version.
pull() { "$bin" --source fixtures --fixtures-dir fixtures "$@"; }
pulled() { for b in $bills; do pull --bill "$b" --track; done; }
check "pull exit bills with fixturesrc" pulled
stored() {
	for b in $bills; do
		p=$(path "$b")
		for v in ih rh eh rfs; do
			d=$root/bills/$p/v/$v
			[ -f "$d/source.xml" ] && [ -f "$d/text.json" ] || { echo "missing $d"; return 1; }
			cmp -s "$d/source.xml" "fixtures/bills/$b/v/$v/source.xml" || { echo "$d/source.xml differs from the fixture"; return 1; }
			grep -q '"parser_version": *"billxml/1"' "$d/text.json" || { echo "no parser version in $d/text.json"; return 1; }
			grep -q '"source_url"' "$d/source.xml.meta.json" && grep -q '"retrieved_at"' "$d/source.xml.meta.json" && grep -q '"sha256"' "$d/source.xml.meta.json" || { echo "provenance missing in $d/source.xml.meta.json"; return 1; }
		done
	done
}
check "every original (byte for byte, with provenance) and parsed version stored" stored

before=$(tree_sum)
check "second pull stores nothing" sh -c "for b in $bills; do '$bin' --source fixtures --fixtures-dir fixtures --bill \$b | grep -q '\"stored\": *\[\]' || exit 1; done"
[ "$(tree_sum)" = "$before" ] && echo "pass: second pull changes no file" || { echo "FAIL: second pull changed files"; fail=1; }

# Untracked bill (a catalog-only record): catalog entry only, no text.
check "untracked bill stores no text" sh -c "'$bin' --source fixtures --fixtures-dir fixtures --bill us-119-hr-84 >/dev/null && [ ! -e '$root/bills/us/119/hr/84' ]"
check "untracked bill has a catalog entry, not tracked" sh -c "grep -q '\"tracked\":false' '$root/catalog/us%2F119%2Fhr%2F84.json'"

# Dry run writes nothing.
before=$(tree_sum)
check "dry run reports" sh -c "'$bin' --source fixtures --fixtures-dir fixtures --bill us-119-hr-3115 --track --dry-run | grep -q '\"dry_run\": *true'"
[ "$(tree_sum)" = "$before" ] && echo "pass: dry run writes nothing" || { echo "FAIL: dry run wrote files"; fail=1; }

# Forced SHA-256 mismatch is reported and does not overwrite (in-process test).
check "forced SHA-256 mismatch reported, nothing overwritten" go test -tags memory -count=1 -run TestHashMismatchReportedNotApplied ./internal/ingest/...

# Live pull of the exit bills from govinfo (stage L2; politeness: one request at a time, 2 s apart).
if [ "${LIVE:-0}" = 1 ]; then
	live=$(mktemp -d)
	livepull() { for b in $bills; do DATA_DIR=$live "$bin" --source govinfo --bill "$b" --track; done; }
	check "live pull of exit bills from govinfo" livepull
	same() {
		for b in $bills; do
			p=$(path "$b")
			for v in ih rh eh rfs; do
				[ -f "$live/bills/$p/v/$v/text.json" ] || { echo "missing live $b $v"; return 1; }
				cmp -s "$live/bills/$p/v/$v/source.xml" "fixtures/bills/$b/v/$v/source.xml" || echo "note: live $b $v differs from the committed fixture" >>/tmp/m1-05-notes
			done
		done
	}
	: >/tmp/m1-05-notes
	check "live: every official version stored and parsed" same
	cat /tmp/m1-05-notes
fi
[ "$fail" -eq 0 ] && echo "M1-05 acceptance: all checks passed" || { echo "M1-05 acceptance: FAILED"; exit 1; }
