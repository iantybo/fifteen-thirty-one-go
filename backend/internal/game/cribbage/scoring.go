package cribbage

import (
	"fifteen-thirty-one-go/backend/internal/game/common"
	"fifteen-thirty-one-go/backend/internal/game/scoring"
)

// ScoreBreakdown is the categorized point breakdown for a scored cribbage hand.
type ScoreBreakdown = scoring.Breakdown

// ScoreHand scores a cribbage hand: 4 hand cards + cut card. (Pass the 4-card hand as hand.)
func ScoreHand(hand []common.Card, cut common.Card, isCrib bool) ScoreBreakdown {
	return scoring.Hand(hand, cut, isCrib)
}

// PeggingScore computes points for a pegging play.
// playSeq are the cards in the current count since the last reset (oldest->newest).
// currentTotal is the total before playing newCard.
func PeggingScore(playSeq []common.Card, newCard common.Card, currentTotal int) (points int, newTotal int, reasons []string) {
	return scoring.Pegging(playSeq, newCard, currentTotal)
}
