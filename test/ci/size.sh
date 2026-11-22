#!/bin/sh
# Binary size budget: build each binary as the image does and compare with
# test/ci/size-budget.txt (lines of "<binary> <max bytes>"). A binary with no
# budget line fails, so adding a service means setting its budget.
set -eu
dir=$(dirname "$0")
out=$(mktemp -d)
status=0
for pkg in $(go list ./cmd/...); do
	name=$(basename "$pkg")
	CGO_ENABLED=0 go build -tags memory -trimpath -ldflags="-s -w" -o "$out/$name" "$pkg"
	size=$(wc -c <"$out/$name")
	budget=$(awk -v n="$name" '$1 == n {print $2}' "$dir/size-budget.txt")
	if [ -z "$budget" ]; then
		echo "FAIL: $name ($size bytes) has no budget in size-budget.txt" >&2
		status=1
	elif [ "$size" -gt "$budget" ]; then
		echo "FAIL: $name is $size bytes, budget $budget" >&2
		status=1
	else
		echo "ok: $name $size bytes (budget $budget)"
	fi
done
rm -rf "$out"
exit $status
