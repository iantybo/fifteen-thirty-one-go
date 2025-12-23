package cribbage

import (
	"encoding/json"
	"errors"
	"fmt"

	"fifteen-thirty-one-go/backend/internal/game/common"
	"fifteen-thirty-one-go/backend/internal/models"
)

// A minimal server-authoritative cribbage state model.
// This will be expanded as REST/WS handlers are implemented.
type State struct {
	Rules Rules `json:"rules"`

	DealerIndex  int `json:"dealer_index"`
	CurrentIndex int `json:"current_index"`
	LastPlayIndex int `json:"last_play_index"`

	// Deck is persisted for crash/restart recovery but never exposed to clients
	// (handlers intentionally omit it from public snapshots).
	Deck []common.Card `json:"deck"`
	Cut  *common.Card  `json:"cut,omitempty"`

	Hands [][]common.Card `json:"hands"` // per player
	KeptHands [][]common.Card `json:"kept_hands"` // 4-card hands used for counting (set after discards)
	Crib  []common.Card   `json:"crib"`

	PeggingTotal int           `json:"pegging_total"`
	PeggingSeq   []common.Card `json:"pegging_seq"`
	PeggingPassed []bool       `json:"pegging_passed"`
	DiscardCompleted []bool    `json:"discard_completed"`

	Scores []int `json:"scores"`
	Stage  string `json:"stage"` // dealing|discard|pegging|counting|finished
}

func NewState(players int) *State {
	r := DefaultRules(players)
	st := &State{
		Rules:        r,
		DealerIndex:  0,
		CurrentIndex: 0,
		LastPlayIndex: -1,
		Hands:        make([][]common.Card, r.MaxPlayers),
		KeptHands:    make([][]common.Card, r.MaxPlayers),
		Crib:         []common.Card{},
		Scores:       make([]int, r.MaxPlayers),
		Stage:        "dealing",
	}
	st.DiscardCompleted = make([]bool, st.Rules.MaxPlayers)
	return st
}

func (s *State) Deal() error {
	if s.Rules.MaxPlayers < 2 || s.Rules.MaxPlayers > 4 {
		return errors.New("invalid player count")
	}
	s.Deck = common.NewStandardDeck()
	if err := common.Shuffle(s.Deck); err != nil {
		return err
	}

	handSize := s.Rules.HandSize()
	for i := 0; i < s.Rules.MaxPlayers; i++ {
		// NewState initializes Hands as a slice of nil slices; nil[:0] would panic.
		// Ensure each hand is a non-nil empty slice with enough capacity for dealing.
		if s.Hands[i] == nil {
			s.Hands[i] = make([]common.Card, 0, handSize)
		} else {
			s.Hands[i] = s.Hands[i][:0]
		}
	}
	for round := 0; round < handSize; round++ {
		for p := 0; p < s.Rules.MaxPlayers; p++ {
			c, err := s.pop()
			if err != nil {
				return err
			}
			s.Hands[p] = append(s.Hands[p], c)
		}
	}
	s.Crib = s.Crib[:0]
	s.Cut = nil
	s.Stage = "discard"
	s.KeptHands = make([][]common.Card, s.Rules.MaxPlayers)
	s.PeggingPassed = make([]bool, s.Rules.MaxPlayers)
	s.PeggingSeq = nil
	s.PeggingTotal = 0
	s.LastPlayIndex = -1
	s.DiscardCompleted = make([]bool, s.Rules.MaxPlayers)

	// Next player after dealer starts discarding in UI flows; pegging starts left of dealer.
	s.CurrentIndex = (s.DealerIndex + 1) % s.Rules.MaxPlayers
	return nil
}

