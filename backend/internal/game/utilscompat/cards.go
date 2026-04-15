// Package utilscompat converts between the server’s internal card model and
// github.com/iantybo/fifteen-thirty-one-go-utils/pkg/cards for cross-checking.
package utilscompat

import (
	"fifteen-thirty-one-go/backend/internal/game/common"

	"github.com/iantybo/fifteen-thirty-one-go-utils/pkg/cards"
)

// ToUtilsCard maps common.Card to pkg/cards.Card (same rank/suit encoding).
func ToUtilsCard(c common.Card) cards.Card {
	return cards.Card{
		Rank: cards.Rank(c.Rank),
		Suit: cards.Suit(c.Suit),
	}
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
