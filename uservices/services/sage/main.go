// Command sage is the "sage" microservice: it scores every combination of cards summing to 15 (2 points each).
//
// Tier and Argo-rollout status come from the shared catalog (see
// pkg/catalog); this service is tier-1 (impact tier 1 of 4) and deploys via an Argo Rollout. It builds on the shared service
// scaffold and cribbage domain library and is a leaf service that performs its work locally.
package main

import (
	"fmt"
	"net/http"

	"fifteen-thirty-one-go/uservices/pkg/cribbage"
	"fifteen-thirty-one-go/uservices/pkg/service"
)

const serviceName = "sage"

func main() {
	svc := service.New(serviceName)
	svc.Handle("/score", handleScore(svc))
	if err := svc.ListenAndServe(":8080"); err != nil {
		panic(err)
	}
}

// handleScore decodes a hand, applies the fifteens rule, and returns the
// resulting score components.
func handleScore(_ *service.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var hand cribbage.Hand
		if err := service.DecodeJSON(r, &hand); err != nil {
			service.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := hand.Validate(); err != nil {
			service.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		service.WriteJSON(w, http.StatusOK, score(hand))
	}
}

// score awards two points for every subset of cards whose pip values sum to 15.
func score(hand cribbage.Hand) cribbage.ScoreBreakdown {
	cards := hand.AllCards()
	count := 0
	n := len(cards)
	for mask := 1; mask < (1 << n); mask++ {
		sum := 0
		for i := 0; i < n; i++ {
			if mask&(1<<i) != 0 {
				sum += cards[i].PipValue()
			}
		}
		if sum == 15 {
			count++
		}
	}
	if count == 0 {
		return cribbage.ScoreBreakdown{}
	}
	return cribbage.ScoreBreakdown{Components: []cribbage.ScoreComponent{{
		Rule:   "fifteen",
		Points: count * 2,
		Detail: fmt.Sprintf("%d combination(s) summing to 15", count),
	}}}
}
