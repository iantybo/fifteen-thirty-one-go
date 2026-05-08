# Hearts Rust Expansion

The repository includes a new Rust crate at `backend/rust/hearts-engine` for future Hearts game support.

## Included now

- Round-level Hearts scoring.
- `Standard` and `Omnibus` variant support.
- Shoot-the-moon handling.
- Validation for invalid round input.
- Unit tests for core scoring behavior.

## Integration seam

Go contracts for this engine live at `backend/internal/game/hearts/contracts.go`.
This keeps the boundary stable regardless of whether integration happens through service calls or FFI.
