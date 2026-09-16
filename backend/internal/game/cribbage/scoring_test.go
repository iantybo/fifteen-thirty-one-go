package cribbage

import (
	"testing"

	"fifteen-thirty-one-go/backend/internal/game/common"
)

func c(r common.Rank, s common.Suit) common.Card { return common.Card{Rank: r, Suit: s} }

// mustCards parses a compact "5H 5S 5D JC" spec into cards.
func mustCards(t *testing.T, spec string) []common.Card {
	t.Helper()
	var out []common.Card
	start := 0
	for i := 0; i <= len(spec); i++ {
		if i == len(spec) || spec[i] == ' ' {
			if i > start {
				card, err := common.ParseCard(spec[start:i])
				if err != nil {
					t.Fatalf("ParseCard(%q): %v", spec[start:i], err)
				}
				out = append(out, card)
			}
			start = i + 1
		}
	}
	return out
}

func TestScoreHand(t *testing.T) {
	tests := []struct {
		name   string
		hand   string
		cut    string
		isCrib bool
		want   ScoreBreakdown
	}{
		{
			name: "perfect 29",
			hand: "5H 5S 5C JD",
			cut:  "5D",
			// fifteens: 8 ways x2 = 16, pairs: 4C2=6 pairs x2 = 12, nobs 1
			want: ScoreBreakdown{Total: 29, Fifteens: 16, Pairs: 12, Nobs: 1},
		},
		{
			name: "zero points",
			hand: "2H 4S 6C 8D",
			cut:  "KH",
			want: ScoreBreakdown{Total: 0},
		},
		{
			name: "run of five",
			hand: "3H 4S 5C 6D",
			cut:  "7H",
			// run of 5 = 5, fifteens: 7+8(3+5)... computed by impl
			want: ScoreBreakdown{Total: 9, Fifteens: 4, Runs: 5},
		},
		{
			name: "double run of three",
			hand: "4H 5S 6C 6D",
			cut:  "KH",
			// fifteens: 4+5+6 two ways (one per 6) and K+5 = 3 x2 = 6;
			// runs: 4-5-6 of length 3 with multiplicity 2 = 6; pair of 6s = 2
			want: ScoreBreakdown{Total: 14, Fifteens: 6, Pairs: 2, Runs: 6},
		},
		{
			name: "hand flush of four, cut off suit",
			hand: "2H 4H 6H 8H",
			cut:  "KS",
			want: ScoreBreakdown{Total: 4, Flush: 4},
		},
		{
			name: "hand flush of five",
			hand: "2H 4H 6H 8H",
			cut:  "KH",
			want: ScoreBreakdown{Total: 5, Flush: 5},
		},
		{
			name:   "crib flush of four does not score",
			hand:   "2H 4H 6H 8H",
			cut:    "KS",
			isCrib: true,
			want:   ScoreBreakdown{Total: 0},
		},
		{
			name:   "crib flush of five scores",
			hand:   "2H 4H 6H 8H",
			cut:    "KH",
			isCrib: true,
			want:   ScoreBreakdown{Total: 5, Flush: 5},
		},
		{
			name: "nobs only",
			hand: "JH 2S 4C 6D",
			cut:  "KH",
			want: ScoreBreakdown{Total: 1, Nobs: 1},
		},
		{
			name: "four of a kind",
			hand: "7H 7S 7C 7D",
			cut:  "KH",
			// pairs: 6 pairs x2 = 12
			want: ScoreBreakdown{Total: 12, Pairs: 12},
		},
		{
			name: "fifteens with face cards",
			hand: "5H 5S KC QD",
			cut:  "JH",
			// fifteens: each 5 with each of J/Q/K = 6 x2 = 12; pair of 5s = 2;
			// J-Q-K is a run of 3. No nobs: the Jack is the cut, not a hand card.
			want: ScoreBreakdown{Total: 17, Fifteens: 12, Pairs: 2, Runs: 3},
		},
		{
			name: "double double run",
			hand: "2H 3S 3C 4D",
			cut:  "4H",
			// runs of 3 with mult 4 = 12, two pairs = 4
			want: ScoreBreakdown{Total: 16, Pairs: 4, Runs: 12},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hand := mustCards(t, tt.hand)
			cutCards := mustCards(t, tt.cut)
			if len(cutCards) != 1 {
				t.Fatalf("want exactly one cut card, got %d", len(cutCards))
			}
			got := ScoreHand(hand, cutCards[0], tt.isCrib)

			if got.Total != tt.want.Total {
				t.Errorf("Total = %d, want %d", got.Total, tt.want.Total)
			}
			if got.Fifteens != tt.want.Fifteens {
				t.Errorf("Fifteens = %d, want %d", got.Fifteens, tt.want.Fifteens)
			}
			if got.Pairs != tt.want.Pairs {
				t.Errorf("Pairs = %d, want %d", got.Pairs, tt.want.Pairs)
			}
			if got.Runs != tt.want.Runs {
				t.Errorf("Runs = %d, want %d", got.Runs, tt.want.Runs)
			}
			if got.Flush != tt.want.Flush {
				t.Errorf("Flush = %d, want %d", got.Flush, tt.want.Flush)
			}
			if got.Nobs != tt.want.Nobs {
				t.Errorf("Nobs = %d, want %d", got.Nobs, tt.want.Nobs)
			}
		})
	}
}

