package cribbage

import (
	"fmt"
	"log"
	"sort"

	"fifteen-thirty-one-go/backend/internal/game/common"
)

// StageTransitionInfo captures metadata about a state transition for observability.
type StageTransitionInfo struct {
	FromStage    string `json:"from_stage"`
	ToStage      string `json:"to_stage"`
	DealerIndex  int    `json:"dealer_index"`
	CurrentIndex int    `json:"current_index"`
	Round        int    `json:"round"`
}

// TransitionToDiscard prepares the state for the discard phase after dealing.
// This encapsulates the logic that was previously inline in Deal().
func (s *State) TransitionToDiscard() StageTransitionInfo {
	info := StageTransitionInfo{
		FromStage:    s.Stage,
		ToStage:      "discard",
		DealerIndex:  s.DealerIndex,
		CurrentIndex: s.CurrentIndex,
		Round:        len(s.History) + 1,
	}

	s.Stage = "discard"
	s.DiscardCompleted = make([]bool, s.Rules.MaxPlayers)
	s.KeptHands = make([][]common.Card, s.Rules.MaxPlayers)
	s.PeggingPassed = make([]bool, s.Rules.MaxPlayers)
	s.PeggingSeq = nil
	s.PeggingTotal = 0
	s.LastPlayIndex = -1
	s.CountSummary = nil
	s.ReadyNextHand = nil
	s.CurrentIndex = (s.DealerIndex + 1) % s.Rules.MaxPlayers

	return info
}

// TransitionToPegging prepares the state for the pegging phase after all discards.
func (s *State) TransitionToPegging() (StageTransitionInfo, error) {
	info := StageTransitionInfo{
		FromStage:    s.Stage,
		ToStage:      "pegging",
		DealerIndex:  s.DealerIndex,
		CurrentIndex: s.CurrentIndex,
		Round:        len(s.History) + 1,
	}

	if s.Stage != "discard" {
		return info, fmt.Errorf("cannot transition to pegging from %s", s.Stage)
	}

	cut, err := s.pop()
	if err != nil {
		return info, fmt.Errorf("cut card: %w", err)
	}
	s.Cut = &cut
	s.Stage = "pegging"
	s.CountSummary = nil
	s.PeggingTotal = 0
	s.PeggingSeq = nil
	s.PeggingPassed = make([]bool, s.Rules.MaxPlayers)
	s.LastPlayIndex = -1
	s.DiscardCompleted = make([]bool, s.Rules.MaxPlayers)

	for i := 0; i < s.Rules.MaxPlayers; i++ {
		s.KeptHands[i] = append([]common.Card(nil), s.Hands[i]...)
	}
	s.CurrentIndex = (s.DealerIndex + 1) % s.Rules.MaxPlayers

	return info, nil
}

// AnalyzeHandStrength returns a simple numeric evaluation of a 4-card kept hand
// based on potential scoring combinations. Higher is better.
func AnalyzeHandStrength(hand []common.Card) int {
	if len(hand) != 4 {
		return 0
	}

	strength := 0

	// Check for pair potential
	rankCounts := make(map[common.Rank]int)
	for _, c := range hand {
		rankCounts[c.Rank]++
	}
	for _, count := range rankCounts {
		if count >= 2 {
			strength += count * 2
		}
	}

	// Check for fifteen potential (all subsets summing to 15)
	n := len(hand)
	for mask := 1; mask < (1 << n); mask++ {
		sum := 0
		for i := 0; i < n; i++ {
			if mask&(1<<i) != 0 {
				sum += hand[i].Value15()
			}
		}
		if sum == 15 {
			strength += 2
		}
	}

	// Check for run potential
	ranks := make([]int, 0, 4)
	for _, c := range hand {
		ranks = append(ranks, int(c.Rank))
	}
	sort.Ints(ranks)
	consecutive := 1
	maxConsecutive := 1
	for i := 1; i < len(ranks); i++ {
		if ranks[i] == ranks[i-1]+1 {
			consecutive++
			if consecutive > maxConsecutive {
				maxConsecutive = consecutive
			}
		} else if ranks[i] != ranks[i-1] {
			consecutive = 1
		}
	}
	if maxConsecutive >= 3 {
		strength += maxConsecutive
	}

	// Check for flush
	suit := hand[0].Suit
	flushCount := 1
	for i := 1; i < len(hand); i++ {
		if hand[i].Suit == suit {
			flushCount++
		}
	}
	if flushCount == 4 {
		strength += 4
	}

	return strength
}

