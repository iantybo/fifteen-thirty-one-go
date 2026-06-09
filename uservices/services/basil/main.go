// Command basil is the "basil" microservice: it deals a random validated hand for demos and load tests.
//
// Tier and Argo-rollout status come from the shared catalog (see
// pkg/catalog); this service is tier-4 (impact tier 4 of 4) and deploys via an Argo Rollout. It builds on the shared service
// scaffold and cribbage domain library and is a leaf service that performs its work locally.
package main

import (
	"net/http"

	"fifteen-thirty-one-go/uservices/pkg/cribbage"
	"fifteen-thirty-one-go/uservices/pkg/service"
)

const serviceName = "basil"

func main() {
	svc := service.New(serviceName)
	svc.Handle("/score", handleScore(svc))
	if err := svc.ListenAndServe(":8080"); err != nil {
		panic(err)
	}
}

// handleScore deals a fixed demo hand (deterministic for reproducible load
// tests) and returns it validated.
func handleScore(_ *service.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hand := cribbage.Hand{
			Cards: []cribbage.Card{
				{Rank: 5, Suit: cribbage.Spades},
				{Rank: 5, Suit: cribbage.Hearts},
				{Rank: 5, Suit: cribbage.Clubs},
				{Rank: 11, Suit: cribbage.Diamonds},
			},
			Starter: cribbage.Card{Rank: 5, Suit: cribbage.Diamonds},
		}
		if err := hand.Validate(); err != nil {
			service.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		service.WriteJSON(w, http.StatusOK, hand)
	}
}
