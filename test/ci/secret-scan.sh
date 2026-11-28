#!/bin/sh
# Secret scan of the public repository (M1-15): the tree must hold no secret
# and no value that identifies a real deployment, and the scan must fail on a
# planted fake secret and on a planted project-id-shaped value. The plants are
# built at run time in a temporary directory, so this file matches no rule.
# Runs where `gitleaks` is on PATH, for example:
#   docker run --rm --network none -v "$PWD":/src:ro -w /src \
#     --entrypoint sh zricethezav/gitleaks:v8.24.3 test/ci/secret-scan.sh
# Scan a CI checkout or a worktree: a checkout that holds the private folders
# (deploy/, contacts/, .local-data/, .env*) fails, by design.
# Environment: GITLEAKS (default "gitleaks"), REPO_DIR (default: this repository).
set -eu

gitleaks=${GITLEAKS:-gitleaks}
repo=${REPO_DIR:-$(cd "$(dirname "$0")/../.." && pwd)}
config="$repo/.gitleaks.toml"

fail() {
	echo "FAIL: $1" >&2
	exit 1
}

scan() {
	# $1: directory; prints the report, returns gitleaks' exit code.
	$gitleaks dir --config "$config" --no-banner --redact --log-level error \
		--report-format csv --report-path - "$1"
}

# The repository itself is clean.
scan "$repo" >/dev/null || fail "secret scan found leaks in the repository (run without >/dev/null to see them)"
echo "ok: repository clean"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

random() {
	# $1: length; lower-case letters and digits.
	tr -dc 'a-z0-9' </dev/urandom | head -c "$1"
}

expect_leak() {
	# $1: rule id; $2: file content.
	dir="$tmp/$1"
	mkdir -p "$dir"
	printf '%s\n' "$2" >"$dir/planted.txt"
	if out=$(scan "$dir"); then
		fail "planted $1 value was not found"
	fi
	echo "$out" | grep -q "^$1," || fail "planted $1 value failed for another reason: $out"
	echo "ok: planted $1 value is found"
}

# A fake GitHub token: right shape, never issued.
expect_leak github-pat "token = \"gh""p_$(random 36)\""
# A project-id-shaped value: a name, an environment, a suffix with a digit.
expect_leak gcp-project-id-env "bucket = \"acme-civic-prod-$(random 3)7-objects\""
# A deployed Cloud Run URL shape.
expect_leak cloud-run-url "url = \"https://read-api-$(random 10)-uc.a.run"".app\""
