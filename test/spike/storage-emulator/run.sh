#!/usr/bin/env bash
# M1-00 spike: start each Cloud Storage stand-in from its digest-pinned image with explicit
# limits, measure container start to first successful request (test 16), then run the suite
# (tests 1-17, with -race for test 15) against it from a Go container on the same network.
# Needs only Docker and network access to pull images and modules. No cloud credentials.
set -euo pipefail

RUN_ID="${RUN_ID:-local}"
HERE="$(cd "$(dirname "$0")" && pwd)"
NET="pc-m1-00-spike-net"
GO_IMAGE="${GO_IMAGE:-golang:1.27}"
TESTBENCH=gcr.io/cloud-devrel-public-resources/storage-testbench@sha256:06729d407e8bba055798707dccfffd1f792ea818937954c56a776811116ede5c
FAKEGCS=fsouza/fake-gcs-server@sha256:797ce226d62f947c009dc40246b30cfb456b8473d8241407f9d6f2c04e4d69ef
LABELS=(--label "pubcommittee.run=$RUN_ID" --label pubcommittee.task=M1-00)

cleanup() {
  docker rm -f pc-m1-00-s-testbench pc-m1-00-s-fakegcs pc-m1-00-s-go >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
}
trap cleanup EXIT
cleanup
docker network create "${LABELS[@]}" "$NET" >/dev/null

now_ms() { date +%s%3N; }

# start <name> <image> <cpus> <memory> <host port> <container port> <probe path> [args...]
start() {
  local name=$1 image=$2 cpus=$3 mem=$4 hport=$5 cport=$6 path=$7
  shift 7
  local t0 t1
  t0=$(now_ms)
  docker run -d --name "$name" "${LABELS[@]}" --network "$NET" --cpus "$cpus" --memory "$mem" \
    --memory-swap "$mem" --pids-limit 256 -p "127.0.0.1:$hport:$cport" "$image" "$@" >/dev/null
  until curl -fsS -o /dev/null "http://127.0.0.1:$hport$path" 2>/dev/null; do sleep 0.1; done
  t1=$(now_ms)
  echo "startup $name: $((t1 - t0)) ms (container start to first 200 on $path)"
}

docker run -d --name pc-m1-00-s-go "${LABELS[@]}" --network "$NET" --cpus 1 --memory 1200m \
  --memory-swap 1200m --pids-limit 512 -v "$HERE":/spike -w /spike \
  -v pc-gomod:/go/pkg/mod -v pc-gocache:/root/.cache/go-build -e GOFLAGS=-buildvcs=false \
  "$GO_IMAGE" sleep infinity >/dev/null
docker exec pc-m1-00-s-go go test -c -race -o /tmp/spike.test .

run_suite() {
  local stand_in=$1 host=$2
  echo "== $stand_in ($host)"
  docker exec -e STORAGE_EMULATOR_HOST="$host" -e STAND_IN="$stand_in" pc-m1-00-s-go \
    /tmp/spike.test -test.v -test.count=1 | grep -E '^\s*--- |calls|DATA RACE|^(PASS|FAIL|ok)' || true
}

start pc-m1-00-s-testbench "$TESTBENCH" 0.5 768m 19000 9000 /
run_suite testbench pc-m1-00-s-testbench:9000
docker rm -f pc-m1-00-s-testbench >/dev/null

start pc-m1-00-s-fakegcs "$FAKEGCS" 0.5 512m 14443 4443 /storage/v1/b \
  -scheme http -port 4443 -public-host pc-m1-00-s-fakegcs:4443 \
  -external-url http://pc-m1-00-s-fakegcs:4443 -backend memory
run_suite fakegcs pc-m1-00-s-fakegcs:4443
