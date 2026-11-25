#!/bin/sh
# Test of scripts/law-copy.sh against a fake gcloud backed by local
# directories (gs://B/p is $root/B/p). Run: sh scripts/law-copy_test.sh
set -eu
here=$(cd "$(dirname "$0")" && pwd)
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
mkdir -p "$root/bin"
cat >"$root/bin/gcloud" <<'FAKE'
#!/bin/sh
# Fake "gcloud storage" for cat, ls --recursive, rsync and cp.
set -eu
p() { printf '%s' "$GCS_ROOT/${1#gs://}"; }
[ "$1" = storage ] || exit 9
shift; cmd=$1; shift
case $cmd in
cat) cat "$(p "$1")" ;;
ls) shift; d=$(p "${1%/\*\*}"); [ -d "$d" ] && find "$d" -type f | sed "s#^$GCS_ROOT/#gs://#" ;;
rsync)
	excl=; while [ $# -gt 2 ]; do case $1 in --exclude=*) excl=${1#--exclude=} ;; esac; shift; done
	s=$(p "$1"); d=$(p "$2")
	find "$s" -type f | while read -r f; do
		rel=${f#"$s"/}
		if [ -n "$excl" ] && printf '%s' "$rel" | grep -Eq "$excl"; then continue; fi
		[ -e "$d/$rel" ] && continue
		mkdir -p "$(dirname "$d/$rel")"; cp "$f" "$d/$rel"
	done ;;
cp)
	nc=0; [ "$1" = --no-clobber ] && { nc=1; shift; }
	d=$(p "$2"); [ $nc = 1 ] && [ -e "$d" ] && exit 0
	mkdir -p "$(dirname "$d")"; cp "$(p "$1")" "$d" ;;
*) exit 9 ;;
esac
FAKE
chmod +x "$root/bin/gcloud"
export PATH="$root/bin:$PATH" GCS_ROOT="$root"
put() { mkdir -p "$(dirname "$root/$1")"; printf '%s' "$2" >"$root/$1"; }
fail() { echo "FAIL: $*" >&2; exit 1; }

# Source: release 119-142 with two sections.
put src/law/usc/current.json '{"id":"119-142"}'
put src/law/usc/119-142/release.json '{"id":"119-142","public_law":"119-142","date":"2026-09-30T00:00:00Z"}'
put src/law/usc/119-142/t42/1395cc-7.json '{"s":1}'
put src/law/usc/119-142/t3/101.json '{"s":2}'

# 1. Empty destination: everything is copied and the pointer is set.
sh "$here/law-copy.sh" src dst >/dev/null
for f in current.json 119-142/release.json 119-142/t42/1395cc-7.json 119-142/t3/101.json; do
	cmp -s "$root/src/law/usc/$f" "$root/dst/law/usc/$f" || fail "copy of $f"
done
echo "pass: empty destination"

# 2. A repeat run changes nothing and never overwrites a stored section.
put dst/law/usc/119-142/t3/101.json '{"s":"kept"}'
sh "$here/law-copy.sh" src dst >/dev/null
grep -q kept "$root/dst/law/usc/119-142/t3/101.json" || fail "a stored section was overwritten"
echo "pass: repeat run, no overwrite"

# 3. A destination with a newer release keeps its pointer.
put new/law/usc/current.json '{"id":"119-200"}'
put new/law/usc/119-200/release.json '{"id":"119-200","date":"2027-03-31T00:00:00Z"}'
sh "$here/law-copy.sh" src new | grep -q "pointer unchanged" || fail "newer destination pointer moved"
grep -q 119-200 "$root/new/law/usc/current.json" || fail "newer destination pointer changed"
[ -e "$root/new/law/usc/119-142/release.json" ] || fail "older release not stored beside the newer one"
echo "pass: newer destination pointer kept"

# 4. A destination with an older release gets the newer pointer.
put old/law/usc/current.json '{"id":"119-100"}'
put old/law/usc/119-100/release.json '{"id":"119-100","date":"2026-01-31T00:00:00Z"}'
sh "$here/law-copy.sh" src old >/dev/null
grep -q 119-142 "$root/old/law/usc/current.json" || fail "older destination pointer not moved"
echo "pass: older destination pointer moved"

# 5. Refusals: same bucket, no source release.
sh "$here/law-copy.sh" src src >/dev/null 2>&1 && fail "same bucket accepted"
sh "$here/law-copy.sh" none dst >/dev/null 2>&1 && fail "missing source accepted"
echo "pass: refusals"
