#!/bin/bash

# Test runner for trading card tests
# This script runs the trading card model and handler tests
# Requires: Go 1.25+, CGO_ENABLED=1, and gcc/clang compiler

set -e

echo "Running Trading Card Tests..."
echo "============================="
echo ""

# Check for CGO requirements
if ! command -v gcc &> /dev/null && ! command -v clang &> /dev/null; then
    echo "Error: C compiler (gcc or clang) is required for SQLite tests"
    echo "Please install build-essential (Debian/Ubuntu) or build-base (Alpine)"
    exit 1
fi

cd "$(dirname "$0")"

echo "1. Running model tests (internal/models)..."
CGO_ENABLED=1 go test -v -race ./internal/models/trading_card_test.go

echo ""
echo "2. Running handler tests (internal/handlers)..."
CGO_ENABLED=1 go test -v -race ./internal/handlers/trading_cards_test.go

echo ""
echo "3. Running all trading card tests together..."
CGO_ENABLED=1 go test -v -race -run ".*TradingCard.*|.*Card.*" ./internal/models/ ./internal/handlers/

echo ""
echo "All tests completed successfully!"