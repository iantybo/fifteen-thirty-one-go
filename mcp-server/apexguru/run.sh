#!/usr/bin/env bash
#
# run.sh builds and starts the apexguru MCP server, then exposes it on
# the public internet via ngrok so remote MCP clients (e.g. CodeRabbit) can
# reach it.
#
# The /mcp endpoint requires a static API key. Set APEXGURU_MCP_API_KEY
# before running; clients must send it as either:
#   Authorization: Bearer <key>
#   X-API-Key: <key>
#
# Usage:
#   APEXGURU_MCP_API_KEY=your-secret ./run.sh
#
# Optional env:
#   PORT  local port to listen on (default 8766)
set -euo pipefail

cd "$(dirname "$0")"

: "${APEXGURU_MCP_API_KEY:?APEXGURU_MCP_API_KEY must be set (static API key for MCP auth)}"
PORT="${PORT:-8766}"

if ! command -v ngrok >/dev/null 2>&1; then
  echo "error: ngrok is not installed or not on PATH" >&2
  exit 1
fi

echo "building apexguru..."
go build -o apexguru .

echo "starting apexguru on :${PORT} (MCP endpoint /mcp, API key required)..."
PORT="${PORT}" APEXGURU_MCP_API_KEY="${APEXGURU_MCP_API_KEY}" ./apexguru &
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
