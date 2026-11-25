#!/bin/sh
# Copy the current US Code release point from one environment's object bucket
# to another's, so the destination need not download it again from the Law
# Revision Counsel. Run by the operator with their own gcloud login, which must
# read the source bucket and write the destination bucket. No job ever reads
# another environment's bucket.
#
# Usage: scripts/law-copy.sh SRC_BUCKET DST_BUCKET
#   SRC_BUCKET, DST_BUCKET: bucket names (the stack output objects_bucket), no gs:// prefix.
#
# What it copies, under law/usc/ (internal/data/citations.go):
#   <release>/t<title>/<section>.json   every section, never overwriting one already there
#   <release>/release.json              last, so a partial copy never looks like a stored release
#   current.json                        only if the destination has none or an older release
# Server-side copies keep each object's metadata (immutable flag, SHA-256, cache control).
# Then run job-ingest --analyze in the destination to build its citation sets.
set -eu

[ $# -eq 2 ] || { echo "usage: $0 SRC_BUCKET DST_BUCKET" >&2; exit 2; }
src="gs://$1/law/usc"
dst="gs://$2/law/usc"
[ "$1" != "$2" ] || { echo "source and destination are the same bucket" >&2; exit 2; }

# field FILE-CONTENT NAME: the string value of a top-level field in compact JSON.
field() { printf '%s' "$1" | sed -n "s/.*\"$2\":\"\([^\"]*\)\".*/\1/p"; }

cur=$(gcloud storage cat "$src/current.json") || { echo "no current release in $src" >&2; exit 1; }
rel=$(field "$cur" id)
case "$rel" in "" | */* | *.*) echo "bad release id in $src/current.json" >&2; exit 1 ;; esac
release=$(gcloud storage cat "$src/$rel/release.json") || { echo "release $rel has no release.json in $src" >&2; exit 1; }
date=$(field "$release" date)
[ -n "$date" ] || { echo "release $rel has no date" >&2; exit 1; }
echo "copying release $rel (current through $date)"

gcloud storage rsync --recursive --no-clobber --exclude='release\.json$' "$src/$rel" "$dst/$rel"

# count BASE: the section objects of the release under BASE (release.json excluded).
count() { gcloud storage ls --recursive "$1/$rel/**" 2>/dev/null | grep '\.json$' | grep -vc '/release\.json$' || true; }
ns=$(count "$src")
nd=$(count "$dst")
[ "$ns" = "$nd" ] || { echo "copied $nd of $ns objects; run again to finish" >&2; exit 1; }

gcloud storage cp --no-clobber "$src/$rel/release.json" "$dst/$rel/release.json"

if dcur=$(gcloud storage cat "$dst/current.json" 2>/dev/null); then
	drel=$(field "$dcur" id)
	ddate=$(field "$(gcloud storage cat "$dst/$drel/release.json")" date)
	# RFC 3339 UTC times compare as strings.
	if [ "$drel" = "$rel" ] || [ "$(printf '%s\n%s\n' "$ddate" "$date" | sort | tail -n 1)" = "$ddate" ]; then
		echo "destination current release $drel is the same or newer: pointer unchanged"
		echo "done: $ns objects; next: job-ingest --analyze in the destination"
		exit 0
	fi
fi
gcloud storage cp "$src/current.json" "$dst/current.json"
echo "done: $ns objects, current release $rel; next: job-ingest --analyze in the destination"
