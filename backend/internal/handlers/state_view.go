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

	view := *st

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

	// Deep copy hands slice headers (but leave cards empty; filled selectively by caller).
	view.Hands = make([][]common.Card, len(st.Hands))
	for i := range view.Hands {
		view.Hands[i] = []common.Card{}
	}

	// Do not copy hidden-card fields by default.
	view.KeptHands = nil
	view.Crib = nil
	view.PeggingSeq = nil

	// Do not copy Deck.
	view.Deck = nil

	return view
}


