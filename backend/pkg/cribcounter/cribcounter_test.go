package cribcounter

import (
	"reflect"
	"testing"

	"fifteen-thirty-one-go/backend/internal/game/common"
)

func mustParse(t *testing.T, s string) common.Card {
	t.Helper()
	c, err := common.ParseCard(s)
	if err != nil {
		t.Fatalf("ParseCard(%q) error: %v", s, err)
	}
	return c
}

func TestParseCards(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"spaces", "5H 5S 5D 5C", 4},
		{"commas", "AS,10C,KD,2H", 4},
		{"mixed", "5H, 5S; 5D  5C  JD", 5},
		{"empty", "   ", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseCards(tc.in)
			if err != nil {
				t.Fatalf("ParseCards(%q) error: %v", tc.in, err)
			}
			if len(got) != tc.want {
				t.Fatalf("ParseCards(%q) = %d cards, want %d", tc.in, len(got), tc.want)
			}
		})
	}

	if _, err := ParseCards("XX"); err == nil {
		t.Fatalf("expected error for invalid card, got nil")
	}
}

func TestCountHandSize(t *testing.T) {
	_, err := CountHand([]common.Card{mustParse(t, "5H")}, mustParse(t, "5S"), false)
	if err != ErrHandSize {
		t.Fatalf("expected ErrHandSize, got %v", err)
	}
}

func TestCountHandDuplicateCard(t *testing.T) {
	// The cut duplicates a card already in the hand.
	_, err := CountHandStrings("5H 5S 5D JC", "5H", false)
	if err != ErrDuplicateCard {
		t.Fatalf("expected ErrDuplicateCard, got %v", err)
	}
}

func TestCountHand(t *testing.T) {
	cases := []struct {
		name   string
		hand   string
		cut    string
		isCrib bool
		want   HandScore
	}{
		{
			// The perfect 29 hand: J5-5-5 with the matching-suit five as the cut.
			name: "perfect-29",
			hand: "5H 5S 5D JC",
			cut:  "5C",
			want: HandScore{
				Total: 29, Fifteens: 16, Pairs: 12, Nobs: 1,
				Reasons: map[string]int{"fifteens": 16, "pairs": 12, "nobs": 1},
			},
		},
		{
			// Four fives (no jack): eight fifteens (16) + four-of-a-kind (12) = 28.
			name: "four-fives",
			hand: "5H 5S 5D 5C",
			cut:  "KD",
			want: HandScore{
				Total: 28, Fifteens: 16, Pairs: 12,
				Reasons: map[string]int{"fifteens": 16, "pairs": 12},
			},
		},
		{
			// Run of four, two fifteens ({4,5,6} and {6,9}) and a four-card hand flush.
			name: "run-and-flush",
			hand: "4H 5H 6H 7H",
			cut:  "9S",
			want: HandScore{
				Total: 12, Fifteens: 4, Runs: 4, Flush: 4,
				Reasons: map[string]int{"fifteens": 4, "runs": 4, "flush": 4},
			},
		},
		{
			// Five-card flush when the cut matches the hand suit; one fifteen ({3,4,8}).
			name: "five-flush",
			hand: "3H 4H 6H 8H",
			cut:  "10H",
			want: HandScore{
				Total: 7, Fifteens: 2, Flush: 5,
				Reasons: map[string]int{"fifteens": 2, "flush": 5},
			},
		},
		{
			// Non-crib hand: a four-card flush counts even when the cut is off-suit.
			name: "four-card-flush",
			hand: "2H 4H 6H 9H",
			cut:  "10S",
			want: HandScore{
				Total: 8, Fifteens: 4, Flush: 4,
				Reasons: map[string]int{"fifteens": 4, "flush": 4},
			},
		},
		{
			// Same cards scored as a crib: a four-card flush does NOT count (needs all five).
			name:   "crib-no-four-flush",
			hand:   "2H 4H 6H 9H",
			cut:    "10S",
			isCrib: true,
			want: HandScore{
				Total: 4, Fifteens: 4,
				Reasons: map[string]int{"fifteens": 4},
			},
		},
		{
			name: "double-run",
			hand: "3H 3S 4D 5C",
			cut:  "9H",
			// Double run of three (3-4-5) = 6, one pair = 2, two fifteens ({3,3,9},{3,3,4,5}) = 4.
			want: HandScore{
				Total: 12, Fifteens: 4, Pairs: 2, Runs: 6,
				Reasons: map[string]int{"fifteens": 4, "pairs": 2, "runs": 6},
			},
		},
		{
			name: "nineteen-nothing",
			hand: "2H 4S 6D 8C",
			cut:  "KH",
			want: HandScore{Total: 0},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CountHandStrings(tc.hand, tc.cut, tc.isCrib)
			if err != nil {
				t.Fatalf("CountHandStrings error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("CountHandStrings(%q, %q, crib=%v) =\n  %+v\nwant\n  %+v", tc.hand, tc.cut, tc.isCrib, got, tc.want)
			}
		})
	}
}

func TestCountHandTotalConsistency(t *testing.T) {
	got, err := CountHandStrings("5H 5S 5D JC", "5C", false)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got.Total != got.Fifteens+got.Pairs+got.Runs+got.Flush+got.Nobs {
		t.Fatalf("Total %d does not equal sum of components %+v", got.Total, got)
	}
}

func TestCountPegging(t *testing.T) {
	cases := []struct {
		name  string
		seq   []common.Card
		card  common.Card
		total int
		want  PeggingResult
	}{
		{
			name:  "fifteen-two",
			seq:   []common.Card{mustParse(t, "7H")},
			card:  mustParse(t, "8S"),
			total: 7,
			want:  PeggingResult{Points: 2, NewTotal: 15, Reasons: []string{"15"}},
		},
		{
			name:  "thirty-one",
			seq:   []common.Card{mustParse(t, "KH"), mustParse(t, "QS"), mustParse(t, "AC")},
			card:  mustParse(t, "10D"),
			total: 21,
			want:  PeggingResult{Points: 2, NewTotal: 31, Reasons: []string{"31"}},
		},
		{
			name:  "pair",
			seq:   []common.Card{mustParse(t, "4H")},
			card:  mustParse(t, "4S"),
			total: 4,
			want:  PeggingResult{Points: 2, NewTotal: 8, Reasons: []string{"pair"}},
		},
		{
			name:  "run-of-three",
			seq:   []common.Card{mustParse(t, "3H"), mustParse(t, "5S")},
			card:  mustParse(t, "4D"),
			total: 8,
			want:  PeggingResult{Points: 3, NewTotal: 12, Reasons: []string{"run"}},
		},
		{
			name:  "nothing",
			seq:   []common.Card{mustParse(t, "2H")},
			card:  mustParse(t, "5S"),
			total: 2,
			want:  PeggingResult{Points: 0, NewTotal: 7, Reasons: []string{}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CountPegging(tc.seq, tc.card, tc.total)
			if err != nil {
				t.Fatalf("CountPegging() unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("CountPegging() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestCountPeggingInvalidTotal(t *testing.T) {
	cases := []struct {
		name  string
		card  common.Card
		total int
	}{
		{"negative", mustParse(t, "5S"), -1},
		{"above-31", mustParse(t, "5S"), 32},
		{"would-exceed-31", mustParse(t, "10S"), 30},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CountPegging(nil, tc.card, tc.total)
			if err != ErrPeggingTotal {
				t.Fatalf("expected ErrPeggingTotal, got %v", err)
			}
		})
	}
}
