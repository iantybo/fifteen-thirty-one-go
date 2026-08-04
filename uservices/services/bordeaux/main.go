// Command bordeaux is the "bordeaux" microservice: it public entrypoint that routes a crib hand to the cribscore aggregator.
//
// Tier and Argo-rollout status come from the shared catalog (see
// pkg/catalog); this service is tier-3 (impact tier 3 of 4) and deploys via a plain Kubernetes Deployment. It builds on the shared service
// scaffold and cribbage domain library and calls peer "ivy" over HTTP via the shared mesh client.
package main

import (
	"net/http"

	"fifteen-thirty-one-go/uservices/pkg/cribbage"
	"fifteen-thirty-one-go/uservices/pkg/service"
)

const service

Name = "bordeaux"

func main() {
	svc := service.New(serviceName)
	svc.Handle("/score", handleScore(svc))
	if err := svc.ListenAndServe(":8080"); err != nil {
		panic(err)
	}
}

// handleScore is the public entrypoint: it forwards the hand to the
// downstream aggregator and returns the assembled breakdown with its total.
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

		var bd cribbage.ScoreBreakdown
		if err := svc.Mesh.Call(ctx, "ivy", "/score", hand, &bd); err != nil {
			service.WriteJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		service.WriteJSON(w, http.StatusOK, map[string]any{
			"breakdown": bd,
			"total":     bd.Total(),
		})
	}
}
