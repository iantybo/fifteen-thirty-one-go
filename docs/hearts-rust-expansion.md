# Hearts Rust Expansion (Foundation)

This repository now includes a Rust crate at `backend/rust/hearts-engine` for future Hearts game support.

## What is implemented

- Round-level score calculation for Hearts.
- Support for two variants:
  - `Standard`
  - `Omnibus` (jack of diamonds bonus)
- Shoot-the-moon detection and scoring behavior.
- Input validation for common invalid states (player count, total hearts, queen ownership).

## Why this exists

The backend is designed to support multiple card games over time. This crate provides a focused, testable domain module for Hearts scoring that can be integrated later through an API boundary (service call, FFI bridge, or worker process).

## Run tests

From the crate directory:

```bash
cargo test
```

## Next integration steps

1. Add a transport boundary from Go to Rust (likely a dedicated service process first, FFI later if needed).
2. Define shared request/response types in a stable format.
3. Add a game registry entry for Hearts once turn/state rules are implemented.
