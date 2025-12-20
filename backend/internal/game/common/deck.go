package common

import (
	"math/rand"
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
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(cards), func(i, j int) {
		cards[i], cards[j] = cards[j], cards[i]
	})
}