func (s *State) Discard(player int, cards []common.Card) error {
	if s.Stage != "discard" {
		return models.ErrNotInDiscardStage
	}
	if player < 0 || player >= s.Rules.MaxPlayers {
		return models.ErrInvalidPlayer
	}
	if len(s.DiscardCompleted) == s.Rules.MaxPlayers && s.DiscardCompleted[player] {
		return models.ErrDiscardAlreadyCompleted
	}
	if len(cards) != s.Rules.DiscardCount() {
		return models.ErrInvalidDiscardCount
	}
	// remove cards from hand
	for _, dc := range cards {
		found := -1
		for i, hc := range s.Hands[player] {
			if hc.Rank == dc.Rank && hc.Suit == dc.Suit {
				found = i
				break
			}
		}
		if found < 0 {
			return models.ErrDiscardCardNotInHand
		}
		s.Hands[player] = append(s.Hands[player][:found], s.Hands[player][found+1:]...)
		s.Crib = append(s.Crib, dc)
	}

	neededDiscards := s.Rules.MaxPlayers * s.Rules.DiscardCount()
	if len(s.DiscardCompleted) == s.Rules.MaxPlayers {
		s.DiscardCompleted[player] = true
	}
	allDone := len(s.Crib) == neededDiscards
	if len(s.DiscardCompleted) == s.Rules.MaxPlayers {
		for i := 0; i < s.Rules.MaxPlayers; i++ {
			if !s.DiscardCompleted[i] {
				allDone = false
				break
			}
		}
	}

	if allDone && len(s.Crib) == neededDiscards {
		// 3-player cribbage: add one random card from deck to the crib to make 4.
		if s.Rules.MaxPlayers == 3 && len(s.Crib) < s.Rules.CribSize() {
			c, err := s.pop()
			if err != nil {
				return err
			}
			s.Crib = append(s.Crib, c)
		}
		// When crib is complete, cut and start pegging.
		cut, err := s.pop()
		if err != nil {
			return err
		}
		s.Cut = &cut
		s.Stage = "pegging"
		s.PeggingTotal = 0
		s.PeggingSeq = nil
		s.PeggingPassed = make([]bool, s.Rules.MaxPlayers)
		s.LastPlayIndex = -1
		s.DiscardCompleted = make([]bool, s.Rules.MaxPlayers)
		// Snapshot kept hands for later counting; pegging will consume from Hands.
		for i := 0; i < s.Rules.MaxPlayers; i++ {
			s.KeptHands[i] = append([]common.Card(nil), s.Hands[i]...)
		}
		s.CurrentIndex = (s.DealerIndex + 1) % s.Rules.MaxPlayers
	}
	return nil
}

func (s *State) PlayPeggingCard(player int, card common.Card) (score int, reasons []string, err error) {
	if s.Stage != "pegging" {
		return 0, nil, models.ErrNotInPeggingStage
	}
	if player != s.CurrentIndex {
		return 0, nil, models.ErrNotYourTurn
	}
	// validate card is in hand
	found := -1
	for i, hc := range s.Hands[player] {
		if hc.Rank == card.Rank && hc.Suit == card.Suit {
			found = i
			break
		}
	}
	if found < 0 {
		return 0, nil, models.ErrCardNotInHand
	}
	if s.PeggingTotal+card.Value15() > 31 {
		return 0, nil, models.ErrWouldExceed31
	}

	points, newTotal, reasons := PeggingScore(s.PeggingSeq, card, s.PeggingTotal)
	s.PeggingTotal = newTotal
	s.PeggingSeq = append(s.PeggingSeq, card)
	s.Scores[player] += points
	s.LastPlayIndex = player
	s.PeggingPassed[player] = false

	// remove from hand
	s.Hands[player] = append(s.Hands[player][:found], s.Hands[player][found+1:]...)

	// Reset on 31: next player leads.
	if s.PeggingTotal == 31 {
		s.resetPeggingAfterSequenceEnd((player + 1) % s.Rules.MaxPlayers)
		s.advanceToNextPlayableOrGo()
	} else {
		s.CurrentIndex = (s.CurrentIndex + 1) % s.Rules.MaxPlayers
		s.advanceToNextPlayableOrGo()
	}

	if err := s.maybeFinishRound(); err != nil {
		return points, reasons, err
	}

	return points, reasons, nil
}

func (s *State) Go(player int) (awarded int, err error) {
	if s.Stage != "pegging" {
		return 0, models.ErrNotInPeggingStage
	}
	if player != s.CurrentIndex {
		return 0, models.ErrNotYourTurn
	}
	if s.canPlay(player) {
		return 0, models.ErrHasLegalPlay
	}
	s.PeggingPassed[player] = true
	s.CurrentIndex = (s.CurrentIndex + 1) % s.Rules.MaxPlayers

	// If everyone has passed (or nobody can play), end the sequence.
	allPassed := true
	for i := 0; i < s.Rules.MaxPlayers; i++ {
		if !s.PeggingPassed[i] && s.canPlay(i) {
			allPassed = false
			break
		}
	}
	if allPassed {
		// Last card point only if we didn't hit 31.
		awardLast := s.PeggingTotal != 31
		lastPlay := s.LastPlayIndex
		if awardLast && s.LastPlayIndex >= 0 {
			s.Scores[s.LastPlayIndex] += 1
			awarded = 1
			// Prevent a second award when the round finishes.
			s.LastPlayIndex = -1
		}
		nextLead := (lastPlay + 1) % s.Rules.MaxPlayers
		if lastPlay < 0 {
			nextLead = (s.DealerIndex + 1) % s.Rules.MaxPlayers
		}
		s.resetPeggingAfterSequenceEnd(nextLead)
		s.advanceToNextPlayableOrGo()
	} else {
		s.advanceToNextPlayableOrGo()
	}

	if err := s.maybeFinishRound(); err != nil {
		return awarded, err
	}
	return awarded, nil
}

