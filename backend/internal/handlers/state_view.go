package handlers

import (
	"fifteen-thirty-one-go/backend/internal/game/common"
	"fifteen-thirty-one-go/backend/internal/game/cribbage"
)

// cloneStateForView returns a deep-copied state suitable for sending to clients,
// with hidden-card fields omitted to avoid accidental leakage.
func cloneStateForView(st *cribbage.State) cribbage.State {
	if st == nil {
		return cribbage.State{}
	}

	var view cribbage.State
	view.Rules = st.Rules
	view.DealerIndex = st.DealerIndex
	view.CurrentIndex = st.CurrentIndex
	view.LastPlayIndex = st.LastPlayIndex
	view.PeggingTotal = st.PeggingTotal
	view.Stage = st.Stage

	// Copy pointers
	if st.Cut != nil {
		c := *st.Cut
		view.Cut = &c
	}

	// Deep copy scalar slices
	if st.Scores != nil {
		view.Scores = append([]int(nil), st.Scores...)
	}
	if st.PeggingPassed != nil {
		view.PeggingPassed = append([]bool(nil), st.PeggingPassed...)
	}
	if st.DiscardCompleted != nil {
		view.DiscardCompleted = append([]bool(nil), st.DiscardCompleted...)
	}
	if st.PeggingSeq != nil {
		view.PeggingSeq = append([]common.Card(nil), st.PeggingSeq...)
	}

	// History is safe to expose at all times (it contains only past, non-secret info).
	if st.History != nil {
		view.History = append([]cribbage.RoundSummary(nil), st.History...)
	}

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

// CloneStateForView returns a deep-copied state suitable for sending to clients,
// with hidden-card fields omitted to avoid accidental leakage.
//
// This is the exported wrapper used by snapshot builders.
func CloneStateForView(st *cribbage.State) cribbage.State {
	return cloneStateForView(st)
}


