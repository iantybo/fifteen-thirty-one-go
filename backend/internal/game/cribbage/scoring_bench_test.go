package cribbage

import (
	"testing"

	"fifteen-thirty-one-go/backend/internal/game/common"
)

// benchHands covers a spread of scoring shapes so the benchmark is not
// dominated by one cheap path.
var benchHands = []struct {
	hand []common.Card
	cut  common.Card
}{
	{ // 29: maximal fifteens and pairs
		hand: []common.Card{{Rank: 5, Suit: common.Hearts}, {Rank: 5, Suit: common.Spades}, {Rank: 5, Suit: common.Clubs}, {Rank: common.Jack, Suit: common.Diamonds}},
		cut:  common.Card{Rank: 5, Suit: common.Diamonds},
	},
	{ // scoreless
		hand: []common.Card{{Rank: 2, Suit: common.Hearts}, {Rank: 4, Suit: common.Spades}, {Rank: 6, Suit: common.Clubs}, {Rank: 8, Suit: common.Diamonds}},
		cut:  common.Card{Rank: common.King, Suit: common.Hearts},
	},
	{ // run of five
		hand: []common.Card{{Rank: 3, Suit: common.Hearts}, {Rank: 4, Suit: common.Spades}, {Rank: 5, Suit: common.Clubs}, {Rank: 6, Suit: common.Diamonds}},
		cut:  common.Card{Rank: 7, Suit: common.Hearts},
	},
	{ // flush plus run
		hand: []common.Card{{Rank: 3, Suit: common.Hearts}, {Rank: 4, Suit: common.Hearts}, {Rank: 5, Suit: common.Hearts}, {Rank: 9, Suit: common.Hearts}},
		cut:  common.Card{Rank: common.King, Suit: common.Hearts},
	},
	{ // double double run
		hand: []common.Card{{Rank: 2, Suit: common.Hearts}, {Rank: 3, Suit: common.Spades}, {Rank: 3, Suit: common.Clubs}, {Rank: 4, Suit: common.Diamonds}},
		cut:  common.Card{Rank: 4, Suit: common.Hearts},
	},
}

func BenchmarkScoreHand(b *testing.B) {
	b.ReportAllocs()
	var sink int
	for i := 0; i < b.N; i++ {
		h := benchHands[i%len(benchHands)]
		sink += ScoreHand(h.hand, h.cut, false).Total
	}
	if sink < 0 {
		b.Fatal("unreachable")
	}
}

func BenchmarkPeggingScore(b *testing.B) {
	seq := []common.Card{
		{Rank: 3, Suit: common.Hearts},
		{Rank: 4, Suit: common.Spades},
		{Rank: 5, Suit: common.Clubs},
	}
	newCard := common.Card{Rank: 6, Suit: common.Diamonds}

	b.ReportAllocs()
	var sink int
	for i := 0; i < b.N; i++ {
		pts, _, _ := PeggingScore(seq, newCard, 12)
		sink += pts
	}
	if sink < 0 {
		b.Fatal("unreachable")
	}
}

// BenchmarkChoosePeggingPlay exercises the bot path, which calls PeggingScore
// once per legal card.
func BenchmarkChoosePeggingPlay(b *testing.B) {
	hand := []common.Card{
		{Rank: 6, Suit: common.Diamonds},
		{Rank: 7, Suit: common.Hearts},
		{Rank: common.King, Suit: common.Spades},
		{Rank: common.Ace, Suit: common.Clubs},
	}
	seq := []common.Card{
		{Rank: 3, Suit: common.Hearts},
		{Rank: 4, Suit: common.Spades},
		{Rank: 5, Suit: common.Clubs},
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ChoosePeggingPlay(hand, 12, seq, BotHard)
	}
}

// BenchmarkChooseDiscardN compares the discard strategies. Hard runs the full
// expected-value search (C(6,2) keeps x 46 cuts), medium the cheap heuristic,
// so the pair shows what the lookahead actually costs per decision.
func BenchmarkChooseDiscardN(b *testing.B) {
	hand := []common.Card{
		{Rank: 5, Suit: common.Spades},
		{Rank: common.Queen, Suit: common.Clubs},
		{Rank: 7, Suit: common.Hearts},
		{Rank: 5, Suit: common.Hearts},
		{Rank: 8, Suit: common.Hearts},
		{Rank: 10, Suit: common.Spades},
	}

	for _, diff := range []BotDifficulty{BotMedium, BotHard} {
		b.Run(string(diff), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := ChooseDiscardN(hand, 2, diff); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkScoreParts measures the allocation-free scoring core used by the
// discard search, without the Reasons map that ScoreHand builds.
func BenchmarkScoreParts(b *testing.B) {
	b.ReportAllocs()
	var sink int
	for i := 0; i < b.N; i++ {
		h := benchHands[i%len(benchHands)]
		sink += scoreParts(h.hand, h.cut, false).Total
	}
	if sink < 0 {
		b.Fatal("unreachable")
	}
}