func (s *State) resetPeggingAfterSequenceEnd(nextLead int) {
	s.PeggingTotal = 0
	s.PeggingSeq = nil
	for i := range s.PeggingPassed {
		s.PeggingPassed[i] = false
	}
	s.CurrentIndex = nextLead
}

func (s *State) canPlay(player int) bool {
	if player < 0 || player >= s.Rules.MaxPlayers {
		return false
	}
	for _, c := range s.Hands[player] {
		if s.PeggingTotal+c.Value15() <= 31 {
			return true
		}
	}
	return false
}

func (s *State) advanceToNextPlayableOrGo() {
	// Move CurrentIndex forward until we find a player who can play,
	// or fall back to current (UI can call Go()).
	for i := 0; i < s.Rules.MaxPlayers; i++ {
		p := (s.CurrentIndex + i) % s.Rules.MaxPlayers
		if s.canPlay(p) {
			s.CurrentIndex = p
			return
		}
	}
}

func (s *State) maybeFinishRound() error {
	// If all hands are empty, finish pegging and count hands + crib.
	allEmpty := true
	for i := 0; i < s.Rules.MaxPlayers; i++ {
		if len(s.Hands[i]) > 0 {
			allEmpty = false
			break
		}
	}
	if !allEmpty {
		return nil
	}

	// Award last card point if the last sequence didn't end on 31.
	if s.PeggingTotal != 31 && s.LastPlayIndex >= 0 {
		s.Scores[s.LastPlayIndex] += 1
		s.LastPlayIndex = -1
	}

	s.Stage = "counting"
	if s.Cut == nil {
		// Should not happen, but avoid panic.
		s.Stage = "finished"
		return fmt.Errorf("missing cut card")
	}

	// Count hands in official order:
	// 1) Players left of dealer (clockwise) up to dealer
	// 2) Dealer's hand
	// 3) Dealer's crib
	//
	// We must check for a winner immediately after each hand/crib is counted so
	// the first player to reach 121 wins (no "overcount" by later hands).
	for off := 1; off < s.Rules.MaxPlayers; off++ {
		i := (s.DealerIndex + off) % s.Rules.MaxPlayers
		b := ScoreHand(s.KeptHands[i], *s.Cut, false)
		s.Scores[i] += b.Total
		if s.Scores[i] >= 121 {
			s.Stage = "finished"
			return nil
		}
	}
	// Dealer hand
	{
		i := s.DealerIndex
		b := ScoreHand(s.KeptHands[i], *s.Cut, false)
		s.Scores[i] += b.Total
		if s.Scores[i] >= 121 {
			s.Stage = "finished"
			return nil
		}
	}
	// Crib (dealer)
	{
		crib := ScoreHand(s.Crib, *s.Cut, true)
		s.Scores[s.DealerIndex] += crib.Total
		if s.Scores[s.DealerIndex] >= 121 {
			s.Stage = "finished"
			return nil
		}
	}

	// New round.
	s.DealerIndex = (s.DealerIndex + 1) % s.Rules.MaxPlayers
	if err := s.Deal(); err != nil {
		// revert dealer increment on failure
		s.DealerIndex = (s.DealerIndex - 1 + s.Rules.MaxPlayers) % s.Rules.MaxPlayers
		s.Stage = "finished"
		return err
	}
	return nil
}

func (s *State) MarshalJSON() ([]byte, error) {
	// Ensure cut pointer is represented (default encoding is fine, but we keep this for future shaping).
	type Alias State
	return json.Marshal((*Alias)(s))
}

func (s *State) pop() (common.Card, error) {
	if len(s.Deck) == 0 {
		return common.Card{}, fmt.Errorf("empty deck")
	}
	c := s.Deck[0]
	s.Deck = s.Deck[1:]
	return c, nil
}