func TestScoreHandReasons(t *testing.T) {
	t.Run("nil when scoreless", func(t *testing.T) {
		got := ScoreHand(mustCards(t, "2H 4S 6C 8D"), c(common.King, common.Hearts), false)
		if got.Reasons != nil {
			t.Errorf("Reasons = %v, want nil", got.Reasons)
		}
	})

	t.Run("populated per category", func(t *testing.T) {
		got := ScoreHand(mustCards(t, "5H 5S 5C JD"), c(5, common.Diamonds), false)
		want := map[string]int{"fifteens": 16, "pairs": 12, "nobs": 1}
		if len(got.Reasons) != len(want) {
			t.Fatalf("Reasons = %v, want %v", got.Reasons, want)
		}
		for k, v := range want {
			if got.Reasons[k] != v {
				t.Errorf("Reasons[%q] = %d, want %d", k, got.Reasons[k], v)
			}
		}
	})
}

func TestPeggingScore(t *testing.T) {
	tests := []struct {
		name        string
		seq         string
		newCard     string
		total       int
		wantPoints  int
		wantTotal   int
		wantReasons []string
	}{
		{
			name: "empty sequence scores nothing", seq: "", newCard: "4H", total: 0,
			wantPoints: 0, wantTotal: 4, wantReasons: []string{},
		},
		{
			name: "fifteen two", seq: "7H", newCard: "8S", total: 7,
			wantPoints: 2, wantTotal: 15, wantReasons: []string{"15"},
		},
		{
			name: "thirty one for two", seq: "KH KS", newCard: "AC", total: 30,
			wantPoints: 2, wantTotal: 31, wantReasons: []string{"31"},
		},
		{
			name: "pair", seq: "5H", newCard: "5S", total: 5,
			wantPoints: 2, wantTotal: 10, wantReasons: []string{"pair"},
		},
		{
			name: "three of a kind plus fifteen", seq: "5H 5S", newCard: "5C", total: 10,
			wantPoints: 8, wantTotal: 15, wantReasons: []string{"15", "three-of-a-kind"},
		},
		{
			name: "four of a kind", seq: "2H 2S 2C", newCard: "2D", total: 6,
			wantPoints: 12, wantTotal: 8, wantReasons: []string{"four-of-a-kind"},
		},
		{
			name: "run of three plus fifteen", seq: "4H 5S", newCard: "6C", total: 9,
			wantPoints: 5, wantTotal: 15, wantReasons: []string{"15", "run"},
		},
		{
			name: "run of three out of order", seq: "5H 4S", newCard: "3C", total: 9,
			wantPoints: 3, wantTotal: 12, wantReasons: []string{"run"},
		},
		{
			name: "run of four", seq: "3H 4S 5C", newCard: "6D", total: 12,
			wantPoints: 4, wantTotal: 18, wantReasons: []string{"run"},
		},
		{
			// A pegging run needs the last N cards to be distinct ranks; the
			// duplicate 4 blocks both the 3-card and 4-card window.
			name: "duplicate rank blocks the run", seq: "3H 4S 4C", newCard: "5D", total: 11,
			wantPoints: 0, wantTotal: 16, wantReasons: []string{},
		},
		{
			name: "face cards are ten but rank distinct for runs", seq: "JH", newCard: "QS", total: 10,
			wantPoints: 0, wantTotal: 20, wantReasons: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := mustCards(t, tt.seq)
			newCards := mustCards(t, tt.newCard)
			if len(newCards) != 1 {
				t.Fatalf("want exactly one new card, got %d", len(newCards))
			}

			gotPoints, gotTotal, gotReasons := PeggingScore(seq, newCards[0], tt.total)

			if gotPoints != tt.wantPoints {
				t.Errorf("points = %d, want %d", gotPoints, tt.wantPoints)
			}
			if gotTotal != tt.wantTotal {
				t.Errorf("newTotal = %d, want %d", gotTotal, tt.wantTotal)
			}
			if len(gotReasons) != len(tt.wantReasons) {
				t.Fatalf("reasons = %v, want %v", gotReasons, tt.wantReasons)
			}
			for i := range tt.wantReasons {
				if gotReasons[i] != tt.wantReasons[i] {
					t.Errorf("reasons[%d] = %q, want %q", i, gotReasons[i], tt.wantReasons[i])
				}
			}
		})
	}
}

func TestPeggingScoreDoesNotMutateSequence(t *testing.T) {
	seq := mustCards(t, "3H 4S 5C")
	before := append([]common.Card(nil), seq...)

	PeggingScore(seq, c(6, common.Diamonds), 12)

	for i := range before {
		if seq[i] != before[i] {
			t.Fatalf("PeggingScore mutated playSeq at %d: got %v, want %v", i, seq[i], before[i])
		}
	}
}
