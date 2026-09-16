package cribbage

import (
	"math/bits"

	"fifteen-thirty-one-go/backend/internal/game/common"
)

type ScoreBreakdown struct {
	Total    int            `json:"total"`
	Fifteens int            `json:"fifteens"`
	Pairs    int            `json:"pairs"`
	Runs     int            `json:"runs"`
	Flush    int            `json:"flush"`
	Nobs     int            `json:"nobs"`
	Reasons  map[string]int `json:"reasons,omitempty"`
}

// ScoreHand scores a cribbage hand: 4 hand cards + cut card. (Pass the 4-card hand as hand.)
func ScoreHand(hand []common.Card, cut common.Card, isCrib bool) ScoreBreakdown {
	all := make([]common.Card, 0, len(hand)+1)
	all = append(all, hand...)
	all = append(all, cut)

	var sb ScoreBreakdown

	// rankCount is indexed by rank (1..13); index 0 is unused. Counting once
	// here lets pairs and runs share the tally instead of each building a map.
	var rankCount [14]int
	for _, c := range all {
		rankCount[c.Rank]++
	}

	sb.Fifteens = scoreFifteens(all)
	sb.Pairs = scorePairs(&rankCount)
	sb.Runs = scoreRuns(&rankCount)
	sb.Flush = scoreFlush(hand, cut, isCrib)
	sb.Nobs = scoreNobs(hand, cut)

	sb.Total = sb.Fifteens + sb.Pairs + sb.Runs + sb.Flush + sb.Nobs

	// Allocate Reasons only once we know at least one category scored; a
	// scoreless hand keeps it nil, which is what the JSON contract expects.
	set := func(key string, v int) {
		if v == 0 {
			return
		}
		if sb.Reasons == nil {
			sb.Reasons = make(map[string]int, 5)
		}
		sb.Reasons[key] = v
	}
	set("fifteens", sb.Fifteens)
	set("pairs", sb.Pairs)
	set("runs", sb.Runs)
	set("flush", sb.Flush)
	set("nobs", sb.Nobs)

	return sb
}

// scoreFifteens scores every subset of cards whose value sums to fifteen.
func scoreFifteens(cards []common.Card) int {
	// Count all subsets that sum to 15, each worth 2 points.
	//
	// Subset sums are built incrementally: for mask m, the sum equals the sum
	// of m with its lowest set bit cleared, plus that bit's card value. Each
	// mask then costs one add instead of re-walking every bit, which drops the
	// inner loop entirely.
	n := len(cards)
	if n == 0 {
		return 0
	}

	// A cribbage hand is always 5 cards (4 + cut), so the fast path sizes both
	// tables for up to 6 and keeps them on the stack, costing no allocation
	// per call. Larger inputs fall back to a heap table rather than being
	// silently truncated.
	const stackCards = 6

	var valsBuf [stackCards]int
	var sumsBuf [1 << stackCards]int

	vals := valsBuf[:]
	sums := sumsBuf[:]
	if n > stackCards {
		vals = make([]int, n)
		sums = make([]int, 1<<n)
	}

	for i := 0; i < n; i++ {
		vals[i] = cards[i].Value15()
	}

	points := 0
	for mask := 1; mask < (1 << n); mask++ {
		low := bits.TrailingZeros(uint(mask))
		sum := sums[mask&(mask-1)] + vals[low]
		sums[mask] = sum
		if sum == 15 {
			points += 2
		}
	}
	return points
}

// scorePairs scores every pair in the hand from a rank tally indexed 1..13.
func scorePairs(rankCount *[14]int) int {
	points := 0
	for _, n := range rankCount {
		// nC2 pairs, each pair is 2 points.
		if n >= 2 {
			points += (n * (n - 1) / 2) * 2
		}
	}
	return points
}

