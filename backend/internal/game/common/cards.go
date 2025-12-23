package common

import (
	"fmt"
	"strings"
)

type Suit string

const (
	Spades   Suit = "S"
	Hearts   Suit = "H"
	Diamonds Suit = "D"
	Clubs    Suit = "C"
)

type Rank int

const (
	Ace   Rank = 1
	Jack  Rank = 11
	Queen Rank = 12
	King  Rank = 13
)

type Card struct {
	Rank Rank `json:"rank"`
	Suit Suit `json:"suit"`
}

func (c Card) String() string {
	var r string
	switch c.Rank {
	case Ace:
		r = "A"
	case Jack:
		r = "J"
	case Queen:
		r = "Q"
	case King:
		r = "K"
	default:
		r = fmt.Sprintf("%d", int(c.Rank))
	}
	return r + string(c.Suit)
}

func ParseCard(s string) (Card, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	if len(s) < 2 {
		return Card{}, fmt.Errorf("invalid card")
	}
	suit := Suit(s[len(s)-1:])
	rankStr := s[:len(s)-1]
	var r Rank
	switch rankStr {
	case "A":
		r = Ace
	case "J":
		r = Jack
	case "Q":
		r = Queen
	case "K":
		r = King
	default:
		var v int
		_, err := fmt.Sscanf(rankStr, "%d", &v)
		if err != nil || v < 2 || v > 10 {
			return Card{}, fmt.Errorf("invalid rank")
		}
		r = Rank(v)
	}
	switch suit {
	case Spades, Hearts, Diamonds, Clubs:
	default:
		return Card{}, fmt.Errorf("invalid suit")
	}
	return Card{Rank: r, Suit: suit}, nil
}

func (c Card) Value15() int {
	// For 15s and pegging totals: face cards are 10, ace is 1.
	if c.Rank >= 10 {
		return 10
	}
	return int(c.Rank)
}


