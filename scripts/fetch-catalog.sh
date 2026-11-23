#!/bin/sh
# Fetch the catalog sample (BILLSTATUS records only, no text) from govinfo once, byte for byte,
# into fixtures/catalog/. One request at a time, 2 s apart, descriptive user agent, no personal details.
# Usage: scripts/fetch-catalog.sh <urls-file>   (one BILLSTATUS URL per line)
# Prints "path bytes sha256 url retrieved" lines for fixtures/README.md.
set -eu
UA="pubcommittee-fixture-fetch/0 (open civic project; one-time corpus pull)"
OUT="${CATALOG_DIR:-fixtures/catalog}"
mkdir -p "$OUT"
while read -r url; do
	[ -n "$url" ] || continue
	name=$(basename "$url" .xml)            # BILLSTATUS-119hr123
	rest=${name#BILLSTATUS-}                # 119hr123
	congress=${rest%%[a-z]*}
	tail=${rest#"$congress"}                # hr123
	type=${tail%%[0-9]*}
	num=${tail#"$type"}
	dest="$OUT/us-$congress-$type-$num.xml"
	[ -f "$dest" ] || { curl -fsS -L --retry 3 --retry-delay 5 -A "$UA" -o "$dest" "$url"; sleep 2; }
	printf '%s %s %s %s %s\n' "$dest" "$(wc -c <"$dest")" "$(sha256sum "$dest" | cut -d' ' -f1)" "$url" "$(date -u +%Y-%m-%d)"
done <"$1"
