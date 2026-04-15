package utilscompat

import (
	"testing"

	"fifteen-thirty-one-go/backend/internal/game/common"
	"fifteen-thirty-one-go/backend/internal/game/cribbage"

	"github.com/iantybo/fifteen-thirty-one-go-utils/pkg/analysis"
)

func TestScoreHandParityWithUtils_NonCrib(t *testing.T) {
	t.Parallel()

	// Non-crib hands only (isCrib=false). Avoid four-card flushes in the hand
	// where the cut does not match — runtime and utils agree for standard hand flush scoring.
	cases := []struct {
		name string
		hand [4]string
		cut  string
	}{
		{
			name: "mixed_suits_fifteens_and_runs",
			hand: [4]string{"AH", "4H", "5H", "6H"},
			cut:  "7D",
		},
		{
			name: "pairs",
			hand: [4]string{"7S", "7C", "8D", "9H"},
			cut:  "AS",
		},
		{
			name: "no_flush_all_different",
			hand: [4]string{"AS", "2C", "3D", "4H"},
			cut:  "5S",
		},
		{
			name: "five_card_flush",
			hand: [4]string{"2H", "3H", "4H", "5H"},
			cut:  "6H",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			hand := make([]common.Card, 4)
			for i, s := range tc.hand {
				c, err := common.ParseCard(s)
				if err != nil {
					t.Fatalf("ParseCard %q: %v", s, err)
				}
				hand[i] = c
			}
			cut, err := common.ParseCard(tc.cut)
			if err != nil {
				t.Fatalf("ParseCard cut %q: %v", tc.cut, err)
			}

			gotRuntime := cribbage.ScoreHand(hand, cut, false).Total
			uHand := ToUtilsCards(hand)
			uCut := ToUtilsCard(cut)
			gotUtils := analysis.ScoreHand(uHand, uCut).Total

			if gotRuntime != gotUtils {
				t.Fatalf("ScoreHand total mismatch: runtime=%d utils=%d (hand=%v cut=%v)",
					gotRuntime, gotUtils, tc.hand, tc.cut)
			}
		})
	}
}
