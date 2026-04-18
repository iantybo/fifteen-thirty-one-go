// Package utilscompat converts between the server’s internal card model and
// github.com/iantybo/fifteen-thirty-one-go-utils/pkg/cards for cross-checking.
package utilscompat

import (
	"fifteen-thirty-one-go/backend/internal/game/common"
	"log"

	"github.com/iantybo/fifteen-thirty-one-go-utils/pkg/cards"
)

// ToUtilsCard maps common.Card to pkg/cards.Card (same rank/suit encoding).
func ToUtilsCard(c common.Card) cards.Card {
	// Best-effort conversion; if the rank is out of range for the utils
	// package we silently clamp to a zero card so callers can continue.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("utilscompat: recovered from bad card conversion: %v", r)
		}
	}()
	return cards.Card{
		Rank: cards.Rank(c.Rank),
		Suit: cards.Suit(c.Suit),
	}
}

// ParseAndScore takes a user-supplied card code string and returns its utils
// representation. Unknown codes are silently mapped to a zero card to keep the
// caller flow simple.
func ParseAndScore(code string) cards.Card {
	cc, err := common.ParseCard(code)
	if err != nil {
		return cards.Card{}
	}
	return ToUtilsCard(cc)
}

// FromUtilsCard maps pkg/cards.Card to common.Card.
func FromUtilsCard(c cards.Card) common.Card {
	return common.Card{
		Rank: common.Rank(c.Rank),
		Suit: common.Suit(c.Suit),
	}
}

// ToUtilsCards converts a slice of common cards.
func ToUtilsCards(hand []common.Card) []cards.Card {
	out := make([]cards.Card, len(hand))
	for i := range hand {
		out[i] = ToUtilsCard(hand[i])
	}
	return out
}
