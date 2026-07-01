// Package cribcounter is a small, self-contained cribbage counting library.
//
// It provides an ergonomic surface for scoring a cribbage hand (four hand cards
// plus the cut/starter card) and for scoring a single pegging play. The counting
// rules come from the shared engine in backend/internal/game/scoring (the same
// algorithms the server uses) and reuse the shared card model in
// backend/internal/game/common, so results stay consistent with the rest of the
// platform.
//
// The package is pure Go (no cgo, no database) so it builds and tests on every
// platform, including Windows. A thin cgo wrapper under the dll/ subdirectory
// exposes this library as a Windows DLL (see dll/main_windows.go).
package cribcounter

import (
	"errors"
	"strings"

	"fifteen-thirty-one-go/backend/internal/game/common"
	"fifteen-thirty-one-go/backend/internal/game/scoring"
)

// HandSize is the number of cards in a cribbage hand (excluding the cut card).
const HandSize = scoring.HandSize

// maxPeggingTotal is the highest legal running total during pegging.
const maxPeggingTotal = 31

var (
	// ErrHandSize is returned when a hand does not contain exactly HandSize cards.
	ErrHandSize = errors.New("cribcounter: a cribbage hand must contain exactly 4 cards")
	// ErrDuplicateCard is returned when the hand and cut contain the same physical card twice.
	ErrDuplicateCard = errors.New("cribcounter: hand and cut must not contain duplicate cards")
	// ErrPeggingTotal is returned when a pegging total is negative, exceeds 31, or would exceed 31.
	ErrPeggingTotal = errors.New("cribcounter: pegging total must stay between 0 and 31")
)

// HandScore is the breakdown of points scored by a cribbage hand. It is the same
// type the server engine returns (cribbage.ScoreBreakdown), so callers can
// consume either interchangeably.
type HandScore = scoring.Breakdown

// PeggingResult describes the outcome of a single pegging play.
type PeggingResult struct {
	Points   int      `json:"points"`
	NewTotal int      `json:"new_total"`
	Reasons  []string `json:"reasons"`
}

// ParseCards parses a whitespace- and/or comma-separated list of cards using the
// shared "RankSuit" encoding (e.g. "5H", "10C", "AS", "KD"). Ten must be written
// as "10", never "T".
func ParseCards(s string) ([]common.Card, error) {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	cards := make([]common.Card, 0, len(fields))
	for _, f := range fields {
		c, err := common.ParseCard(f)
		if err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, nil
}

// CountHand scores a cribbage hand: the four hand cards plus the cut card. Set
// isCrib to true when scoring the dealer's crib (a crib only earns a flush when
// all five cards share a suit). It returns an error when the hand does not have
// exactly four cards or when any physical card is repeated across hand and cut.
func CountHand(hand []common.Card, cut common.Card, isCrib bool) (HandScore, error) {
	if len(hand) != HandSize {
		return HandScore{}, ErrHandSize
	}

	all := make([]common.Card, 0, len(hand)+1)
	all = append(all, hand...)
	all = append(all, cut)
	if hasDuplicateCards(all) {
		return HandScore{}, ErrDuplicateCard
	}

	return scoring.Hand(hand, cut, isCrib), nil
}

// CountHandStrings is a convenience wrapper around CountHand that parses the hand
// and cut from their string encodings.
func CountHandStrings(hand, cut string, isCrib bool) (HandScore, error) {
	cards, err := ParseCards(hand)
	if err != nil {
		return HandScore{}, err
	}
	cutCard, err := common.ParseCard(cut)
	if err != nil {
		return HandScore{}, err
	}
	return CountHand(cards, cutCard, isCrib)
}

// CountPegging scores playing newCard onto the current pegging pile. playSeq are
// the cards played since the last reset (oldest first) and currentTotal is the
// running count before newCard is played. It returns ErrPeggingTotal when
// currentTotal is outside [0, 31] or when playing newCard would push the total
// past 31.
func CountPegging(playSeq []common.Card, newCard common.Card, currentTotal int) (PeggingResult, error) {
	if currentTotal < 0 || currentTotal > maxPeggingTotal {
		return PeggingResult{}, ErrPeggingTotal
	}
	if currentTotal+newCard.Value15() > maxPeggingTotal {
		return PeggingResult{}, ErrPeggingTotal
	}
	points, newTotal, reasons := scoring.Pegging(playSeq, newCard, currentTotal)
	return PeggingResult{Points: points, NewTotal: newTotal, Reasons: reasons}, nil
}

type cardKey struct {
	rank common.Rank
	suit common.Suit
}

func hasDuplicateCards(cards []common.Card) bool {
	seen := make(map[cardKey]struct{}, len(cards))
	for _, c := range cards {
		key := cardKey{rank: c.Rank, suit: c.Suit}
		if _, ok := seen[key]; ok {
			return true
		}
		seen[key] = struct{}{}
	}
	return false
}
