#!/bin/sh
# Rebuilds test/accuracy/operator-sample-2.{md,json} from stored outputs and
# grades. No model call. Run from the repository root inside the dev
# container; DATA_DIR is a COPY of the local data directory (the tool takes
# its lock files there, so never point it at the original).
#   docker exec -e DATA_DIR=/tmp/data <container> sh test/accuracy/rebuild-operator-sample-2.sh
# Mix: 10 grader-contradicted, 10 validator-rejected (unverified outputs),
# 10 grader-supported controls; statements of the first sample are skipped.
set -eu
: "${DATA_DIR:?DATA_DIR is required}"
go run -tags local ./cmd/accuracy-report \
  --hard-sample operator-sample-2 \
  --exclude-sample test/accuracy/operator-sample.json \
  --out test/accuracy --fixtures-dir fixtures
