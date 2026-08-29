// Package cribbage holds the small domain model shared by every microservice
// in the fleet. The services compose pieces of cribbage scoring: each owns a
// narrow responsibility (fifteens, pairs, runs, flushes, nobs) and calls peers
// over HTTP to assemble a full hand score. The types here are the common
// vocabulary they exchange.
package cribbage

import (
	"errors"
	"fmt"
	"strings"
)

// Suit is one of the four card suits.
type Suit string

// The four suits, encoded as single upper-case letters.
const (
	Spades   Suit = "S"
	Hearts   Suit = "H"
	Diamonds Suit = "D"
	Clubs    Suit = "C"
)

// Card is a single playing card. Rank is 1 (Ace) .. 13 (King). The wire
// encoding is rank+suit where ten is spelled "10" (e.g. "AS", "10H", "KD"),
// matching the cross-repo contract in fifteen-thirty-one-go-utils.
type Card struct {
	Rank int  `json:"rank"`
	Suit Suit `json:"suit"`
}

// PipValue returns the cribbage counting value of the card: Ace = 1, face
// cards (Jack, Queen, King) = 10, everything else its rank.
func (c Card) PipValue() int {
	if c.Rank >= 10 {
		return 10
	}
	return c.Rank
}

// Code renders the card in canonical rank+suit form (e.g. "10H", "KD").
func (c Card) Code() string {
	var rank string
	switch c.Rank {
	case 1:
		rank = "A"
	case 11:
		rank = "J"
	case 12:
		rank = "Q"
	case 13:
		rank = "K"
	default:
		rank = fmt.Sprintf("%d", c.Rank)
	}
	return rank + string(c.Suit)
}

// ParseCard parses a canonical card code (e.g. "AS", "10H", "KD"). Ten MUST be
// spelled "10" — "T" is rejected to stay consistent with the scoring utils.
func ParseCard(code string) (Card, error) {
	code = strings.TrimSpace(strings.ToUpper(code))
	if len(code) < 2 {
		return Card{}, fmt.Errorf("cribbage: card code %q too short", code)
	}
	suit := Suit(code[len(code)-1:])
	switch suit {
	case Spades, Hearts, Diamonds, Clubs:
	default:
		return Card{}, fmt.Errorf("cribbage: card code %q has invalid suit", code)
	}
	rankPart := code[:len(code)-1]
	var rank int
	switch rankPart {
	case "A":
		rank = 1
	case "J":
		rank = 11
	case "Q":
		rank = 12
	case "K":
		rank = 13
	case "10":
		rank = 10
	case "T":
		return Card{}, fmt.Errorf("cribbage: ten must be encoded as %q, not %q", "10", "T")
	default:
		if _, err := fmt.Sscanf(rankPart, "%d", &rank); err != nil || rank < 2 || rank > 9 {
			return Card{}, fmt.Errorf("cribbage: card code %q has invalid rank %q", code, rankPart)
		}
	}
	return Card{Rank: rank, Suit: suit}, nil
}

// Hand is the four cards in a player's hand plus the cut (starter) card.
type Hand struct {
	Cards   []Card `json:"cards"`
	Starter Card   `json:"starter"`
	IsCrib  bool   `json:"is_crib"`
}

// ErrInvalidHand indicates a hand did not contain exactly four hand cards.
var ErrInvalidHand = errors.New("cribbage: hand must contain exactly four cards")

// Validate checks that the hand is well formed for scoring.
func (h Hand) Validate() error {
	if len(h.Cards) != 4 {
		return fmt.Errorf("%w: got %d", ErrInvalidHand, len(h.Cards))
	}
	return nil
}

// AllCards returns the four hand cards plus the starter, the set every scoring
// rule operates over.
func (h Hand) AllCards() []Card {
	out := make([]Card, 0, len(h.Cards)+1)
	out = append(out, h.Cards...)
	out = append(out, h.Starter)
	return out
}

// ScoreComponent is a single named contribution to a hand's total score, the
// unit each scoring microservice returns.
type ScoreComponent struct {
	Rule   string `json:"rule"`
	Points int    `json:"points"`
	Detail string `json:"detail,omitempty"`
}

// ScoreBreakdown is the assembled set of components for a hand.
type ScoreBreakdown struct {
	Components []ScoreComponent `json:"components"`
}

// Total sums the points across all components.
func (b ScoreBreakdown) Total() int {
	total := 0
	for _, c := range b.Components {
		total += c.Points
	}
	return total
}

// IsZero reports whether the breakdown carries no scoring components.
func (b ScoreBreakdown) IsZero() bool {
	return len(b.Components) == 0
}

// Merge appends another breakdown's components into this one and returns the
// result, letting an aggregator stitch peer responses together.
func (b ScoreBreakdown) Merge(other ScoreBreakdown) ScoreBreakdown {
	merged := make([]ScoreComponent, 0, len(b.Components)+len(other.Components))
	merged = append(merged, b.Components...)
	merged = append(merged, other.Components...)
	return ScoreBreakdown{Components: merged}
}
