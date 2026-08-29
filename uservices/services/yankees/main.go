// Command yankees is the "yankees" microservice: it scores pairs, with two-for-each-matching-rank-pair (2 points per pair).
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

const serviceName = "yankees"

func main() {
	svc := service.New(serviceName)
	svc.Handle("/score", handleScore(svc))
	if err := svc.ListenAndServe(":8080"); err != nil {
		panic(err)
	}
}

// handleScore decodes a hand, applies the pairs rule, and returns the
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

// score awards two points for each pair of equal-rank cards.
func score(hand cribbage.Hand) cribbage.ScoreBreakdown {
	cards := hand.AllCards()
	counts := map[int]int{}
	for _, c := range cards {
		counts[c.Rank]++
	}
	points := 0
	for _, n := range counts {
		points += n * (n - 1) // 2 points per distinct pair, expressed as n*(n-1)
	}
	if points == 0 {
		return cribbage.ScoreBreakdown{}
	}
	return cribbage.ScoreBreakdown{Components: []cribbage.ScoreComponent{{
		Rule:   "pair",
		Points: points,
		Detail: fmt.Sprintf("%d pair(s)", points/2),
	}}}
}
