package common

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

func NewStandardDeck() []Card {
	deck := make([]Card, 0, 52)
	suits := []Suit{Spades, Hearts, Diamonds, Clubs}
	for _, s := range suits {
		for r := 1; r <= 13; r++ {
			deck = append(deck, Card{Rank: Rank(r), Suit: s})
		}
	}
	return deck
}

func Shuffle(cards []Card) error {
	// Crypto-secure Fisher–Yates shuffle.
	for i := len(cards) - 1; i > 0; i-- {
		nBig, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			// Fail fast: a broken CSPRNG must not degrade shuffling security silently.
			return fmt.Errorf("secure shuffle failed: %w", err)
		}
		j := int(nBig.Int64())
		cards[i], cards[j] = cards[j], cards[i]
	}
	return nil
}


