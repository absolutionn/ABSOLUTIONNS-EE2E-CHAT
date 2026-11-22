#!/usr/bin/env bash
set -euo pipefail
mkdir -p artifacts

command -v k6 >/dev/null || { echo "k6 not installed" >&2; exit 1; }
go build -o artifacts/server-bin ./cmd/server
SRV_PID=""
cleanup() {
  [ -n "$SRV_PID" ] && kill "$SRV_PID" 2>/dev/null || true
  rm -f artifacts/server-bin
}
trap cleanup EXIT

start_fresh_server() {
  [ -n "$SRV_PID" ] && kill "$SRV_PID" 2>/dev/null || true
  sleep 0.3
  ADDR=127.0.0.1:18080 WEB_DIR=./web ./artifacts/server-bin > artifacts/load-server.log 2>&1 &
  SRV_PID=$!
  for _ in $(seq 1 30); do
    if curl -sf http://127.0.0.1:18080/health >/dev/null; then return 0; fi
    sleep 0.2
  done
  echo "Server failed to start" >&2
  exit 1
}

start_fresh_server
timeout 30s k6 run -e MODE=smoke -e BASE_URL=http://127.0.0.1:18080 \
  --summary-export=artifacts/load-smoke.json tests/load.js | tee artifacts/load-smoke.log

for run in 1 2 3; do
  start_fresh_server
  timeout 60s k6 run -e MODE=main -e BASE_URL=http://127.0.0.1:18080 \
    --summary-export="artifacts/load-run-${run}.json" tests/load.js | tee "artifacts/load-run-${run}.log"
done
