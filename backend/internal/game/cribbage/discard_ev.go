package cribbage

import (
	"fifteen-thirty-one-go/backend/internal/game/common"
)

// deckRanks is the number of distinct ranks in a standard deck.
const deckRanks = 13

// suitsPerRank is how many cards share each rank in a standard deck.
const suitsPerRank = 4

// evScale is the fixed-point scale for expected values. Averages are kept in
// scaled integers so the search stays in integer math and comparisons are
// exact, avoiding float accumulation order effects.
const evScale = 1 << 16

// chooseDiscardEV picks the discard that maximises the expected score of the
// hand that is kept, averaged over every cut card that could still appear.
//
// For each candidate keep, the hand is scored against all 46 unseen cuts (52
// minus the 6 dealt) and the mean is taken. The discard with the best-scoring
// remainder wins. This is the standard expected-hand-value heuristic; it
// ignores crib ownership and pegging potential, which are handled elsewhere.
//
// Cost is C(len(hand), keepCount) keeps x unseen cuts calls to scoreParts.
// For the usual 6-card hand that is 15 x 46 = 690 scorings, which is
// affordable only because scoreParts allocates nothing.
func chooseDiscardEV(hand []common.Card, discardCount int) []common.Card {
	keepCount := len(hand) - discardCount

	// Cuts are drawn from the cards not already in hand. Rank counts are enough
	// to weight the average, but nobs and flush depend on suit, so cuts are
	// enumerated as concrete cards.
	var dealt [deckRanks + 1][suitsPerRank]bool
	for _, c := range hand {
		// Ranks outside 1..13 are not dealable cards; ignoring them keeps the
		// cut enumeration in range for hands rehydrated from stored JSON.
		if c.Rank < 1 || c.Rank > deckRanks {
			continue
		}
		dealt[c.Rank][suitIndex(c.Suit)] = true
	}

	var (
		bestEV    int
		bestKeep  []common.Card
		keep      = make([]common.Card, 0, keepCount)
		bestFound bool
	)

	forEachCombination(len(hand), keepCount, func(idx []int) {
		keep = keep[:0]
		for _, i := range idx {
			keep = append(keep, hand[i])
		}

		total := 0
		cuts := 0
		for r := common.Rank(1); r <= deckRanks; r++ {
			for s := 0; s < suitsPerRank; s++ {
				if dealt[r][s] {
					continue
				}
				cut := common.Card{Rank: r, Suit: suitFromIndex(s)}
				total += scoreParts(keep, cut, false).Total
				cuts++
			}
		}
		if cuts == 0 {
			return
		}

		ev := total * evScale / cuts
		if !bestFound || ev > bestEV {
			bestFound = true
			bestEV = ev
			bestKeep = append(bestKeep[:0], keep...)
		}
	})

	if !bestFound {
		return nil
	}

	// Return the complement: the cards not kept are the discard.
	return complementOf(hand, bestKeep)
}

// complementOf returns the cards of hand that are not present in keep.
//
// keep is always a subset of hand built from distinct positions, so matching
// is done by walking hand and consuming keep entries by value. That avoids
// indexing by rank, which would need bounds checks for malformed input, and
// correctly handles the case where hand contains duplicate cards.
func complementOf(hand, keep []common.Card) []common.Card {
	used := make([]bool, len(keep))

	out := make([]common.Card, 0, len(hand)-len(keep))
	for _, c := range hand {
		matched := false
		for i, k := range keep {
			if used[i] || k != c {
				continue
			}
			used[i] = true
			matched = true
			break
		}
		if !matched {
			out = append(out, c)
		}
	}
	return out
}

// forEachCombination calls fn with each set of k indices chosen from [0,n),
// in lexicographic order. The index slice is reused between calls, so fn must
// not retain it.
func forEachCombination(n, k int, fn func(idx []int)) {
	if k < 0 || k > n {
		return
	}
	if k == 0 {
		fn(nil)
		return
	}

	idx := make([]int, k)
	for i := range idx {
		idx[i] = i
	}
	for {
		fn(idx)

		// Advance to the next combination: find the rightmost index that can
		// still be incremented, bump it, and reset everything after it.
		i := k - 1
		for i >= 0 && idx[i] == i+n-k {
			i--
		}
		if i < 0 {
			return
		}
		idx[i]++
		for j := i + 1; j < k; j++ {
			idx[j] = idx[j-1] + 1
		}
	}
}

// suitIndex maps a suit to a stable slot for the small lookup arrays above.
func suitIndex(s common.Suit) int {
	switch s {
	case common.Spades:
		return 0
	case common.Hearts:
		return 1
	case common.Diamonds:
		return 2
	case common.Clubs:
		return 3
	default:
		return 0
	}
}

// suitFromIndex is the inverse of suitIndex.
func suitFromIndex(i int) common.Suit {
	switch i {
	case 0:
		return common.Spades
	case 1:
		return common.Hearts
	case 2:
		return common.Diamonds
	case 3:
		return common.Clubs
	default:
		return common.Spades
	}
}
