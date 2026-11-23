#!/bin/sh
# M1-16 local gate: prepare the local data root the stack serves. Runs in the dev container (Go)
# with the repository mounted at /src; offline (fixtures only, no model calls).
#   - the 200-entry catalog sample, evaluated against the stored selection policy;
#   - two larger real bills of the parser corpus for the long-page check (M1-13 deferred it):
#     hr-3944 (reported, appropriations, about 160 KB) and hr-3115 (introduced, lists, about 180 KB).
#     The corpus has no BILLSTATUS file, so a minimal one is written from the exit bill's, in a
#     scratch fixtures directory (never in fixtures/);
#   - exports and the change feed (job-exports).
# Environment: DATA_DIR (required: the data root the read-api container serves).
set -eu
cd "$(dirname "$0")/../../.."
: "${DATA_DIR:?DATA_DIR is required}"
bin=$(mktemp -d)
for c in job-ingest job-exports; do go build -tags local -o "$bin/$c" "./cmd/$c"; done
export DATA_DIR
ji() { "$bin/job-ingest" --source fixtures "$@" 2>/dev/null; }

# Catalog sample: tracked bills without text in the fixtures source report a failed pull and keep their entry.
args=""
for f in fixtures/catalog/us-*.xml; do args="$args --bill $(basename "$f" .xml)"; done
# shellcheck disable=SC2086
ji --fixtures-dir fixtures $args >/dev/null || true
ji --fixtures-dir fixtures --evaluate >/dev/null
echo "catalog entries: $(find "$DATA_DIR/catalog" -name '*.json' ! -name '*.meta.json' | wc -l)"

# Larger real bills: scratch fixtures with a one-version BILLSTATUS.
extra=$(mktemp -d)
mkdir -p "$extra/bills" "$extra/catalog"
for spec in "3944 rh" "3115 ih"; do
	set -- $spec
	n=$1 code=$2
	id=us-119-hr-$n
	mkdir -p "$extra/bills/$id/v/$code"
	cp "fixtures/bills/$id/v/$code/source.xml" "$extra/bills/$id/v/$code/source.xml"
	awk -v n="$n" -v code="$code" '
		/<textVersions>/ { print; print "      <item><type>Reported in House</type><date>2025-07-02T04:00:00Z</date><formats><item><url>https://www.govinfo.gov/content/pkg/BILLS-119hr" n code "/xml/BILLS-119hr" n code ".xml</url></item></formats></item>"; skip=1; next }
		/<\/textVersions>/ { skip=0 }
		skip { next }
		{ gsub(/3062/, n); print }
	' fixtures/bills/us-119-hr-3062/status.xml >"$extra/bills/$id/status.xml"
	ji --fixtures-dir "$extra" --bill "$id" --track >/dev/null || echo "NOTE pull of $id reported a failure"
done
"$bin/job-exports" >/dev/null 2>&1
echo "prepared $DATA_DIR"
