// Command rockies is the "rockies" microservice: it scores his-nobs: a Jack in hand matching the starter's suit.
//
// Tier and Argo-rollout status come from the shared catalog (see
// pkg/catalog); this service is tier-1 (impact tier 1 of 4) and deploys via a plain Kubernetes Deployment. It builds on the shared service
// scaffold and cribbage domain library and is a leaf service that performs its work locally.
package main

import (
	"net/http"

	"fifteen-thirty-one-go/uservices/pkg/cribbage"
	"fifteen-thirty-one-go/uservices/pkg/service"
)

const serviceName = "rockies"

func main() {
	svc := service.New(serviceName)
	svc.Handle("/score", handleScore(svc))
	if err := svc.ListenAndServe(":8080"); err != nil {
		panic(err)
	}
}

// handleScore decodes a hand, applies the nobs rule, and returns the
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

// score awards one point for a Jack in hand whose suit matches the starter.
func score(hand cribbage.Hand) cribbage.ScoreBreakdown {
	for _, c := range hand.Cards {
		if c.Rank == 11 && c.Suit == hand.Starter.Suit {
			return cribbage.ScoreBreakdown{Components: []cribbage.ScoreComponent{{
				Rule:   "nobs",
				Points: 1,
				Detail: c.Code(),
			}}}
		}
	}
	return cribbage.ScoreBreakdown{}
}
