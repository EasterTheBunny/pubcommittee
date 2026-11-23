#!/bin/sh
# M1-01 acceptance, runtime part: the placeholder read-api answers health,
# readiness, and metrics. Build-time criteria (tags, conformance, dependency
# rules, image properties) run in test/ci and with go test.
# Environment: READ_API_URL (default http://localhost:8080),
#              READ_API_METRICS_URL (default http://localhost:9464).
set -eu
api=${READ_API_URL:-http://localhost:8080}
metrics=${READ_API_METRICS_URL:-http://localhost:9464}

check() {
	code=$(curl -s -o /tmp/m101.out -w '%{http_code}' "$2")
	if [ "$code" != "$3" ]; then
		echo "FAIL: $1 $2 returned $code, want $3" >&2
		exit 1
	fi
	echo "ok: $1"
}

check healthz "$api/healthz" 200
check readyz "$api/readyz" 200
check metrics "$metrics/metrics" 200
grep -q 'http_server_request_duration' /tmp/m101.out || curl -s "$api/v1" >/dev/null
curl -s "$metrics/metrics" | grep -q 'pubcommittee_startup_duration' || {
	echo "FAIL: pubcommittee.startup.duration missing from /metrics" >&2
	exit 1
}
echo "ok: standard instruments exported"
check "metrics not on app port" "$api/metrics" 404