// OptimalDiscardPair finds the best pair of cards to discard from a 6-card hand
// by evaluating all possible 4-card kept hands.
func OptimalDiscardPair(hand []common.Card) ([]common.Card, []common.Card) {
	if len(hand) != 6 {
		return nil, hand
	}

	bestStrength := -1
	bestDiscard := []int{0, 1}

	for i := 0; i < 6; i++ {
		for j := i + 1; j < 6; j++ {
			kept := make([]common.Card, 0, 4)
			for k := 0; k < 6; k++ {
				if k != i && k != j {
					kept = append(kept, hand[k])
				}
			}
			s := AnalyzeHandStrength(kept)
			if s > bestStrength {
				bestStrength = s
				bestDiscard = []int{i, j}
			}
		}
	}

	discards := []common.Card{hand[bestDiscard[0]], hand[bestDiscard[1]]}
	kept := make([]common.Card, 0, 4)
	for k := 0; k < 6; k++ {
		if k != bestDiscard[0] && k != bestDiscard[1] {
			kept = append(kept, hand[k])
		}
	}

	return discards, kept
}

func PeggingPlaySummary(seq []common.Card, total int) string {
	if len(seq) == 0 {
		return fmt.Sprintf("empty sequence, total=%d", total)
	}
	cards := ""
	for i, c := range seq {
		if i > 0 {
			cards += ", "
		}
		cards += c.String()
	}
	return fmt.Sprintf("[%s] total=%d", cards, total)
}

// ValidateGameIntegrity performs a comprehensive check on the game state to detect
// any inconsistencies that could affect gameplay.
func ValidateGameIntegrity(s *State) []string {
	if s == nil {
		return []string{"nil state"}
	}

	var issues []string

	if s.Rules.MaxPlayers < 2 || s.Rules.MaxPlayers > 4 {
		issues = append(issues, fmt.Sprintf("invalid max_players: %d", s.Rules.MaxPlayers))
	}

	if len(s.Hands) != s.Rules.MaxPlayers {
		issues = append(issues, fmt.Sprintf("hands length %d != max_players %d", len(s.Hands), s.Rules.MaxPlayers))
	}
	if len(s.Scores) != s.Rules.MaxPlayers {
		issues = append(issues, fmt.Sprintf("scores length %d != max_players %d", len(s.Scores), s.Rules.MaxPlayers))
	}

	if s.DealerIndex < 0 || s.DealerIndex >= s.Rules.MaxPlayers {
		issues = append(issues, fmt.Sprintf("dealer_index out of bounds: %d", s.DealerIndex))
	}
	if s.CurrentIndex < 0 || s.CurrentIndex >= s.Rules.MaxPlayers {
		issues = append(issues, fmt.Sprintf("current_index out of bounds: %d", s.CurrentIndex))
	}

	validStages := map[string]bool{
		"dealing": true, "discard": true, "pegging": true,
		"counting": true, "finished": true,
	}
	if !validStages[s.Stage] {
		issues = append(issues, fmt.Sprintf("unknown stage: %s", s.Stage))
	}

	if s.PeggingTotal < 0 || s.PeggingTotal > 31 {
		issues = append(issues, fmt.Sprintf("pegging_total out of bounds: %d", s.PeggingTotal))
	}

	allCards := make(map[string]bool)
	for pi, hand := range s.Hands {
		for _, c := range hand {
			key := c.String()
			if allCards[key] {
				issues = append(issues, fmt.Sprintf("duplicate card %s in hand[%d]", key, pi))
			}
			allCards[key] = true
		}
	}

	for _, c := range s.Crib {
		key := c.String()
		if allCards[key] {
			issues = append(issues, fmt.Sprintf("duplicate card %s in crib", key))
		}
		allCards[key] = true
	}

	for _, c := range s.Deck {
		key := c.String()
		if allCards[key] {
			log.Printf("ValidateGameIntegrity: duplicate card %s in deck (may be acceptable during dealing)", key)
		}
		allCards[key] = true
	}

	return issues
}
