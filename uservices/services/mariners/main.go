// Command mariners is the "mariners" microservice: it combines nobs and heels into a jacks subtotal.
//
// Tier and Argo-rollout status come from the shared catalog (see
// pkg/catalog); this service is tier-2 (impact tier 2 of 4) and deploys via a plain Kubernetes Deployment. It builds on the shared service
// scaffold and cribbage domain library and calls peer "rockies" and "toulouse" over HTTP via the shared mesh client.
package main

import (
	"net/http"

	"fifteen-thirty-one-go/uservices/pkg/cribbage"
	"fifteen-thirty-one-go/uservices/pkg/service"
)

const serviceName = "mariners"

const serviceVersion = "1.0.0"

func main() {
	svc := service.New(serviceName)
	svc.Handle("/score", handleScore(svc))
	svc.Handle("/version", handleVersion(svc))
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
		if err := svc.Mesh.Call(ctx, "rockies", "/score", hand, &part0); err != nil {
			service.WriteJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		result = result.Merge(part0)
		var part1 cribbage.ScoreBreakdown
		if err := svc.Mesh.Call(ctx, "toulouse", "/score", hand, &part1); err != nil {
			service.WriteJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		result = result.Merge(part1)
		service.WriteJSON(w, http.StatusOK, result)
	}
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
