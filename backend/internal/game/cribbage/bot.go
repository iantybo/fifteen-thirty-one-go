package cribbage

import (
	"errors"
	"math/rand"
	"sort"
	"sync"
	"time"

	"fifteen-thirty-one-go/backend/internal/game/common"
)

type BotDifficulty string

const (
	BotEasy   BotDifficulty = "easy"
	BotMedium BotDifficulty = "medium"
	BotHard   BotDifficulty = "hard"
)

var botRandMu sync.Mutex
var botRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func ChooseDiscard(hand []common.Card, difficulty BotDifficulty) ([]common.Card, error) {
	if len(hand) < 2 {
		return nil, errors.New("hand too small")
	}

	// Always operate on a copy.
	cards := append([]common.Card(nil), hand...)

	switch difficulty {
	case BotEasy:
		botRandMu.Lock()
		botRand.Shuffle(len(cards), func(i, j int) { cards[i], cards[j] = cards[j], cards[i] })
		botRandMu.Unlock()
		return cards[:2], nil
	case BotMedium, BotHard:
		// Simple heuristic: discard two lowest Value15 cards, breaking ties by rank.
		sort.Slice(cards, func(i, j int) bool {
			vi, vj := cards[i].Value15(), cards[j].Value15()
			if vi != vj {
				return vi < vj
			}
			if cards[i].Rank != cards[j].Rank {
				return cards[i].Rank < cards[j].Rank
			}
			return cards[i].Suit < cards[j].Suit
		})
		return cards[:2], nil
	default:
		return ChooseDiscard(hand, BotEasy)
	}
}

// ChoosePeggingPlay returns either a card to play, or go=true if no legal play exists.
func ChoosePeggingPlay(hand []common.Card, peggingTotal int, peggingSeq []common.Card, difficulty BotDifficulty) (card *common.Card, goPlay bool) {
	var legal []common.Card
	for _, c := range hand {
		if peggingTotal+c.Value15() <= 31 {
			legal = append(legal, c)
		}
	}
	if len(legal) == 0 {
		return nil, true
	}

	botRandMu.Lock()
	defer botRandMu.Unlock()

	switch difficulty {
	case BotEasy:
		pick := legal[botRand.Intn(len(legal))]
		return &pick, false
	case BotMedium:
		return bestImmediatePegging(legal, peggingTotal, peggingSeq, false)
	case BotHard:
		return bestImmediatePegging(legal, peggingTotal, peggingSeq, true)
	default:
		pick := legal[botRand.Intn(len(legal))]
		return &pick, false
	}
}

func bestImmediatePegging(legal []common.Card, peggingTotal int, peggingSeq []common.Card, applyRiskPenalty bool) (*common.Card, bool) {
	bestIdx := 0
	bestScore := -999999
	for i, c := range legal {
		points, newTotal, _ := PeggingScore(peggingSeq, c, peggingTotal)
		score := points * 100

		// Prefer lower cards when no points are gained (keeps flexibility).
		score -= c.Value15()

		// Hard: avoid leaving an easy 15/31 setup when possible.
		if applyRiskPenalty {
			need15 := 15 - newTotal
			need31 := 31 - newTotal
			if need15 >= 1 && need15 <= 10 {
				score -= 3
			}
			if need31 >= 1 && need31 <= 10 {
				score -= 3
			}
		}

		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	pick := legal[bestIdx]
	return &pick, false
}


