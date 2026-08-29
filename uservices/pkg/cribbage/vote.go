package cribbage

import (
	"errors"
	"fmt"
	"sync"
)

// Color is the visual color of a playing card, determined entirely by its
// suit: Hearts and Diamonds are Red; Spades and Clubs are Black. It is the
// thing players vote on in a ColorPoll.
type Color string

// The two card colors.
const (
	Red   Color = "red"
	Black Color = "black"
)

// Colors lists the valid card colors in a stable order. It is used both to
// validate votes and to break ties deterministically, so callers never depend
// on Go's randomized map iteration order.
var Colors = []Color{Red, Black}

// ErrUnknownColor indicates a suit (or color string) could not be mapped to
// one of the two card colors.
var ErrUnknownColor = errors.New("cribbage: unknown card color")

// Valid reports whether c is one of the two recognized card colors.
func (c Color) Valid() bool {
	return c == Red || c == Black
}

// Color returns the card color for the suit, or ErrUnknownColor if the suit is
// not one of the four recognized suits.
func (s Suit) Color() (Color, error) {
	switch s {
	case Hearts, Diamonds:
		return Red, nil
	case Spades, Clubs:
		return Black, nil
	default:
		return "", fmt.Errorf("%w: suit %q", ErrUnknownColor, string(s))
	}
}

// Color returns the card's color (Red for Hearts/Diamonds, Black for
// Spades/Clubs), or ErrUnknownColor if the card has an invalid suit.
func (c Card) Color() (Color, error) {
	return c.Suit.Color()
}

// ColorPoll tallies votes for the "coolest card color". It is a small,
// in-memory ballot box: callers record one vote at a time with Vote (or
// VoteCard) and read the standings with Count, Tally, or Leader. The zero
// value is an empty, ready-to-use poll. ColorPoll is safe for concurrent
// use by multiple goroutines.
type ColorPoll struct {
	mu    sync.RWMutex
	votes map[Color]int
}

// Vote records a single vote for color c. It returns ErrUnknownColor (wrapped)
// without mutating the poll if c is not a recognized card color.
func (p *ColorPoll) Vote(c Color) error {
	if !c.Valid() {
		return fmt.Errorf("%w: %q", ErrUnknownColor, string(c))
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.votes == nil {
		p.votes = make(map[Color]int, len(Colors))
	}
	p.votes[c]++
	return nil
}

// VoteCard records a vote for the color of card. It returns ErrUnknownColor
// (wrapped) without mutating the poll if the card's suit is invalid.
func (p *ColorPoll) VoteCard(card Card) error {
	color, err := card.Color()
	if err != nil {
		return err
	}
	return p.Vote(color)
}

// Count returns the number of votes recorded for color c.
func (p *ColorPoll) Count(c Color) int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.votes[c]
}

// Total returns the number of votes recorded across all colors.
func (p *ColorPoll) Total() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	total := 0
	for _, n := range p.votes {
		total += n
	}
	return total
}

// Tally returns a copy of the per-color vote counts, including an explicit
// zero for any recognized color that has not received a vote. The returned map
// is owned by the caller and may be modified freely.
func (p *ColorPoll) Tally() map[Color]int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make(map[Color]int, len(Colors))
	for _, c := range Colors {
		out[c] = p.votes[c]
	}
	return out
}

// Leader returns the winning color and true when exactly one color has the
// most votes. It returns the zero Color and false when no votes have been cast
// or the top colors are tied, so a tie never silently resolves to an arbitrary
// winner. Colors are scanned in the fixed Colors order, so the result does not
// depend on map iteration order.
func (p *ColorPoll) Leader() (Color, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var best Color
	bestVotes := 0
	tied := false
	for _, c := range Colors {
		switch n := p.votes[c]; {
		case n > bestVotes:
			best, bestVotes, tied = c, n, false
		case n == bestVotes && n > 0:
			tied = true
		}
	}
	if bestVotes == 0 || tied {
		return "", false
	}
	return best, true
}
