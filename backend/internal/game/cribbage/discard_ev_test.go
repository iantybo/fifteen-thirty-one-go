package cribbage

import (
	"testing"

	"fifteen-thirty-one-go/backend/internal/game/common"
)

// bruteForceBestDiscard is an independent, deliberately naive reference: it
// enumerates every discard, averages the kept hand over all unseen cuts with
// the public ScoreHand, and returns the best average. chooseDiscardEV must
// agree with it.
func bruteForceBestDiscard(t *testing.T, hand []common.Card, discardCount int) (bestEV float64) {
	t.Helper()

	inHand := func(c common.Card) bool {
		for _, h := range hand {
			if h == c {
				return true
			}
		}
		return false
	}

	keepCount := len(hand) - discardCount
	best := -1.0
	forEachCombination(len(hand), keepCount, func(idx []int) {
		keep := make([]common.Card, 0, keepCount)
		for _, i := range idx {
			keep = append(keep, hand[i])
		}

		total, cuts := 0, 0
		for _, s := range []common.Suit{common.Spades, common.Hearts, common.Diamonds, common.Clubs} {
			for r := common.Rank(1); r <= 13; r++ {
				cut := common.Card{Rank: r, Suit: s}
				if inHand(cut) {
					continue
				}
				total += ScoreHand(keep, cut, false).Total
				cuts++
			}
		}
		if avg := float64(total) / float64(cuts); avg > best {
			best = avg
		}
	})
	return best
}

// evOfDiscard computes the average kept-hand score for a specific discard.
func evOfDiscard(t *testing.T, hand, discard []common.Card) float64 {
	t.Helper()

	keep := complementOf(hand, discard)
	inHand := func(c common.Card) bool {
		for _, h := range hand {
			if h == c {
				return true
			}
		}
		return false
	}

	total, cuts := 0, 0
	for _, s := range []common.Suit{common.Spades, common.Hearts, common.Diamonds, common.Clubs} {
		for r := common.Rank(1); r <= 13; r++ {
			cut := common.Card{Rank: r, Suit: s}
			if inHand(cut) {
				continue
			}
			total += ScoreHand(keep, cut, false).Total
			cuts++
		}
	}
	return float64(total) / float64(cuts)
}

func TestChooseDiscardEVMatchesBruteForce(t *testing.T) {
	hands := []string{
		"5H 5S 5C JD 4H 6S",
		"AH 2S 3C 4D 5H 6S",
		"KH KS QC QD JH 9S",
		"2H 4S 6C 8D 10H QS",
		"5H 5S 10C JD QH KS",
		"3H 3S 4C 4D 5H 5S",
		"AH AS 2C 9D 10H JS",
	}

	for _, spec := range hands {
		hand := mustCards(t, spec)
		got := chooseDiscardEV(hand, 2)
		if len(got) != 2 {
			t.Fatalf("hand %q: got %d discards, want 2", spec, len(got))
		}

		wantEV := bruteForceBestDiscard(t, hand, 2)
		gotEV := evOfDiscard(t, hand, got)
		if gotEV != wantEV {
			t.Errorf("hand %q: discard %v has EV %.4f, best possible is %.4f",
				spec, got, gotEV, wantEV)
		}
	}
}

// TestChooseDiscardEVBeatsHeuristic confirms the lookahead is actually an
// improvement: on a hand where dumping the two lowest cards breaks up a strong
// holding, the EV search must do strictly better.
func TestChooseDiscardEVBeatsHeuristic(t *testing.T) {
	// The heuristic throws the two lowest cards, which here means breaking up
	// a pair of fives -- the most valuable holding in cribbage. The EV search
	// keeps them and discards the disconnected 7-8 instead.
	hand := mustCards(t, "5S QC 7H 5H 8H 10S")

	evPick := chooseDiscardEV(hand, 2)
	heuristicPick := lowestValueDiscard(append([]common.Card(nil), hand...), 2)

	evScore := evOfDiscard(t, hand, evPick)
	heuristicScore := evOfDiscard(t, hand, heuristicPick)

	if evScore <= heuristicScore {
		t.Errorf("EV discard %v scored %.4f, not better than heuristic %v at %.4f",
			evPick, evScore, heuristicPick, heuristicScore)
	}
}

