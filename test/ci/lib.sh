# Shared helpers for the CI dependency checks (POSIX sh).

# forbid_in_list <extended-regex> reads import paths on stdin, prints any that
# match, and returns 1 when at least one does.
forbid_in_list() {
	found=$(grep -E "$1" || true)
	if [ -n "$found" ]; then
		printf '%s\n' "$found"
		return 1
	fi
	return 0
}
