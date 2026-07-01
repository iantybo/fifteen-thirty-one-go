// Package cribcounter is a small, self-contained cribbage counting library.
//
// It provides an ergonomic surface for scoring a cribbage hand (four hand cards
// plus the cut/starter card) and for scoring a single pegging play. The counting
// rules mirror the server-authoritative engine in
// backend/internal/game/cribbage/scoring.go and reuse the shared card model in
// backend/internal/game/common so results are consistent with the rest of the
// platform.
//
// The package is pure Go (no cgo, no database) so it builds and tests on every
// platform, including Windows. A thin cgo wrapper under the dll/ subdirectory
// exposes this library as a Windows DLL (see dll/main_windows.go).
package cribcounter

import (
	"errors"
	"sort"
	"strings"

	"fifteen-thirty-one-go/backend/internal/game/common"
)

// HandSize is the number of cards in a cribbage hand (excluding the cut card).
const HandSize = 4

// ErrHandSize is returned when a hand does not contain exactly HandSize cards.
var ErrHandSize = errors.New("cribcounter: a cribbage hand must contain exactly 4 cards")

// HandScore is the breakdown of points scored by a cribbage hand. Its JSON shape
// matches cribbage.ScoreBreakdown so callers can consume either interchangeably.
type HandScore struct {
	Total    int            `json:"total"`
	Fifteens int            `json:"fifteens"`
	Pairs    int            `json:"pairs"`
	Runs     int            `json:"runs"`
	Flush    int            `json:"flush"`
	Nobs     int            `json:"nobs"`
	Reasons  map[string]int `json:"reasons,omitempty"`
}

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
// all five cards share a suit).
func CountHand(hand []common.Card, cut common.Card, isCrib bool) (HandScore, error) {
	if len(hand) != HandSize {
		return HandScore{}, ErrHandSize
	}

	all := make([]common.Card, 0, len(hand)+1)
	all = append(all, hand...)
	all = append(all, cut)

	hs := HandScore{Reasons: map[string]int{}}
	hs.Fifteens = scoreFifteens(all)
	hs.Pairs = scorePairs(all)
	hs.Runs = scoreRuns(all)
	hs.Flush = scoreFlush(hand, cut, isCrib)
	hs.Nobs = scoreNobs(hand, cut)
	hs.Total = hs.Fifteens + hs.Pairs + hs.Runs + hs.Flush + hs.Nobs

	if hs.Fifteens > 0 {
		hs.Reasons["fifteens"] = hs.Fifteens
	}
	if hs.Pairs > 0 {
		hs.Reasons["pairs"] = hs.Pairs
	}
	if hs.Runs > 0 {
		hs.Reasons["runs"] = hs.Runs
	}
	if hs.Flush > 0 {
		hs.Reasons["flush"] = hs.Flush
	}
	if hs.Nobs > 0 {
		hs.Reasons["nobs"] = hs.Nobs
	}
	if len(hs.Reasons) == 0 {
		hs.Reasons = nil
	}
	return hs, nil
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
// running count before newCard is played.
func CountPegging(playSeq []common.Card, newCard common.Card, currentTotal int) PeggingResult {
	points, newTotal, reasons := peggingScore(playSeq, newCard, currentTotal)
	return PeggingResult{Points: points, NewTotal: newTotal, Reasons: reasons}
}

func scoreFifteens(cards []common.Card) int {
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
		if n >= 2 {
			points += (n * (n - 1) / 2) * 2
		}
	}
	return points
}

func scoreRuns(cards []common.Card) int {
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
			mult := 1
			for i := start; i <= end; i++ {
				mult *= count[ranks[i]]
			}
			if runLen > bestLen {
				bestLen = runLen
				bestMult = mult
			} else if runLen == bestLen {
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
	if len(hand) != HandSize {
		return 0
	}
	s := hand[0].Suit
	for i := 1; i < HandSize; i++ {
		if hand[i].Suit != s {
			return 0
		}
	}
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

func peggingScore(playSeq []common.Card, newCard common.Card, currentTotal int) (points int, newTotal int, reasons []string) {
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
	minR := 99
	maxR := -99
	for _, c := range cards {
		r := int(c.Rank)
		if seen[r] {
			return false
		}
		seen[r] = true
		if r < minR {
			minR = r
		}
		if r > maxR {
			maxR = r
		}
	}
	if (maxR - minR + 1) != len(cards) {
		return false
	}
	for r := minR; r <= maxR; r++ {
		if !seen[r] {
			return false
		}
	}
	return true
}