func TestChooseDiscardNHardIsDeterministic(t *testing.T) {
	hand := mustCards(t, "5H 5S 5C JD 4H 6S")

	first, err := ChooseDiscardN(hand, 2, BotHard)
	if err != nil {
		t.Fatalf("ChooseDiscardN: %v", err)
	}
	for i := 0; i < 5; i++ {
		again, err := ChooseDiscardN(hand, 2, BotHard)
		if err != nil {
			t.Fatalf("ChooseDiscardN: %v", err)
		}
		if len(again) != len(first) {
			t.Fatalf("discard length changed: %v vs %v", first, again)
		}
		for j := range first {
			if first[j] != again[j] {
				t.Fatalf("hard discard not deterministic: %v vs %v", first, again)
			}
		}
	}
}

// TestChooseDiscardNDoesNotMutateInput guards the caller's hand slice, which
// the handler reuses after the bot picks.
func TestChooseDiscardNDoesNotMutateInput(t *testing.T) {
	for _, diff := range []BotDifficulty{BotEasy, BotMedium, BotHard} {
		hand := mustCards(t, "5H 5S 5C JD 4H 6S")
		before := append([]common.Card(nil), hand...)

		if _, err := ChooseDiscardN(hand, 2, diff); err != nil {
			t.Fatalf("%s: %v", diff, err)
		}
		for i := range before {
			if hand[i] != before[i] {
				t.Fatalf("%s: input hand mutated at %d: %v -> %v", diff, i, before, hand)
			}
		}
	}
}

// TestChooseDiscardNReturnsCardsFromHand checks every difficulty returns real
// cards drawn from the hand, with no duplicates.
func TestChooseDiscardNReturnsCardsFromHand(t *testing.T) {
	hand := mustCards(t, "AH 2S 4C 5D 6H KS")

	for _, diff := range []BotDifficulty{BotEasy, BotMedium, BotHard, BotDifficulty("bogus")} {
		for _, n := range []int{1, 2, 3} {
			got, err := ChooseDiscardN(hand, n, diff)
			if err != nil {
				t.Fatalf("%s n=%d: %v", diff, n, err)
			}
			if len(got) != n {
				t.Fatalf("%s n=%d: got %d cards, want %d", diff, n, len(got), n)
			}

			seen := map[common.Card]bool{}
			for _, c := range got {
				if seen[c] {
					t.Errorf("%s n=%d: duplicate card %v", diff, n, c)
				}
				seen[c] = true

				found := false
				for _, h := range hand {
					if h == c {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("%s n=%d: card %v not in hand", diff, n, c)
				}
			}
		}
	}
}

func TestForEachCombination(t *testing.T) {
	tests := []struct {
		n, k, want int
	}{
		{6, 4, 15},
		{6, 2, 15},
		{5, 1, 5},
		{4, 4, 1},
		{4, 0, 1},
		{3, 5, 0},
		{3, -1, 0},
	}
	for _, tt := range tests {
		count := 0
		forEachCombination(tt.n, tt.k, func(idx []int) {
			count++
			if len(idx) != tt.k && tt.k > 0 {
				t.Errorf("n=%d k=%d: got %d indices", tt.n, tt.k, len(idx))
			}
			// Indices must be strictly increasing and in range.
			for i, v := range idx {
				if v < 0 || v >= tt.n {
					t.Errorf("n=%d k=%d: index %d out of range", tt.n, tt.k, v)
				}
				if i > 0 && idx[i-1] >= v {
					t.Errorf("n=%d k=%d: indices not increasing: %v", tt.n, tt.k, idx)
				}
			}
		})
		if count != tt.want {
			t.Errorf("n=%d k=%d: got %d combinations, want %d", tt.n, tt.k, count, tt.want)
		}
	}
}

func TestSuitIndexRoundTrip(t *testing.T) {
	for _, s := range []common.Suit{common.Spades, common.Hearts, common.Diamonds, common.Clubs} {
		if got := suitFromIndex(suitIndex(s)); got != s {
			t.Errorf("round trip for %v gave %v", s, got)
		}
	}
}
