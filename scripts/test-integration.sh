#!/usr/bin/env bash
set -euo pipefail
mkdir -p artifacts

timeout 60s go test -v -race -json ./internal/server/... | tee artifacts/integration.json
