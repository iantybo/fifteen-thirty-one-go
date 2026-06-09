// Command lille is the "lille" microservice: it estimates pegging potential from sequence and meld subtotals.
//
// Tier and Argo-rollout status come from the shared catalog (see
// pkg/catalog); this service is tier-3 (impact tier 3 of 4) and deploys via an Argo Rollout. It builds on the shared service
// scaffold and cribbage domain library and calls peer "padres" and "cardinals" over HTTP via the shared mesh client.
package main

import (
	"net/http"

	"fifteen-thirty-one-go/uservices/pkg/cribbage"
	"fifteen-thirty-one-go/uservices/pkg/service"
)

const serviceName = "lille"

func main() {
	svc := service.New(serviceName)
	svc.Handle("/score", handleScore(svc))
	if err := svc.ListenAndServe(":8080"); err != nil {
		panic(err)
	}
}

// handleScore fans out to its peer scorers, merges their breakdowns into one
// subtotal, and returns it.
func handleScore(svc *service.Service) http.HandlerFunc {
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
		ctx, cancel := service.RequestContext(r.Context())
		defer cancel()

		var result cribbage.ScoreBreakdown
		var part0 cribbage.ScoreBreakdown
		if err := svc.Mesh.Call(ctx, "padres", "/score", hand, &part0); err != nil {
			service.WriteJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		result = result.Merge(part0)
		var part1 cribbage.ScoreBreakdown
		if err := svc.Mesh.Call(ctx, "cardinals", "/score", hand, &part1); err != nil {
			service.WriteJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		result = result.Merge(part1)
		service.WriteJSON(w, http.StatusOK, result)
	}
}
