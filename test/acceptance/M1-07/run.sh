#!/bin/sh
# M1-07 acceptance (local phase): US Code release point, citation extraction and resolution,
# unresolved list, freshness flags, measured resolution rate, citation routes.
# Run inside the dev container with the repository mounted at /src (cwd = repository root or below).
# LAW_DIR (optional): directory with the downloaded xml_usc<TT>@119-142.zip files; when set, the
# resolution rate is measured over the whole labelled sample, otherwise over the three exit bills
# against the committed fixtures/uscode sections.
set -eu
cd "$(dirname "$0")/../../.."
fail=0
check() { # name, command...
	name=$1; shift
	if "$@" >/tmp/m1-07-check.log 2>&1; then echo "pass: $name"; else echo "FAIL: $name"; sed 's/^/    /' /tmp/m1-07-check.log | head -30; fail=1; fi
}

# USLM title parser and the release point step: the weekly run with no new release changes nothing.
check "USLM parser" go test -count=1 ./internal/uslm/
check "release point step is a no-op when nothing is new" go test -count=1 -tags memory ./internal/lawingest/
check "LawSource adapter (httptest, politeness)" go test -count=1 ./internal/adapters/uscode/
# Every citation of the exit bills has a state; the unresolved and ambiguous ones are listed with reasons.
check "exit bills: every citation has a state" go test -count=1 -tags memory -run 'EveryCitationHasState|BuildExitBills' ./internal/citations/
check "exit bills: unresolved and ambiguous are listed" go test -count=1 -tags memory -run UnresolvedListed ./internal/citations/
check "text patterns and Act-section pairing" go test -count=1 -tags memory -run ExtractTextPatterns ./internal/citations/
# Freshness flags from laws enacted after the release point; the step is idempotent.
check "freshness flags and idempotent step" go test -count=1 -tags memory -run StepIdempotentAndFreshness ./internal/citations/
# Storage conformance: write-once on memory and the local object store.
check "law and citation storage (memory)" go test -count=1 -tags memory -run 'Law|Citations' ./internal/adapters/memory/
check "law and citation storage (fsobject)" go test -count=1 -tags memory -run 'Law|Citations' ./internal/adapters/fsobject/
# The read-api routes serve the stored set, 404 otherwise.
check "citation routes" go test -count=1 -tags memory -run Citations ./internal/readapi/
# Resolution rate on the labelled sample.
if [ -n "${LAW_DIR:-}" ]; then
	check "resolution rate measured (full sample)" sh -c "go run -tags memory ./scripts/measure-citations -law '$LAW_DIR' -labels test/acceptance/M1-07/labels.json | tee /tmp/m1-07-report.md | grep -q 'Precision of'"
else
	check "resolution rate measured (exit bills, fixtures)" sh -c "go run -tags memory ./scripts/measure-citations -fixtures fixtures/uscode -only us-119-hr-4313,us-119-hr-3062,us-119-hr-3427 -labels test/acceptance/M1-07/labels.json | tee /tmp/m1-07-report.md | grep -q 'Precision of'"
fi

# job-ingest wiring (part 2): --law stores the release point from fixtures/uscode with no network; a repeat run
# changes nothing; a pull of the exit bills stores a citation set for every version; unresolved cites are listed.
check "job-ingest: law mode, citations step, offline" sh -c '
	set -eu
	d=$(mktemp -d); trap "rm -rf $d" EXIT
	export DATA_DIR=$d
	go build -tags local -o "$d.ji" ./cmd/job-ingest
	"$d.ji" --law 2>/dev/null | grep -q "\"new_release\": true"
	snap=$(cd "$d" && find . -type f | sort | xargs sha256sum | sha256sum)
	"$d.ji" --law 2>/dev/null | grep -q "\"new_release\": false"
	[ "$snap" = "$(cd "$d" && find . -type f | sort | xargs sha256sum | sha256sum)" ]
	"$d.ji" --track --bill us-119-hr-4313,us-119-hr-3062,us-119-hr-3427 2>/dev/null >"$d.out"
	[ "$(grep -c "\"citation_sets\": 4" "$d.out")" = 3 ]
	[ "$(find "$d/bills" -path "*/citations/*" -name "*@119-142.json" | wc -l)" = 12 ]
	snap=$(cd "$d" && find . -type f | sort | xargs sha256sum | sha256sum)
	"$d.ji" --track --bill us-119-hr-4313,us-119-hr-3062,us-119-hr-3427 >/dev/null 2>&1
	[ "$snap" = "$(cd "$d" && find . -type f | sort | xargs sha256sum | sha256sum)" ]
	rm -f "$d.ji" "$d.out"
'
check "job-ingest: pipeline steps and fixture law source" go test -count=1 -tags memory -run 'Analysis|LawSource' ./internal/ingest/ ./internal/adapters/fixturesrc/
exit $fail
