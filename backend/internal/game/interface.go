package game

// Game is the pluggable interface for different game engines (cribbage first).
type Game interface {
	Type() string
}
