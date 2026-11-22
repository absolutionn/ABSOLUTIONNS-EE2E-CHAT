#!/usr/bin/env bash
set -euo pipefail
mkdir -p artifacts

go build -o artifacts/server-bin ./cmd/server
ADDR=127.0.0.1:18080 WEB_DIR=./web ./artifacts/server-bin > artifacts/e2e-server.log 2>&1 &
SRV_PID=$!
trap 'kill "$SRV_PID" 2>/dev/null || true; rm -f artifacts/server-bin' EXIT

for _ in $(seq 1 30); do
  if curl -sf http://127.0.0.1:18080/health >/dev/null; then break; fi
  sleep 0.2
done
curl -sf http://127.0.0.1:18080/health >/dev/null

timeout 60s env BASE_URL=http://127.0.0.1:18080 node tests/e2e.mjs | tee artifacts/e2e.log
