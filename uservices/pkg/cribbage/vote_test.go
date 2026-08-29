package cribbage

import (
	"errors"
	"testing"
)

func TestSuitColor(t *testing.T) {
	cases := []struct {
		suit Suit
		want Color
	}{
		{Hearts, Red},
		{Diamonds, Red},
		{Spades, Black},
		{Clubs, Black},
	}
	for _, tc := range cases {
		got, err := tc.suit.Color()
		if err != nil {
			t.Errorf("Suit(%q).Color() returned error: %v", tc.suit, err)
			continue
		}
		if got != tc.want {
			t.Errorf("Suit(%q).Color() = %q, want %q", tc.suit, got, tc.want)
		}
	}
}

func TestSuitColorInvalid(t *testing.T) {
	if _, err := Suit("X").Color(); !errors.Is(err, ErrUnknownColor) {
		t.Errorf("Suit(\"X\").Color() error = %v, want ErrUnknownColor", err)
	}
}

func TestCardColor(t *testing.T) {
	got, err := Card{Rank: 11, Suit: Hearts}.Color()
	if err != nil {
		t.Fatalf("Card.Color() returned error: %v", err)
	}
	if got != Red {
		t.Errorf("Card{J,H}.Color() = %q, want %q", got, Red)
	}
}

func TestColorPollVote(t *testing.T) {
	var p ColorPoll
	if err := p.Vote(Red); err != nil {
		t.Fatalf("Vote(Red) returned error: %v", err)
	}
	if err := p.Vote(Red); err != nil {
		t.Fatalf("Vote(Red) returned error: %v", err)
	}
	if err := p.Vote(Black); err != nil {
		t.Fatalf("Vote(Black) returned error: %v", err)
	}
	if got := p.Count(Red); got != 2 {
		t.Errorf("Count(Red) = %d, want 2", got)
	}
	if got := p.Total(); got != 3 {
		t.Errorf("Total() = %d, want 3", got)
	}
}

func TestColorPollVoteInvalid(t *testing.T) {
	var p ColorPoll
	if err := p.Vote(Color("green")); !errors.Is(err, ErrUnknownColor) {
		t.Errorf("Vote(green) error = %v, want ErrUnknownColor", err)
	}
	if got := p.Total(); got != 0 {
		t.Errorf("Total() = %d after rejected vote, want 0", got)
	}
}

func TestColorPollVoteCard(t *testing.T) {
	var p ColorPoll
	if err := p.VoteCard(Card{Rank: 5, Suit: Diamonds}); err != nil {
		t.Fatalf("VoteCard returned error: %v", err)
	}
	if got := p.Count(Red); got != 1 {
		t.Errorf("Count(Red) = %d, want 1", got)
	}
	if err := p.VoteCard(Card{Rank: 5, Suit: Suit("X")}); !errors.Is(err, ErrUnknownColor) {
		t.Errorf("VoteCard with invalid suit error = %v, want ErrUnknownColor", err)
	}
	if got := p.Total(); got != 1 {
		t.Errorf("Total() = %d after rejected card vote, want 1", got)
	}
}

func TestColorPollTally(t *testing.T) {
	var p ColorPoll
	if err := p.Vote(Black); err != nil {
		t.Fatalf("Vote(Black) returned error: %v", err)
	}
	tally := p.Tally()
	if tally[Black] != 1 || tally[Red] != 0 {
		t.Errorf("Tally() = %v, want red:0 black:1", tally)
	}
	// Mutating the returned map must not affect the poll.
	tally[Black] = 99
	if got := p.Count(Black); got != 1 {
		t.Errorf("Count(Black) = %d after mutating returned tally, want 1", got)
	}
}

func TestColorPollLeader(t *testing.T) {
	t.Run("clear winner", func(t *testing.T) {
		var p ColorPoll
		mustVote(t, &p, Red, Red, Black)
		got, ok := p.Leader()
		if !ok || got != Red {
			t.Errorf("Leader() = (%q, %t), want (red, true)", got, ok)
		}
	})
	t.Run("tie", func(t *testing.T) {
		var p ColorPoll
		mustVote(t, &p, Red, Black)
		if got, ok := p.Leader(); ok {
			t.Errorf("Leader() = (%q, %t), want (\"\", false) on a tie", got, ok)
		}
	})
	t.Run("no votes", func(t *testing.T) {
		var p ColorPoll
		if got, ok := p.Leader(); ok {
			t.Errorf("Leader() = (%q, %t), want (\"\", false) with no votes", got, ok)
		}
	})
}

func mustVote(t *testing.T, p *ColorPoll, colors ...Color) {
	t.Helper()
	for _, c := range colors {
		if err := p.Vote(c); err != nil {
			t.Fatalf("Vote(%q) returned error: %v", c, err)
		}
	}
}