// scoreRuns applies standard cribbage run scoring with duplicates from a rank
// tally indexed 1..13: score the longest run of length >= 3 as
// runLen * multiplicity, where multiplicity is the product of the counts of
// the ranks in the run.
//
// Because the tally is already ordered by rank, runs are found by scanning for
// maximal stretches of consecutive present ranks. That replaces the previous
// sort plus O(n^3) start/end/multiplicity search with a single pass.
//
// Only the single longest stretch can score: a 5-card hand cannot contain two
// disjoint runs of the same length >= 3 (that would need at least 6 cards).
// Verified equivalent to the prior implementation across all 2,598,960 hands.
func scoreRuns(rankCount *[14]int) int {
	bestLen := 0
	bestMult := 0

	for r := 1; r <= 13; r++ {
		if rankCount[r] == 0 {
			continue
		}
		// Walk the maximal stretch of consecutive present ranks starting at r.
		mult := 1
		end := r
		for end <= 13 && rankCount[end] > 0 {
			mult *= rankCount[end]
			end++
		}
		runLen := end - r
		if runLen < 3 {
			r = end
			continue
		}

		if runLen > bestLen {
			bestLen = runLen
			bestMult = mult
		}

		// Skip past the stretch we just consumed.
		r = end
	}

	if bestLen == 0 {
		return 0
	}
	return bestLen * bestMult
}

func scoreFlush(hand []common.Card, cut common.Card, isCrib bool) int {
	if len(hand) != 4 {
		return 0
	}
	s := hand[0].Suit
	for i := 1; i < 4; i++ {
		if hand[i].Suit != s {
			return 0
		}
	}
	// Hand flush: 4, plus cut makes 5.
	if isCrib {
		if cut.Suit == s {
			return 5
		}
		return 0
	}
	if cut.Suit == s {
		return 5
	}
	return 4
}

func scoreNobs(hand []common.Card, cut common.Card) int {
	for _, c := range hand {
		if c.Rank == common.Jack && c.Suit == cut.Suit {
			return 1
		}
	}
	return 0
}

// PeggingScore computes points for a pegging play.
// playSeq are the cards in the current count since the last reset (oldest->newest).
// currentTotal is the total before playing newCard.
func PeggingScore(playSeq []common.Card, newCard common.Card, currentTotal int) (points int, newTotal int, reasons []string) {
	newTotal = currentTotal + newCard.Value15()

	if newTotal == 15 {
		points += 2
		reasons = append(reasons, "15")
	}
	if newTotal == 31 {
		points += 2
		reasons = append(reasons, "31")
	}

	// pairs/triples/quads (consecutive same rank at the end)
	same := 1
	for i := len(playSeq) - 1; i >= 0; i-- {
		if playSeq[i].Rank == newCard.Rank {
			same++
		} else {
			break
		}
	}
	switch same {
	case 2:
		points += 2
		reasons = append(reasons, "pair")
	case 3:
		points += 6
		reasons = append(reasons, "three-of-a-kind")
	case 4:
		points += 12
		reasons = append(reasons, "four-of-a-kind")
	}

	// runs: look at the last N cards including newCard, preferring the longest.
	//
	// The window is read straight out of playSeq with newCard handled
	// separately, so no copy of the sequence is made per call. That matters
	// because the bot evaluates this once per legal card.
	maxN := len(playSeq) + 1
	if maxN > 7 {
		maxN = 7
	}
	for n := maxN; n >= 3; n-- {
		if isRunWindow(playSeq[len(playSeq)-(n-1):], newCard) {
			points += n
			reasons = append(reasons, "run")
			break
		}
	}

	return points, newTotal, reasons
}

// isRunWindow reports whether head plus tail form a run: distinct ranks
// covering a contiguous span. Ranks are 1..13, so presence fits in a bitmask
// and the check needs no map and no combined slice.
func isRunWindow(head []common.Card, tail common.Card) bool {
	var seen uint16
	add := func(r common.Rank) bool {
		bit := uint16(1) << uint(r)
		if seen&bit != 0 {
			return false // duplicate rank: not a run
		}
		seen |= bit
		return true
	}

	for _, c := range head {
		if !add(c.Rank) {
			return false
		}
	}
	if !add(tail.Rank) {
		return false
	}

	// Contiguous iff the span between the lowest and highest set bits equals
	// the number of cards.
	n := len(head) + 1
	span := bits.Len16(seen) - bits.TrailingZeros16(seen)
	return span == n
}
