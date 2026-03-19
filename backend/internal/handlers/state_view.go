package handlers

import (
	"fifteen-thirty-one-go/backend/internal/game/common"
	"fifteen-thirty-one-go/backend/internal/game/cribbage"
)

// cloneStateBase deep-copies all shared scalar and slice fields of a cribbage.State.
// Caller is responsible for setting Version and populating Deck, Hands, KeptHands, Crib,
// CountSummary, and History as appropriate.
func cloneStateBase(st *cribbage.State) cribbage.State {
	var out cribbage.State
	out.Rules = st.Rules
	out.DealerIndex = st.DealerIndex
	out.CurrentIndex = st.CurrentIndex
	out.LastPlayIndex = st.LastPlayIndex
	out.PeggingTotal = st.PeggingTotal
	out.Stage = st.Stage

	if st.Cut != nil {
		c := *st.Cut
		out.Cut = &c
	}
	if st.Scores != nil {
		out.Scores = append([]int(nil), st.Scores...)
	}
	if st.PeggingPassed != nil {
		out.PeggingPassed = append([]bool(nil), st.PeggingPassed...)
	}
	if st.DiscardCompleted != nil {
		out.DiscardCompleted = append([]bool(nil), st.DiscardCompleted...)
	}
	if st.ReadyNextHand != nil {
		out.ReadyNextHand = append([]bool(nil), st.ReadyNextHand...)
	}
	if st.PeggingSeq != nil {
		out.PeggingSeq = append([]common.Card(nil), st.PeggingSeq...)
	}
	if st.History != nil {
		out.History = append([]cribbage.RoundSummary(nil), st.History...)
	}
	return out
}

// cloneStateDeep returns a full deep copy including Version, Deck, Hands, KeptHands, and Crib.
func cloneStateDeep(st *cribbage.State) cribbage.State {
	if st == nil {
		return cribbage.State{}
	}
	out := cloneStateBase(st)
	out.Version = st.Version
	if st.Deck != nil {
		out.Deck = append([]common.Card(nil), st.Deck...)
	}
	if st.Crib != nil {
		out.Crib = append([]common.Card(nil), st.Crib...)
	}
	out.Hands = make([][]common.Card, len(st.Hands))
	for i := range st.Hands {
		out.Hands[i] = append([]common.Card(nil), st.Hands[i]...)
	}
	out.KeptHands = make([][]common.Card, len(st.KeptHands))
	for i := range st.KeptHands {
		out.KeptHands[i] = append([]common.Card(nil), st.KeptHands[i]...)
	}
	out.CountSummary = st.CountSummary
	return out
}

// CloneStateForView returns a deep-copied state suitable for sending to clients,
// with hidden-card fields omitted to avoid accidental leakage.
func CloneStateForView(st *cribbage.State) cribbage.State {
	if st == nil {
		return cribbage.State{}
	}

	view := cloneStateBase(st)

	// Deep copy hands slice headers (but leave cards empty; filled selectively by caller).
	view.Hands = make([][]common.Card, len(st.Hands))
	for i := range view.Hands {
		view.Hands[i] = []common.Card{}
	}

	// Hidden-card fields:
	// - During discard/pegging we omit kept hands + crib to avoid leaking information.
	// - During counting/finished we reveal them so the UI can show the full hand scoring breakdown.
	if st.Stage == "counting" || st.Stage == "finished" {
		if st.KeptHands != nil {
			view.KeptHands = make([][]common.Card, len(st.KeptHands))
			for i := range st.KeptHands {
				view.KeptHands[i] = append([]common.Card(nil), st.KeptHands[i]...)
			}
		}
		if st.Crib != nil {
			view.Crib = append([]common.Card(nil), st.Crib...)
		}
		view.CountSummary = st.CountSummary
	} else {
		view.KeptHands = nil
		view.Crib = nil
		view.CountSummary = nil
	}
	view.Deck = nil

	return view
}
