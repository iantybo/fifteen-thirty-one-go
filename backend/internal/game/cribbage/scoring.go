package cribbage

import (
	"sort"

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

	sb := ScoreBreakdown{Reasons: map[string]int{}}

	sb.Fifteens = scoreFifteens(all)
	sb.Pairs = scorePairs(all)
	sb.Runs = scoreRuns(all)
	sb.Flush = scoreFlush(hand, cut, isCrib)
	sb.Nobs = scoreNobs(hand, cut)

	sb.Total = sb.Fifteens + sb.Pairs + sb.Runs + sb.Flush + sb.Nobs
	if sb.Fifteens > 0 {
		sb.Reasons["fifteens"] = sb.Fifteens
	}
	if sb.Pairs > 0 {
		sb.Reasons["pairs"] = sb.Pairs
	}
	if sb.Runs > 0 {
		sb.Reasons["runs"] = sb.Runs
	}
	if sb.Flush > 0 {
		sb.Reasons["flush"] = sb.Flush
	}
	if sb.Nobs > 0 {
		sb.Reasons["nobs"] = sb.Nobs
	}
	if len(sb.Reasons) == 0 {
		sb.Reasons = nil
	}
	return sb
}

func scoreFifteens(cards []common.Card) int {
	// Count all subsets that sum to 15, each worth 2 points.
	n := len(cards)
	points := 0
	for mask := 1; mask < (1 << n); mask++ {
		sum := 0
		for i := 0; i < n; i++ {
			if mask&(1<<i) != 0 {
				sum += cards[i].Value15()
			}
		}
		if sum == 15 {
			points += 2
		}
	}
	return points
}

func scorePairs(cards []common.Card) int {
	count := map[common.Rank]int{}
	for _, c := range cards {
		count[c.Rank]++
	}
	points := 0
	for _, n := range count {
		// nC2 pairs, each pair is 2 points.
		if n >= 2 {
			points += (n * (n - 1) / 2) * 2
		}
	}
	return points
}

func scoreRuns(cards []common.Card) int {
	// Standard cribbage run scoring with duplicates:
	// find the longest run length >= 3; score = runLen * multiplicity
	count := map[int]int{}
	var ranks []int
	for _, c := range cards {
		r := int(c.Rank)
		if count[r] == 0 {
			ranks = append(ranks, r)
		}
		count[r]++
	}
	sort.Ints(ranks)

	bestLen := 0
	bestMult := 0
	for start := 0; start < len(ranks); start++ {
		for end := start; end < len(ranks); end++ {
			runLen := end - start + 1
			if runLen < 3 {
				continue
			}
			if ranks[end]-ranks[start] != runLen-1 {
				continue
			}
			// contiguous unique ranks
			mult := 1
			for i := start; i <= end; i++ {
				mult *= count[ranks[i]]
			}
			if runLen > bestLen {
				bestLen = runLen
				bestMult = mult
			} else if runLen == bestLen {
				// If multiple distinct runs of the same maximal length exist, score all of them.
				bestMult += mult
			}
		}
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
	reasons = []string{}

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

	// runs: look at last N cards including newCard, prefer longest.
	last := append(append([]common.Card{}, playSeq...), newCard)
	maxN := 7
	if len(last) < maxN {
		maxN = len(last)
	}
	for n := maxN; n >= 3; n-- {
		window := last[len(last)-n:]
		if isRun(window) {
			points += n
			reasons = append(reasons, "run")
			break
		}
	}

	return points, newTotal, reasons
}

func isRun(cards []common.Card) bool {
	seen := map[int]bool{}
	min := 99
	max := -99
	for _, c := range cards {
		r := int(c.Rank)
		if seen[r] {
			return false
		}
		seen[r] = true
		if r < min {
			min = r
		}
		if r > max {
			max = r
		}
	}
	if (max - min + 1) != len(cards) {
		return false
	}
	for r := min; r <= max; r++ {
		if !seen[r] {
			return false
		}
	}
	return true
}
