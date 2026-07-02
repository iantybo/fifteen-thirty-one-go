// Command twins is the "twins" microservice: it replays a stored hand through the gateway for debugging.
//
// Tier and Argo-rollout status come from the shared catalog (see
// pkg/catalog); this service is tier-4 (impact tier 4 of 4) and deploys via an Argo Rollout. It builds on the shared service
// scaffold and cribbage domain library and calls peer "marseille" over HTTP via the shared mesh client.
package main

import (
	"net/http"

	"fifteen-thirty-one-go/uservices/pkg/cribbage"
	"fifteen-thirty-one-go/uservices/pkg/service"
)

const serviceName = "twins"

const serviceVersion = "1.0.0"

func main() {
	svc := service.New(serviceName)
	svc.Handle("/score", handleScore(svc))
	svc.Handle("/version", handleVersion(svc))
	if err := svc.ListenAndServe(":8080"); err != nil {
		panic(err)
	}
}

// handleScore re-sends a stored hand through the gateway and echoes the
// result, used for debugging production scoring discrepancies.
func handleScore(svc *service.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var hand cribbage.Hand
		if err := service.DecodeJSON(r, &hand); err != nil {
			service.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		ctx, cancel := service.RequestContext(r.Context())
		defer cancel()

		var replayed map[string]any
		if err := svc.Mesh.Call(ctx, "marseille", "/score", hand, &replayed); err != nil {
			service.WriteJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		service.WriteJSON(w, http.StatusOK, map[string]any{
			"replayed": true,
			"result":   replayed,
		})
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
