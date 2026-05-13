# Hearts Rust Expansion (Foundation)

> **Status: experimental / not yet integrated.**
> Hearts is not playable on the platform today. This crate is a scoring
> primitive spike; no Go service calls into it yet.

This repository includes a Rust crate at `backend/rust/hearts-engine` that
provides Hearts round-scoring logic for future multi-game expansion.

## What is implemented

- Round-level score calculation for Hearts.
- Support for two variants:
  - `Standard`
  - `Omnibus` (jack of diamonds bonus: -10 pts for capturing the J♦)
- Shoot-the-moon detection and scoring: capturing all 13 hearts and Q♠ gives
  0 pts to the shooter and 26 pts to every other player.
- Input validation for common invalid states (player count 3–6, total hearts
  must equal 13, exactly one Q♠ capture per round).

## What is NOT yet implemented

- Full turn/state management (dealing, trick-taking, passing phase).
- Go↔Rust transport boundary (no HTTP service or FFI bridge exists yet).
- Game registry entry for Hearts.
- Shoot-the-moon opt-to-add (adding 26 to others) is not implemented; only
  the subtract-from-shooter variant is supported.

## Why this exists

The backend is designed to support multiple card games over time. This crate
provides a focused, testable domain module for Hearts scoring that can be
integrated later through a stable API boundary.

## Run tests

From the crate directory:

```bash
cargo test
```

## Integration plan

### Phase 1 — Standalone service (current target)

- Wrap the crate in a small HTTP service (e.g. `axum` or `actix-web`).
- Go calls the service over localhost HTTP (no shared memory, no FFI).
- Request/response types: JSON with fields matching `RoundInput`/`RoundResult`.
- **Success criteria**: Go leaderboard integration test POSTs a round and
  receives correct scores; all existing Go tests still pass.

### Phase 2 — FFI (if latency requires it)

- Change `crate-type` to `["cdylib"]` in `Cargo.toml`.
- Add a `#[no_mangle] extern "C"` surface with C-safe types (no Rust enums
  or `Vec` across the boundary).
- Use `cgo` on the Go side with the generated C header.
- **Success criteria**: round-trip latency < 1 ms; no memory leaks under
  `valgrind`/`cargo-valgrind`; CI runs `cargo test` and Go FFI integration
  test in the same pipeline job.

### Phase 3 — Game registry

- Implement turn/state rules (trick-taking, passing phase, end-of-hand).
- Add Hearts to the game registry so lobbies can select it.
- **Success criteria**: a full 4-player Hearts hand completes without errors;
  final scores match hand-calculated reference fixture.
