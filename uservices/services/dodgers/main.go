// Command dodgers is the "dodgers" microservice: it scores flushes (4 in hand, or 5 including the starter).
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

const serviceName = "dodgers"

const serviceVersion = "1.0.0"

func main() {
	svc := service.New(serviceName)
	svc.Handle("/score", handleScore(svc))
	svc.Handle("/version", handleVersion(svc))
	if err := svc.ListenAndServe(":8080"); err != nil {
		panic(err)
	}
}

// handleScore decodes a hand, applies the flush rule, and returns the
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

// score awards four points if all hand cards share a suit, plus one more if
// the starter matches (five total). The crib requires the starter to match.
func score(hand cribbage.Hand) cribbage.ScoreBreakdown {
	if len(hand.Cards) != 4 {
		return cribbage.ScoreBreakdown{}
	}
	suit := hand.Cards[0].Suit
	for _, c := range hand.Cards[1:] {
		if c.Suit != suit {
			return cribbage.ScoreBreakdown{}
		}
	}
	points := 4
	if hand.Starter.Suit == suit {
		points = 5
	} else if hand.IsCrib {
		return cribbage.ScoreBreakdown{} // crib flush needs all five
	}
	return cribbage.ScoreBreakdown{Components: []cribbage.ScoreComponent{{
		Rule:   "flush",
		Points: points,
	}}}
}

// handleVersion reports the service's name and build version.
func handleVersion(svc *service.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		service.WriteJSON(w, http.StatusOK, map[string]string{
			"service": svc.Self.Name,
			"version": serviceVersion,
		})
	}
}
