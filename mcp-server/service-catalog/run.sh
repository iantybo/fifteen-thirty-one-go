#!/usr/bin/env bash
#
# run.sh builds and starts the service-catalog MCP server, then exposes it on
# the public internet via ngrok so remote MCP clients (e.g. CodeRabbit) can
# reach it.
#
# The /mcp endpoint requires a static API key. Set SERVICE_CATALOG_API_KEY
# before running; clients must send it as either:
#   Authorization: Bearer <key>
#   X-API-Key: <key>
#
# Usage:
#   SERVICE_CATALOG_API_KEY=your-secret ./run.sh
#
# Optional env:
#   PORT  local port to listen on (default 8765)
set -euo pipefail

cd "$(dirname "$0")"

: "${SERVICE_CATALOG_API_KEY:?SERVICE_CATALOG_API_KEY must be set (static API key for MCP auth)}"
PORT="${PORT:-8765}"

if ! command -v ngrok >/dev/null 2>&1; then
  echo "error: ngrok is not installed or not on PATH" >&2
  exit 1
fi

echo "building service-catalog..."
go build -o service-catalog .

echo "starting service-catalog on :${PORT} (MCP endpoint /mcp, API key required)..."
PORT="${PORT}" SERVICE_CATALOG_API_KEY="${SERVICE_CATALOG_API_KEY}" ./service-catalog &
SERVER_PID=$!

cleanup() {
  kill "${SERVER_PID}" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

# Wait for the local server to come up.
for _ in $(seq 1 25); do
  if curl -fsS "http://127.0.0.1:${PORT}/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done

echo "exposing via ngrok..."
echo "your public MCP URL will be printed below as <https-url>/mcp"
ngrok http "${PORT}"
