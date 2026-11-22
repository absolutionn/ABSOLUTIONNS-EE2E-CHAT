# E2EE Webchat

## Environment & Versions
- OS / Shell: EndeavourOS Linux (`x86_64`), Bash
- Go: `1.27.4` (`net/http`, `github.com/coder/websocket`)
- Node.js: `v26.13.1` (ES Modules, built-in Web Crypto API & `node:test`)
- k6: `v2.4.0`
- Browser E2E: Playwright + Chromium (`devDependency` only)

## Setup (Clean State)

    go mod download
    npm install
    npx playwright install chromium
    chmod +x scripts/*.sh

## Running Tests
- Unit (Go + `gofmt` + JS crypto + coverage): `./scripts/test-unit.sh`
- Integration + race detector (`-race`): `./scripts/test-integration.sh`
- End-to-End (Playwright Chromium): `./scripts/test-e2e.sh`
- Load (k6, local `127.0.0.1:18080`, 1 smoke + 3 main runs): `./scripts/test-load.sh`

All scripts are non-interactive, exit with `0` on pass (non-zero on fail), save machine-readable outputs to `artifacts/`, and automatically stop temporary server processes via `trap`.

## Cleanup
Temporary server binaries and background processes are cleaned up automatically by the scripts. To wipe generated outputs: `rm -rf artifacts/*`.
