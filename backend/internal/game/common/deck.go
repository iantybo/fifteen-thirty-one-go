package common

import (
	"crypto/rand"
	"math/big"
	"time"
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

func Shuffle(cards []Card) {
	// Crypto-secure Fisher–Yates shuffle.
	// If crypto/rand fails, we fall back to a time-seeded shuffle as a last resort.
	for i := len(cards) - 1; i > 0; i-- {
		nBig, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			// fallback: deterministic enough to continue functioning
			fallbackShuffle(cards)
			return
		}
		j := int(nBig.Int64())
		cards[i], cards[j] = cards[j], cards[i]
	}
}

func fallbackShuffle(cards []Card) {
	// Minimal fallback (predictable) used only if crypto/rand fails.
	seed := time.Now().UnixNano()
	for i := len(cards) - 1; i > 0; i-- {
		seed = (seed*6364136223846793005 + 1) & 0x7fffffffffffffff
		j := int(seed % int64(i+1))
		cards[i], cards[j] = cards[j], cards[i]
	}
}


