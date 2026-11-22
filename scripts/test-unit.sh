#!/usr/bin/env bash
set -euo pipefail
mkdir -p artifacts

unformatted=$(gofmt -l ./cmd ./internal)
if [ -n "$unformatted" ]; then
  echo "Go formatting issues in: $unformatted" >&2
  exit 1
fi

timeout 60s go test -v -coverprofile=artifacts/coverage-go.out ./internal/room/... | tee artifacts/unit-go.log
go tool cover -func=artifacts/coverage-go.out | tee artifacts/coverage-go.txt

timeout 60s node --test --experimental-test-coverage web/crypto.test.mjs | tee artifacts/unit-js.log
