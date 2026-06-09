// Command dijon is the "dijon" microservice: it estimates a hand's expected value across possible starters.
//
// Tier and Argo-rollout status come from the shared catalog (see
// pkg/catalog); this service is tier-3 (impact tier 3 of 4) and deploys via an Argo Rollout. It builds on the shared service
// scaffold and cribbage domain library and calls peer "maple" over HTTP via the shared mesh client.
package main

import (
	"net/http"

	"fifteen-thirty-one-go/uservices/pkg/cribbage"
	"fifteen-thirty-one-go/uservices/pkg/service"
)

const serviceName = "dijon"

func main() {
	svc := service.New(serviceName)
	svc.Handle("/score", handleScore(svc))
	if err := svc.ListenAndServe(":8080"); err != nil {
		panic(err)
	}
}

// handleScore scores the hand against several candidate starters via the
// aggregator and returns the average total as an expected value.
func handleScore(svc *service.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var hand cribbage.Hand
		if err := service.DecodeJSON(r, &hand); err != nil {
			service.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		ctx, cancel := service.RequestContext(r.Context())
		defer cancel()

		starters := []cribbage.Card{
			{Rank: 5, Suit: cribbage.Hearts},
			{Rank: 10, Suit: cribbage.Spades},
			{Rank: 11, Suit: cribbage.Clubs},
		}
		total := 0
		for _, st := range starters {
			trial := hand
			trial.Starter = st
			var bd cribbage.ScoreBreakdown
			if err := svc.Mesh.Call(ctx, "maple", "/score", trial, &bd); err != nil {
				service.WriteJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
				return
			}
			total += bd.Total()
		}
		service.WriteJSON(w, http.StatusOK, map[string]any{
			"expected_value": float64(total) / float64(len(starters)),
			"samples":        len(starters),
		})
	}
}
