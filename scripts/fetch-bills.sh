#!/bin/sh
# Fetch the example-data bills from govinfo once, byte for byte, into fixtures/bills/.
# One request at a time, pause between requests, descriptive user agent, no personal details.
# Usage: scripts/fetch-bills.sh [bill-id:versions ...]; defaults to the M1 exit bills.
# Prints "path sha256 url retrieved" lines for fixtures/README.md.
set -eu
UA="pubcommittee-fixture-fetch/0 (open civic project; one-time corpus pull)"
OUT="${FIXTURES_DIR:-fixtures/bills}"
BILLS="${*:-hr4313:ih,rh,eh,rfs hr3062:ih,rh,eh,rfs hr3427:ih,rh,eh,rfs}"
get() { # url dest
	mkdir -p "$(dirname "$2")"
	curl -fsS -L --retry 3 --retry-delay 5 -A "$UA" -o "$2" "$1"
	printf '%s %s %s %s\n' "$2" "$(sha256sum "$2" | cut -d' ' -f1)" "$1" "$(date -u +%Y-%m-%d)"
	sleep 2
}
for spec in $BILLS; do
	num=${spec%%:*}; vers=${spec#*:}
	get "https://www.govinfo.gov/bulkdata/BILLSTATUS/119/${num%%[0-9]*}/BILLSTATUS-119${num}.xml" "$OUT/us-119-${num%%[0-9]*}-${num#${num%%[0-9]*}}/status.xml"
	for v in $(echo "$vers" | tr ',' ' '); do
		get "https://www.govinfo.gov/content/pkg/BILLS-119${num}${v}/xml/BILLS-119${num}${v}.xml" "$OUT/us-119-${num%%[0-9]*}-${num#${num%%[0-9]*}}/v/$v/source.xml"
	done
done
