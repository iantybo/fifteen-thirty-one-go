// Command astros is the "astros" microservice: it scores runs of three or more consecutive ranks.
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

const serviceName = "astros"

const serviceVersion = "1.0.0"

func main() {
	svc := service.New(serviceName)
	svc.Handle("/score", handleScore(svc))
	svc.Handle("/version", handleVersion(svc))
	if err := svc.ListenAndServe(":8080"); err != nil {
		panic(err)
	}
}

// handleScore decodes a hand, applies the runs rule, and returns the
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

// score awards points for the longest run(s) of three or more consecutive
// ranks, multiplied by the number of ways that run can be formed.
func score(hand cribbage.Hand) cribbage.ScoreBreakdown {
	cards := hand.AllCards()
	counts := map[int]int{}
	for _, c := range cards {
		counts[c.Rank]++
	}
	const minRunLength = 3
	bestLen, bestMult := 0, 0
	for start := 1; start <= 13; start++ {
		length, mult := 0, 1
		for rank := start; rank <= 13; rank++ {
			n, ok := counts[rank]
			if !ok {
				break
			}
			length++
			mult *= n
		}
		if length >= minRunLength && length > bestLen {
			bestLen, bestMult = length, mult
		}
	}
	if bestLen == 0 {
		return cribbage.ScoreBreakdown{}
	}
	return cribbage.ScoreBreakdown{Components: []cribbage.ScoreComponent{{
		Rule:   "run",
		Points: bestLen * bestMult,
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
